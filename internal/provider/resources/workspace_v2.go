package resources

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/api"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/models"
)

var applicationKeyKind = ownedKeyKind{
	attr:  "application_key",
	label: "application key",
	recreate: func(client *api.Client, workspaceID, workspaceName string) (*api.APIKey, error) {
		return client.CreateApplicationKeyV2(ownedKeyName(workspaceName, "application_key"), workspaceID, false)
	},
}

func NewWorkspaceV2Resource() resource.Resource {
	base := &workspaceResource{}
	return &keyOwningResource{base: base, create: base.createV2, typeName: "_workspace_v2", kind: applicationKeyKind}
}

// createV2 is a copy of Create that calls CreateWorkspaceV2, which also creates a v2 application key.
// Keep it in sync with Create.
func (r *workspaceResource) createV2(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) (objectID string, key *api.APIKey) {
	// Retrieve values from plan
	var plan models.Workspace
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Create new workspace
	newWorkspaceID, applicationKey, err := r.client.CreateWorkspaceV2(plan.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating WarpStream Workspace",
			"Could not create WarpStream Workspace, unexpected error: "+err.Error(),
		)
		return
	}
	objectID, key = newWorkspaceID, applicationKey

	// Describe created workspace
	workspace, err := r.client.GetWorkspace(newWorkspaceID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading WarpStream Workspace",
			"Could not read WarpStream Workspace ID "+newWorkspaceID+": "+err.Error(),
		)
		return
	}

	// Map response body to schema and populate Computed attribute values
	state := models.Workspace{
		ID:        types.StringValue(workspace.ID),
		Name:      types.StringValue(workspace.Name),
		CreatedAt: types.StringValue(workspace.CreatedAt),
	}

	// Set state to fully populated data
	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	return
}
