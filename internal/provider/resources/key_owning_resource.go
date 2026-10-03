package resources

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"maps"
	"regexp"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/api"
)

var (
	_ resource.Resource                = &keyOwningResource{}
	_ resource.ResourceWithConfigure   = &keyOwningResource{}
	_ resource.ResourceWithImportState = &keyOwningResource{}
	_ resource.ResourceWithModifyPlan  = &keyOwningResource{}
)

// ownedKeyKind describes the key a v2 resource owns.
type ownedKeyKind struct {
	attr  string // Schema attribute holding the key, e.g. "agent_key".
	label string // e.g. "agent key".
	// recreate creates a replacement hashed key for the object with this ID and name.
	recreate func(client *api.Client, objectID, objectName string) (*api.APIKey, error)
}

var agentKeyKind = ownedKeyKind{
	attr:  "agent_key",
	label: "agent key",
	recreate: func(client *api.Client, clusterID, clusterName string) (*api.APIKey, error) {
		return client.CreateHashedAgentKey(ownedKeyName(strings.TrimPrefix(clusterName, "vcn_"), "agent_key"), clusterID)
	},
}

var applicationKeyKind = ownedKeyKind{
	attr:  "application_key",
	label: "application key",
	recreate: func(client *api.Client, workspaceID, workspaceName string) (*api.APIKey, error) {
		return client.CreateHashedApplicationKey(ownedKeyName(workspaceName, "application_key"), workspaceID)
	},
}

// keyOwningResource is a v2 resource: it wraps a v1 resource, whose object it creates through the v2
// endpoint together with a hashed key, and stores that key, whose secret is only returned once, in state.
// The v1 resource does all the object work on a copy of the plan and state without the key attribute.
type keyOwningResource struct {
	base     resource.Resource
	typeName string
	kind     ownedKeyKind
	client   *api.Client
}

func newKeyOwningResource(base resource.Resource, typeName string, kind ownedKeyKind) resource.Resource {
	return &keyOwningResource{base: base, typeName: typeName, kind: kind}
}

func NewVirtualClusterV2Resource() resource.Resource {
	return newKeyOwningResource(NewVirtualClusterResource(), "_virtual_cluster_v2", agentKeyKind)
}

func NewSchemaRegistryV2Resource() resource.Resource {
	return newKeyOwningResource(NewSchemaRegistryResource(), "_schema_registry_v2", agentKeyKind)
}

func NewTableFlowV2Resource() resource.Resource {
	return newKeyOwningResource(NewTableFlowResource(), "_tableflow_cluster_v2", agentKeyKind)
}

func NewWorkspaceV2Resource() resource.Resource {
	return newKeyOwningResource(NewWorkspaceResource(), "_workspace_v2", applicationKeyKind)
}

func (r *keyOwningResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + r.typeName
}

func (r *keyOwningResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if client, ok := req.ProviderData.(*api.Client); ok {
		r.client = client
	}
	if base, ok := r.base.(resource.ResourceWithConfigure); ok {
		base.Configure(ctx, req, resp)
	}
}

func (r *keyOwningResource) baseSchema(ctx context.Context) (schema.Schema, diag.Diagnostics) {
	var resp resource.SchemaResponse
	r.base.Schema(ctx, resource.SchemaRequest{}, &resp)
	return resp.Schema, resp.Diagnostics
}

