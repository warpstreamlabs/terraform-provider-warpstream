package resources

import (
	"context"
	"errors"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/api"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/models"
)

func NewApplicationKeyV2Resource() resource.Resource {
	base := &applicationKeyResource{}
	return &keyV2Resource{base: base, create: base.createV2, typeName: "_application_key_v2", label: "application key"}
}

// createV2 is a copy of Create that calls CreateApplicationKeyV2 or CreateClusterScopedApplicationKeyV2
// and keeps the secret from the response, since describing a v2 key doesn't return it. Keep it in sync
// with Create.
func (r *applicationKeyResource) createV2(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Retrieve values from plan
	var plan models.ApplicationKey
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	clusterScoped := !plan.VirtualClusterID.IsNull() && !plan.VirtualClusterID.IsUnknown() &&
		plan.VirtualClusterID.ValueString() != ""

	readOnly := false
	if !plan.ReadOnly.IsNull() && !plan.ReadOnly.IsUnknown() {
		readOnly = plan.ReadOnly.ValueBool()
	}

	if clusterScoped && readOnly {
		addClusterScopedReadOnlyError(&resp.Diagnostics)
		return
	}

	var (
		apiKey *api.APIKey
		err    error
	)

	if clusterScoped {
		apiKey, err = r.client.CreateClusterScopedApplicationKeyV2(
			plan.Name.ValueString(),
			plan.WorkspaceID.ValueString(),
			plan.VirtualClusterID.ValueString(),
			plan.ResourceKind.ValueString(),
		)
	} else {
		apiKey, err = r.client.CreateApplicationKeyV2(
			plan.Name.ValueString(),
			plan.WorkspaceID.ValueString(),
			readOnly,
		)
	}

	// TODO: Make client return an structured HTTP error and branch on the specific case where it's the workspace that's not found.
	if err != nil && errors.Is(err, api.ErrNotFound) {
		resp.Diagnostics.AddError(
			"Error Creating WarpStream Application Key",
			"Could not create WarpStream Application Key, workspace not found. "+
				"Either the workspace "+plan.WorkspaceID.ValueString()+" doesn't exist, or the API key used to authenticate "+
				"this provider doesn't have access to it.",
		)
		return
	}

	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating WarpStream Application Key",
			"Could not create WarpStream Application Key, unexpected error: "+err.Error(),
		)
		return
	}

	// Describe created application key
	described, err := r.client.GetAPIKey(apiKey.ID)
	if err != nil {
		// Save the created key so Terraform replaces it instead of leaving it behind.
		state := models.ApplicationKeyFromAPI(*apiKey)
		resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
		resp.Diagnostics.AddError(
			"Error Reading WarpStream Application Key",
			"Could not read WarpStream Application Key ID "+apiKey.ID+": "+err.Error(),
		)
		return
	}

	// Map response body to schema and populate Computed attribute values
	state := models.ApplicationKeyFromAPI(*described)
	state.Key = types.StringValue(apiKey.Key)

	// Set state to fully populated data
	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}
