package resources

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/models"
)

func NewAgentKeyV2Resource() resource.Resource {
	base := &agentKeyResource{}
	return &keyV2Resource{base: base, create: base.createV2, typeName: "_agent_key_v2", label: "agent key"}
}

// createV2 is a copy of Create that calls CreateAgentKeyV2 and keeps the secret from its response, since
// describing a v2 key doesn't return it. Keep it in sync with Create.
func (r *agentKeyResource) createV2(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Retrieve values from plan
	var plan models.AgentKey
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Create new agent key
	readOnly := false
	if !plan.ReadOnly.IsNull() {
		readOnly = plan.ReadOnly.ValueBool()
	}
	apiKey, err := r.client.CreateAgentKeyV2(
		plan.Name.ValueString(),
		plan.VirtualClusterID.ValueString(),
		readOnly,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating WarpStream Agent Key",
			"Could not create WarpStream Agent Key, unexpected error: "+err.Error(),
		)
		return
	}

	// If anything below fails, save the created key so Terraform replaces it instead of leaving it behind.
	state := models.AgentKey{
		ID:               types.StringValue(apiKey.ID),
		Name:             plan.Name,
		VirtualClusterID: plan.VirtualClusterID,
		Key:              types.StringValue(apiKey.Key),
		CreatedAt:        types.StringValue(apiKey.CreatedAt),
		ReadOnly:         types.BoolValue(readOnly),
	}

	// Describe created agent key
	described, err := r.client.GetAPIKey(apiKey.ID)
	if err != nil {
		resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
		resp.Diagnostics.AddError(
			"Error Reading WarpStream Agent Key",
			"Could not read WarpStream Agent Key ID "+apiKey.ID+": "+err.Error(),
		)
		return
	}

	virtualClusterID, found := described.GetVirtualClusterID(&resp.Diagnostics)
	if !found { // Diagnostics handled inside helper.
		resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
		return
	}

	// Map response body to schema and populate Computed attribute values
	state = models.AgentKey{
		ID:               types.StringValue(described.ID),
		Name:             types.StringValue(described.Name),
		VirtualClusterID: types.StringValue(virtualClusterID),
		Key:              types.StringValue(apiKey.Key),
		CreatedAt:        types.StringValue(described.CreatedAt),
		ReadOnly:         types.BoolValue(described.IsReadOnly()),
	}

	// Set state to fully populated data
	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}