func (r *keyOwningResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	s, diags := r.baseSchema(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	s.Attributes = maps.Clone(s.Attributes)
	s.Attributes[r.kind.attr] = schema.SingleNestedAttribute{
		Description: fmt.Sprintf("Hashed %s created and managed by this resource. Its secret is only returned "+
			"when the key is created and is kept in Terraform state. If the key is deleted outside Terraform, "+
			"the next apply creates a new one.", r.kind.label),
		Computed: true,
		Attributes: map[string]schema.Attribute{
			"id":         schema.StringAttribute{Description: "Key ID.", Computed: true},
			"name":       schema.StringAttribute{Description: "Key name.", Computed: true},
			"key":        schema.StringAttribute{Description: "Key secret.", Computed: true, Sensitive: true},
			"created_at": schema.StringAttribute{Description: "Key creation timestamp.", Computed: true},
		},
		PlanModifiers: []planmodifier.Object{objectplanmodifier.UseStateForUnknown()},
	}
	s.Description = fmt.Sprintf("%s\n\nUnlike the v1 resource, this resource creates its %s together with the "+
		"object and manages it, see `%s`.", s.Description, r.kind.label, r.kind.attr)
	resp.Schema = s
}

func (r *keyOwningResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	bs, diags := r.baseSchema(ctx)
	resp.Diagnostics.Append(diags...)
	config, d := r.strip(ctx, bs, req.Config.Raw)
	resp.Diagnostics.Append(d...)
	plan, d := r.strip(ctx, bs, req.Plan.Raw)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}

	ctx, capture := withOwnedKeyCapture(ctx)
	baseResp := resource.CreateResponse{
		State:   tfsdk.State{Schema: bs, Raw: tftypes.NewValue(bs.Type().TerraformType(ctx), nil)},
		Private: resp.Private,
	}
	r.base.Create(ctx, resource.CreateRequest{
		Config:       tfsdk.Config{Schema: bs, Raw: config},
		Plan:         tfsdk.Plan{Schema: bs, Raw: plan},
		ProviderMeta: req.ProviderMeta,
	}, &baseResp)
	resp.Diagnostics.Append(baseResp.Diagnostics...)
	resp.Private = baseResp.Private

	key := ownedKeyValue(capture.key)
	if !baseResp.State.Raw.IsNull() {
		r.setWithKey(ctx, &resp.State, baseResp.State.Raw, key, &resp.Diagnostics)
		return
	}
	if capture.objectID == "" {
		return
	}

	// The object exists but the v1 resource saved no state: save what we know so Terraform taints and
	// replaces it rather than orphaning the object and its key.
	partial, err := tftypes.Transform(req.Plan.Raw, func(_ *tftypes.AttributePath, v tftypes.Value) (tftypes.Value, error) {
		if !v.IsKnown() {
			return tftypes.NewValue(v.Type(), nil), nil
		}
		return v, nil
	})
	if err != nil {
		resp.Diagnostics.AddError("Error Saving Partially Created Resource", err.Error())
		return
	}
	resp.State.Raw = partial
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), capture.objectID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root(r.kind.attr), key)...)
}

func (r *keyOwningResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	bs, diags := r.baseSchema(ctx)
	resp.Diagnostics.Append(diags...)
	state, d := r.strip(ctx, bs, req.State.Raw)
	resp.Diagnostics.Append(d...)
	var key types.Object
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root(r.kind.attr), &key)...)
	if resp.Diagnostics.HasError() {
		return
	}

	baseResp := resource.ReadResponse{State: tfsdk.State{Schema: bs, Raw: state}, Private: resp.Private}
	r.base.Read(ctx, resource.ReadRequest{
		State:              tfsdk.State{Schema: bs, Raw: state},
		Private:            req.Private,
		ProviderMeta:       req.ProviderMeta,
		ClientCapabilities: req.ClientCapabilities,
	}, &baseResp)
	resp.Diagnostics.Append(baseResp.Diagnostics...)
	resp.Private = baseResp.Private
	resp.Deferred = baseResp.Deferred
	if baseResp.State.Raw.IsNull() {
		resp.State.RemoveResource(ctx)
		return
	}

	// A key deleted outside Terraform is dropped from state, so the next plan recreates it.
	if !resp.Diagnostics.HasError() && !key.IsNull() && !key.IsUnknown() {
		keyID, _ := key.Attributes()["id"].(types.String)
		if _, err := r.client.GetAPIKey(keyID.ValueString()); errors.Is(err, api.ErrNotFound) {
			key = ownedKeyValue(nil)
		} else if err != nil {
			resp.Diagnostics.AddError(
				"Error Reading WarpStream "+r.kind.label,
				"Could not read "+r.kind.label+" "+keyID.ValueString()+": "+err.Error(),
			)
		}
	}
	r.setWithKey(ctx, &resp.State, baseResp.State.Raw, key, &resp.Diagnostics)
}

