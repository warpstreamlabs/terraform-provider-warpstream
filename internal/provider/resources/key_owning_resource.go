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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
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
	flag  string // Schema attribute that asks for the key, e.g. "create_agent_key".
	label string // e.g. "agent key".
	// recreate creates a replacement v2 key for the object with this ID and name.
	recreate func(client *api.Client, objectID, objectName string) (*api.APIKey, error)
}

var agentKeyKind = ownedKeyKind{
	attr:  "agent_key",
	flag:  "create_agent_key",
	label: "agent key",
	recreate: func(client *api.Client, clusterID, clusterName string) (*api.APIKey, error) {
		return client.CreateAgentKeyV2(ownedKeyName(strings.TrimPrefix(clusterName, "vcn_"), "agent_key"), clusterID, false)
	},
}

// createV2Func creates the object, and a v2 key with it if createKey is set, working on the plan and
// state without the key attributes, and returns the new object's ID and the key. The ID is empty if
// nothing was created.
type createV2Func func(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse, createKey bool) (string, *api.APIKey)

// keyOwningResource is a v2 resource: it creates its object, and a v2 key with it when asked, and stores
// that key, whose secret is only returned once, in state. It wraps a v1 resource, which does all other
// object work on a copy of the plan and state without the key attributes.
type keyOwningResource struct {
	base     resource.Resource
	create   createV2Func
	typeName string
	kind     ownedKeyKind
	client   *api.Client
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
	s.Attributes[r.kind.flag] = schema.BoolAttribute{
		Description: fmt.Sprintf("Whether to create an %s (v2 key format) with the object and manage it, see `%s`. "+
			"Defaults to false. Cannot be changed after creation.", r.kind.label, r.kind.attr),
		Optional: true,
		Computed: true,
		Default:  booldefault.StaticBool(false),
	}
	s.Attributes[r.kind.attr] = schema.SingleNestedAttribute{
		Description: fmt.Sprintf("The %s (v2 key format) created and managed by this resource when `%s` is true. Its raw "+
			"secret is only returned when the key is created and is kept in Terraform state. If the key is deleted "+
			"outside Terraform, the next apply creates a new one.", r.kind.label, r.kind.flag),
		Computed: true,
		Attributes: map[string]schema.Attribute{
			"id":         schema.StringAttribute{Description: "Key ID.", Computed: true},
			"name":       schema.StringAttribute{Description: "Key name.", Computed: true},
			"key":        schema.StringAttribute{Description: "Key secret.", Computed: true, Sensitive: true},
			"created_at": schema.StringAttribute{Description: "Key creation timestamp.", Computed: true},
		},
		PlanModifiers: []planmodifier.Object{objectplanmodifier.UseStateForUnknown()},
	}
	s.Description = fmt.Sprintf("%s\n\nUnlike the v1 resource, this resource can also create an %s (v2 key format) "+
		"with the object and manage it, see `%s`.", s.Description, r.kind.label, r.kind.flag)
	resp.Schema = s
}

func (r *keyOwningResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	bs, diags := r.baseSchema(ctx)
	resp.Diagnostics.Append(diags...)
	config, d := r.strip(ctx, bs, req.Config.Raw)
	resp.Diagnostics.Append(d...)
	plan, d := r.strip(ctx, bs, req.Plan.Raw)
	resp.Diagnostics.Append(d...)
	var createKey types.Bool
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root(r.kind.flag), &createKey)...)
	if resp.Diagnostics.HasError() {
		return
	}

	baseResp := resource.CreateResponse{
		State:   tfsdk.State{Schema: bs, Raw: tftypes.NewValue(bs.Type().TerraformType(ctx), nil)},
		Private: resp.Private,
	}
	objectID, createdKey := r.create(ctx, resource.CreateRequest{
		Config:       tfsdk.Config{Schema: bs, Raw: config},
		Plan:         tfsdk.Plan{Schema: bs, Raw: plan},
		ProviderMeta: req.ProviderMeta,
	}, &baseResp, createKey.ValueBool())
	resp.Diagnostics.Append(baseResp.Diagnostics...)
	resp.Private = baseResp.Private

	key := ownedKeyValue(createdKey)
	if !baseResp.State.Raw.IsNull() {
		r.setWithKey(ctx, &resp.State, baseResp.State.Raw, key, createKey, &resp.Diagnostics)
		return
	}
	if objectID == "" {
		return
	}

	// The object exists but Create saved no state: save what we know so Terraform taints and replaces
	// it rather than orphaning the object and its key.
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
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), objectID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root(r.kind.attr), key)...)
}

