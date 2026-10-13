package resources

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/api"
)

// fakeBaseResource stands in for a v1 resource with an ID and a name.
type fakeBaseResource struct{}

func (fakeBaseResource) Metadata(context.Context, resource.MetadataRequest, *resource.MetadataResponse) {
}

func (fakeBaseResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{Attributes: map[string]schema.Attribute{
		"id":   schema.StringAttribute{Computed: true},
		"name": schema.StringAttribute{Required: true},
	}}
}

func (fakeBaseResource) Create(context.Context, resource.CreateRequest, *resource.CreateResponse) {}

func (fakeBaseResource) Read(_ context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	resp.State = req.State
}

func (fakeBaseResource) Update(_ context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.State.Raw = req.Plan.Raw
}

func (fakeBaseResource) Delete(context.Context, resource.DeleteRequest, *resource.DeleteResponse) {}

func newFlagTestResource(createdKey *api.APIKey, gotCreateKey *bool) *keyOwningResource {
	return &keyOwningResource{
		base:     fakeBaseResource{},
		typeName: "_test_v2",
		kind:     agentKeyKind,
		create: func(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse, createKey bool) (string, *api.APIKey) {
			*gotCreateKey = createKey
			var name types.String
			resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("name"), &name)...)
			resp.State.Raw = tftypes.NewValue(resp.State.Schema.Type().TerraformType(ctx), map[string]tftypes.Value{
				"id":   tftypes.NewValue(tftypes.String, "vci_1"),
				"name": tftypes.NewValue(tftypes.String, name.ValueString()),
			})
			if !createKey {
				return "vci_1", nil
			}
			return "vci_1", createdKey
		},
	}
}

func flagTestSchema(t *testing.T, r *keyOwningResource) schema.Schema {
	t.Helper()
	var resp resource.SchemaResponse
	r.Schema(context.Background(), resource.SchemaRequest{}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	return resp.Schema
}

// flagTestValue builds a resource value; createKey nil means unset, unknownKey plans the key as unknown.
func flagTestValue(t *testing.T, s schema.Schema, id tftypes.Value, key *api.APIKey, unknownKey bool, createKey *bool) tftypes.Value {
	t.Helper()
	ctx := context.Background()
	keyValue := ownedKeyValue(key)
	if unknownKey {
		keyValue = types.ObjectUnknown(ownedKeyAttrTypes)
	}
	keyRaw, err := keyValue.ToTerraformValue(ctx)
	if err != nil {
		t.Fatal(err)
	}
	flag := types.BoolNull()
	if createKey != nil {
		flag = types.BoolValue(*createKey)
	}
	flagRaw, err := flag.ToTerraformValue(ctx)
	if err != nil {
		t.Fatal(err)
	}
	return tftypes.NewValue(s.Type().TerraformType(ctx), map[string]tftypes.Value{
		"id":               id,
		"name":             tftypes.NewValue(tftypes.String, "vcn_test"),
		"agent_key":        keyRaw,
		"create_agent_key": flagRaw,
	})
}

func TestKeyOwningResourceCreatesKeyOnlyWhenAsked(t *testing.T) {
	ctx := context.Background()
	for _, createKey := range []bool{false, true} {
		var got bool
		r := newFlagTestResource(&api.APIKey{ID: "aki_1", Key: "aks_v2_secret"}, &got)
		s := flagTestSchema(t, r)
		plan := flagTestValue(t, s, tftypes.NewValue(tftypes.String, tftypes.UnknownValue), nil, true, &createKey)
		resp := resource.CreateResponse{State: tfsdk.State{Schema: s, Raw: tftypes.NewValue(s.Type().TerraformType(ctx), nil)}}
		r.Create(ctx, resource.CreateRequest{Config: tfsdk.Config{Schema: s, Raw: plan}, Plan: tfsdk.Plan{Schema: s, Raw: plan}}, &resp)
		if resp.Diagnostics.HasError() {
			t.Fatal(resp.Diagnostics)
		}
		if got != createKey {
			t.Fatalf("create called with createKey %v, want %v", got, createKey)
		}

		var key types.Object
		var flag types.Bool
		resp.Diagnostics.Append(resp.State.GetAttribute(ctx, path.Root("agent_key"), &key)...)
		resp.Diagnostics.Append(resp.State.GetAttribute(ctx, path.Root("create_agent_key"), &flag)...)
		if resp.Diagnostics.HasError() {
			t.Fatal(resp.Diagnostics)
		}
		if key.IsNull() == createKey || flag.ValueBool() != createKey {
			t.Fatalf("createKey %v: state key %v, flag %v", createKey, key, flag)
		}
	}
}

func TestKeyOwningResourceModifyPlan(t *testing.T) {
	yes, no := true, false
	id := tftypes.NewValue(tftypes.String, "vci_1")
	for _, tc := range []struct {
		name        string
		create      bool  // No prior state.
		prior       *bool // Flag in state; nil for an imported object.
		planned     bool
		wantErr     bool
		wantUnknown bool // Whether the key is planned to be created; otherwise it must be planned as null.
	}{
		{name: "create without key", create: true, planned: false},
		{name: "create with key", create: true, planned: true, wantUnknown: true},
		{name: "no key wanted", prior: &no, planned: false},
		{name: "missing key is recreated", prior: &yes, planned: true, wantUnknown: true},
		{name: "flag turned on", prior: &no, planned: true, wantErr: true},
		{name: "flag turned off", prior: &yes, planned: false, wantErr: true},
		{name: "imported with key wanted", prior: nil, planned: true, wantUnknown: true},
		{name: "imported without key", prior: nil, planned: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			var got bool
			r := newFlagTestResource(nil, &got)
			s := flagTestSchema(t, r)
			state := flagTestValue(t, s, id, nil, false, tc.prior)
			if tc.create {
				state = tftypes.NewValue(s.Type().TerraformType(ctx), nil)
			}
			// The framework plans unset computed attributes as unknown whenever anything changes.
			plan := flagTestValue(t, s, id, nil, true, &tc.planned)
			resp := resource.ModifyPlanResponse{Plan: tfsdk.Plan{Schema: s, Raw: plan}}
			r.ModifyPlan(ctx, resource.ModifyPlanRequest{
				Config: tfsdk.Config{Schema: s, Raw: plan},
				Plan:   tfsdk.Plan{Schema: s, Raw: plan},
				State:  tfsdk.State{Schema: s, Raw: state},
			}, &resp)
			if resp.Diagnostics.HasError() != tc.wantErr {
				t.Fatalf("diagnostics %v, want error %v", resp.Diagnostics, tc.wantErr)
			}
			if tc.wantErr {
				return
			}
			var key types.Object
			resp.Diagnostics.Append(resp.Plan.GetAttribute(ctx, path.Root("agent_key"), &key)...)
			if resp.Diagnostics.HasError() {
				t.Fatal(resp.Diagnostics)
			}
			if key.IsUnknown() != tc.wantUnknown || (!tc.wantUnknown && !key.IsNull()) {
				t.Fatalf("planned key %v, want unknown %v", key, tc.wantUnknown)
			}
		})
	}
}