func (r *keyOwningResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	bs, diags := r.baseSchema(ctx)
	resp.Diagnostics.Append(diags...)
	config, d := r.strip(ctx, bs, req.Config.Raw)
	resp.Diagnostics.Append(d...)
	plan, d := r.strip(ctx, bs, req.Plan.Raw)
	resp.Diagnostics.Append(d...)
	state, d := r.strip(ctx, bs, req.State.Raw)
	resp.Diagnostics.Append(d...)
	var key types.Object
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root(r.kind.attr), &key)...)
	if resp.Diagnostics.HasError() {
		return
	}

	baseResp := resource.UpdateResponse{State: tfsdk.State{Schema: bs, Raw: plan}, Private: resp.Private}
	r.base.Update(ctx, resource.UpdateRequest{
		Config:       tfsdk.Config{Schema: bs, Raw: config},
		Plan:         tfsdk.Plan{Schema: bs, Raw: plan},
		State:        tfsdk.State{Schema: bs, Raw: state},
		ProviderMeta: req.ProviderMeta,
		Private:      req.Private,
	}, &baseResp)
	resp.Diagnostics.Append(baseResp.Diagnostics...)
	resp.Private = baseResp.Private

	// An unknown planned key means it is missing: imported, or deleted outside Terraform.
	if key.IsUnknown() {
		key = ownedKeyValue(nil)
		if !resp.Diagnostics.HasError() {
			key = r.recreateKey(ctx, baseResp.State, &resp.Diagnostics)
		}
	}
	r.setWithKey(ctx, &resp.State, baseResp.State.Raw, key, &resp.Diagnostics)
}

func (r *keyOwningResource) recreateKey(ctx context.Context, state tfsdk.State, respDiags *diag.Diagnostics) types.Object {
	var id, name types.String
	respDiags.Append(state.GetAttribute(ctx, path.Root("id"), &id)...)
	respDiags.Append(state.GetAttribute(ctx, path.Root("name"), &name)...)
	if respDiags.HasError() {
		return ownedKeyValue(nil)
	}

	key, err := r.kind.recreate(r.client, id.ValueString(), name.ValueString())
	if err != nil {
		respDiags.AddError(
			"Error Creating WarpStream "+r.kind.label,
			"Could not create "+r.kind.label+" for "+id.ValueString()+": "+err.Error(),
		)
		return ownedKeyValue(nil)
	}
	return ownedKeyValue(key)
}

func (r *keyOwningResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	bs, diags := r.baseSchema(ctx)
	resp.Diagnostics.Append(diags...)
	state, d := r.strip(ctx, bs, req.State.Raw)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Deleting the object also revokes its key.
	baseResp := resource.DeleteResponse{State: tfsdk.State{Schema: bs, Raw: state}, Private: resp.Private}
	r.base.Delete(ctx, resource.DeleteRequest{
		State:        tfsdk.State{Schema: bs, Raw: state},
		ProviderMeta: req.ProviderMeta,
		Private:      req.Private,
	}, &baseResp)
	resp.Diagnostics.Append(baseResp.Diagnostics...)
	resp.Private = baseResp.Private
}

// ImportState imports the object only: the key's secret can't be read back, so the next apply creates
// a new key.
func (r *keyOwningResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if base, ok := r.base.(resource.ResourceWithImportState); ok {
		base.ImportState(ctx, req, resp)
	}
}

func (r *keyOwningResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() {
		return
	}
	if base, ok := r.base.(resource.ResourceWithModifyPlan); ok {
		r.modifyBasePlan(ctx, base, req, resp)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	// A key missing from state (imported, or deleted outside Terraform) is planned as unknown so the
	// apply creates one; the framework would otherwise keep the null when nothing else changes.
	if req.State.Raw.IsNull() {
		return
	}
	var key types.Object
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root(r.kind.attr), &key)...)
	if !resp.Diagnostics.HasError() && key.IsNull() {
		resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root(r.kind.attr), types.ObjectUnknown(ownedKeyAttrTypes))...)
	}
}

// modifyBasePlan runs the v1 resource's plan modification on the plan without the key attribute.
func (r *keyOwningResource) modifyBasePlan(
	ctx context.Context,
	base resource.ResourceWithModifyPlan,
	req resource.ModifyPlanRequest,
	resp *resource.ModifyPlanResponse,
) {
	bs, diags := r.baseSchema(ctx)
	resp.Diagnostics.Append(diags...)
	config, d := r.strip(ctx, bs, req.Config.Raw)
	resp.Diagnostics.Append(d...)
	plan, d := r.strip(ctx, bs, resp.Plan.Raw)
	resp.Diagnostics.Append(d...)
	state, d := r.strip(ctx, bs, req.State.Raw)
	resp.Diagnostics.Append(d...)
	var key types.Object
	resp.Diagnostics.Append(resp.Plan.GetAttribute(ctx, path.Root(r.kind.attr), &key)...)
	if resp.Diagnostics.HasError() {
		return
	}

	baseResp := resource.ModifyPlanResponse{
		Plan:            tfsdk.Plan{Schema: bs, Raw: plan},
		RequiresReplace: resp.RequiresReplace,
		Private:         resp.Private,
		Deferred:        resp.Deferred,
	}
	base.ModifyPlan(ctx, resource.ModifyPlanRequest{
		Config:             tfsdk.Config{Schema: bs, Raw: config},
		Plan:               tfsdk.Plan{Schema: bs, Raw: plan},
		State:              tfsdk.State{Schema: bs, Raw: state},
		ProviderMeta:       req.ProviderMeta,
		Private:            req.Private,
		ClientCapabilities: req.ClientCapabilities,
	}, &baseResp)
	resp.Diagnostics.Append(baseResp.Diagnostics...)
	resp.RequiresReplace = baseResp.RequiresReplace
	resp.Private = baseResp.Private
	resp.Deferred = baseResp.Deferred

	keyRaw, err := key.ToTerraformValue(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error Planning "+r.kind.label, err.Error())
		return
	}
	full, err := withAttribute(resp.Plan.Schema.Type().TerraformType(ctx), baseResp.Plan.Raw, r.kind.attr, keyRaw)
	if err != nil {
		resp.Diagnostics.AddError("Error Planning "+r.kind.label, err.Error())
		return
	}
	resp.Plan.Raw = full
}