func (r *keyOwningResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	bs, diags := r.baseSchema(ctx)
	resp.Diagnostics.Append(diags...)
	state, d := r.strip(ctx, bs, req.State.Raw)
	resp.Diagnostics.Append(d...)
	var key types.Object
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root(r.kind.attr), &key)...)
	var createKey types.Bool
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root(r.kind.flag), &createKey)...)
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
	r.setWithKey(ctx, &resp.State, baseResp.State.Raw, key, createKey, &resp.Diagnostics)
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
	var createKey types.Bool
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root(r.kind.flag), &createKey)...)
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
	r.setWithKey(ctx, &resp.State, baseResp.State.Raw, key, createKey, &resp.Diagnostics)
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

	var planned types.Bool
	resp.Diagnostics.Append(resp.Plan.GetAttribute(ctx, path.Root(r.kind.flag), &planned)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !req.State.Raw.IsNull() {
		var prior types.Bool
		resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root(r.kind.flag), &prior)...)
		if resp.Diagnostics.HasError() {
			return
		}
		// An imported object has no flag in state yet, so it can be set to either value.
		if !prior.IsNull() && !planned.IsUnknown() && !planned.Equal(prior) {
			resp.Diagnostics.AddAttributeError(path.Root(r.kind.flag), "Cannot change "+r.kind.flag,
				fmt.Sprintf("%s cannot be changed after creation. To add a key to an existing object, use a separate key resource.", r.kind.flag))
			return
		}
	}
	if planned.IsUnknown() {
		return
	}
	if !planned.ValueBool() {
		// No key is wanted: plan it as null, not as unknown, which the framework does whenever anything changes.
		resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root(r.kind.attr), ownedKeyValue(nil))...)
		return
	}
	if req.State.Raw.IsNull() {
		return
	}

	// A key missing from state (imported, or deleted outside Terraform) is planned as unknown so the
	// apply creates one; the framework would otherwise keep the null when nothing else changes.
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
	var createKey types.Bool
	resp.Diagnostics.Append(resp.Plan.GetAttribute(ctx, path.Root(r.kind.flag), &createKey)...)
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

	full, err := r.withKey(ctx, resp.Plan.Schema.Type().TerraformType(ctx), baseResp.Plan.Raw, key, createKey)
	if err != nil {
		resp.Diagnostics.AddError("Error Planning "+r.kind.label, err.Error())
		return
	}
	resp.Plan.Raw = full
}

// strip returns v, an object of the v2 schema, without the key attributes, as an object of the v1 schema.
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
	delete(attrs, r.kind.flag)
	return tftypes.NewValue(baseType, attrs), diags
}

func (r *keyOwningResource) setWithKey(
	ctx context.Context,
	state *tfsdk.State,
	base tftypes.Value,
	key types.Object,
	createKey types.Bool,
	respDiags *diag.Diagnostics,
) {
	full, err := r.withKey(ctx, state.Schema.Type().TerraformType(ctx), base, key, createKey)
	if err != nil {
		respDiags.AddError("Error Saving "+r.kind.label, err.Error())
		return
	}
	state.Raw = full
}

// withKey returns base, an object of the v1 schema, as an object of fullType with the key attributes set.
func (r *keyOwningResource) withKey(ctx context.Context, fullType tftypes.Type, base tftypes.Value, key types.Object, createKey types.Bool) (tftypes.Value, error) {
	keyRaw, err := key.ToTerraformValue(ctx)
	if err != nil {
		return base, err
	}
	flagRaw, err := createKey.ToTerraformValue(ctx)
	if err != nil {
		return base, err
	}
	return withAttributes(fullType, base, map[string]tftypes.Value{r.kind.attr: keyRaw, r.kind.flag: flagRaw})
}

// withAttributes returns base, an object without the given attributes, as an object of fullType with them set.
func withAttributes(fullType tftypes.Type, base tftypes.Value, values map[string]tftypes.Value) (tftypes.Value, error) {
	if base.IsNull() {
		return tftypes.NewValue(fullType, nil), nil
	}
	var attrs map[string]tftypes.Value
	if err := base.As(&attrs); err != nil {
		return base, err
	}
	attrs = maps.Clone(attrs)
	maps.Copy(attrs, values)
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
