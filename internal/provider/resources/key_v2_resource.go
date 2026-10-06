package resources

import (
	"context"
	"fmt"
	"maps"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                   = &keyV2Resource{}
	_ resource.ResourceWithConfigure      = &keyV2Resource{}
	_ resource.ResourceWithModifyPlan     = &keyV2Resource{}
	_ resource.ResourceWithValidateConfig = &keyV2Resource{}
)

// keyV2Resource is a v2 key resource: it creates a v2 key, whose raw secret is only returned when it is
// created, and keeps that secret in state. It wraps a v1 key resource, which does everything else.
type keyV2Resource struct {
	base     resource.Resource
	create   func(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse)
	typeName string
	label    string // e.g. "agent key".
}

func (r *keyV2Resource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + r.typeName
}

func (r *keyV2Resource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if base, ok := r.base.(resource.ResourceWithConfigure); ok {
		base.Configure(ctx, req, resp)
	}
}

func (r *keyV2Resource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	r.base.Schema(ctx, req, resp)
	if resp.Diagnostics.HasError() {
		return
	}

	key, ok := resp.Schema.Attributes["key"].(schema.StringAttribute)
	if !ok {
		resp.Diagnostics.AddError("Unexpected schema", fmt.Sprintf("the %s schema has no key attribute", r.label))
		return
	}
	key.Description = fmt.Sprintf("Raw secret of the %s (v2 key format). It is only returned when the key is created "+
		"and is kept in Terraform state. If the key is deleted outside Terraform, the next apply creates a new one.", r.label)
	resp.Schema.Attributes = maps.Clone(resp.Schema.Attributes)
	resp.Schema.Attributes["key"] = key
	resp.Schema.Description += fmt.Sprintf("\nUnlike the v1 resource, this resource creates the %s in the v2 key format: "+
		"its raw secret is only returned when the key is created, so it is kept in Terraform state.\n", r.label)
}

func (r *keyV2Resource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	r.create(ctx, req, resp)
}

func (r *keyV2Resource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	r.base.Read(ctx, req, resp)
	if resp.Diagnostics.HasError() || resp.State.Raw.IsNull() {
		return
	}

	// The API never returns a v2 key's secret, so keep the one stored when the key was created.
	var secret types.String
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("key"), &secret)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("key"), secret)...)
}

func (r *keyV2Resource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	r.base.Update(ctx, req, resp)
}

func (r *keyV2Resource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	r.base.Delete(ctx, req, resp)
}

func (r *keyV2Resource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	if base, ok := r.base.(resource.ResourceWithValidateConfig); ok {
		base.ValidateConfig(ctx, req, resp)
	}
}

func (r *keyV2Resource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if base, ok := r.base.(resource.ResourceWithModifyPlan); ok {
		base.ModifyPlan(ctx, req, resp)
	}
}