// strip returns v, an object of the v2 schema, without the key attribute, as an object of the v1 schema.
func (r *keyOwningResource) strip(ctx context.Context, bs schema.Schema, v tftypes.Value) (tftypes.Value, diag.Diagnostics) {
	var diags diag.Diagnostics
	baseType := bs.Type().TerraformType(ctx)
	if v.IsNull() {
		return tftypes.NewValue(baseType, nil), diags
	}
	if !v.IsKnown() {
		return tftypes.NewValue(baseType, tftypes.UnknownValue), diags
	}

	var attrs map[string]tftypes.Value
	if err := v.As(&attrs); err != nil {
		diags.AddError("Error Converting Resource Data", err.Error())
		return v, diags
	}
	// As shares the value's own map, so copy it before removing the key.
	attrs = maps.Clone(attrs)
	delete(attrs, r.kind.attr)
	return tftypes.NewValue(baseType, attrs), diags
}

func (r *keyOwningResource) setWithKey(ctx context.Context, state *tfsdk.State, base tftypes.Value, key types.Object, respDiags *diag.Diagnostics) {
	keyRaw, err := key.ToTerraformValue(ctx)
	if err != nil {
		respDiags.AddError("Error Saving "+r.kind.label, err.Error())
		return
	}
	full, err := withAttribute(state.Schema.Type().TerraformType(ctx), base, r.kind.attr, keyRaw)
	if err != nil {
		respDiags.AddError("Error Saving "+r.kind.label, err.Error())
		return
	}
	state.Raw = full
}

// withAttribute returns base, an object without attribute name, as an object of fullType with name set.
func withAttribute(fullType tftypes.Type, base tftypes.Value, name string, value tftypes.Value) (tftypes.Value, error) {
	if base.IsNull() {
		return tftypes.NewValue(fullType, nil), nil
	}
	var attrs map[string]tftypes.Value
	if err := base.As(&attrs); err != nil {
		return base, err
	}
	attrs = maps.Clone(attrs)
	attrs[name] = value
	if err := tftypes.ValidateValue(fullType, attrs); err != nil {
		return base, err
	}
	return tftypes.NewValue(fullType, attrs), nil
}

var ownedKeyAttrTypes = map[string]attr.Type{
	"id":         types.StringType,
	"name":       types.StringType,
	"key":        types.StringType,
	"created_at": types.StringType,
}

func ownedKeyValue(key *api.APIKey) types.Object {
	if key == nil {
		return types.ObjectNull(ownedKeyAttrTypes)
	}
	return types.ObjectValueMust(ownedKeyAttrTypes, map[string]attr.Value{
		"id":         types.StringValue(key.ID),
		"name":       types.StringValue(key.Name),
		"key":        types.StringValue(key.Key),
		"created_at": types.StringValue(key.CreatedAt),
	})
}

var nonKeyNameChars = regexp.MustCompile(`[^a-z0-9_]+`)

// ownedKeyName names a recreated key after its object, with a random suffix since names must be unique.
func ownedKeyName(objectName, suffix string) string {
	base := nonKeyNameChars.ReplaceAllString(strings.ToLower(objectName), "_")
	if len(base) > 40 {
		base = base[:40]
	}
	random := make([]byte, 3)
	_, _ = rand.Read(random)
	return fmt.Sprintf("akn_%s_%s_%s", base, suffix, hex.EncodeToString(random))
}
