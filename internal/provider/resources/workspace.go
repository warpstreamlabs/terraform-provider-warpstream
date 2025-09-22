package resources

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/api"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/models"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/utils"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &workspaceResource{}
	_ resource.ResourceWithConfigure   = &workspaceResource{}
	_ resource.ResourceWithImportState = &workspaceResource{}
)

// NewWorkspaceResource is a helper function to simplify the provider implementation.
func NewWorkspaceResource() resource.Resource {
	return &workspaceResource{}
}

// workspaceResource is the resource implementation.
type workspaceResource struct {
	client *api.Client
}

// Configure adds the provider configured client to the data source.
func (r *workspaceResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*api.Client)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *api.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.client = client
}

// Metadata returns the resource type name.
func (r *workspaceResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_workspace"
}

// Schema defines the schema for the resource.
func (r *workspaceResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: `
This resource allows you to create and delete workspaces.

The WarpStream provider must be authenticated with an account key to consume this resource.

Not all customers have the workspaces feature enabled. Please contact Support for details.
`,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Workspace ID.",
				Computed:    true,
				Required:    false,
				Optional:    false,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "Workspace Name. " +
					"Must be unique across WarpStream account. " +
					"Must contain spaces, hyphens, underscores and alphanumeric characters only. " +
					"Must be between 3 and 128 characters in length.",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				Validators:    []validator.String{utils.ValidWorkspaceName()},
			},
			"created_at": schema.StringAttribute{
				Description: "Workspace Creation Timestamp.",
				Computed:    true,
				Required:    false,
				Optional:    false,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

// Create a new resource.
func (r *workspaceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Retrieve values from plan
	var plan models.Workspace
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Create new workspace
	newWorkspaceID, err := r.client.CreateWorkspace(plan.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating WarpStream Workspace",
			"Could not create WarpStream Workspace, unexpected error: "+err.Error(),
		)
		return
	}

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
}

// Read refreshes the Terraform state with the latest data.
func (r *workspaceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state models.Workspace
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	workspace, err := r.client.GetWorkspace(state.ID.ValueString())
	if err != nil {
		if errors.Is(err, api.ErrNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError(
			"Error Reading WarpStream Workspace",
			"Could not read WarpStream Workspace ID "+state.ID.ValueString()+": "+err.Error(),
		)
		return
	}

	// Overwrite Workspace with refreshed state
	state = models.Workspace{
		ID:        types.StringValue(workspace.ID),
		Name:      types.StringValue(workspace.Name),
		CreatedAt: types.StringValue(workspace.CreatedAt),
	}

	// Set state
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Update can only modify the workspace's name.
func (r *workspaceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Retrieve values from plan.
	var plan models.Workspace
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Describe the workspace.
	workspaceID := plan.ID.ValueString()
	workspace, err := r.client.GetWorkspace(workspaceID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading WarpStream Workspace",
			"Could not read WarpStream Workspace ID "+workspaceID+": "+err.Error(),
		)
		return
	}

	// If the workspace name is the same as in the plan, do nothing.
	if workspace.Name == plan.Name.ValueString() {
		return
	}

	// Update workspace name.
	err = r.client.RenameWorkspace(plan.ID.ValueString(), plan.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating WarpStream Workspace",
			"Could not update WarpStream Workspace, unexpected error: "+err.Error(),
		)
		return
	}

	// Set state to fully populated data.
	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *workspaceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Retrieve values from state
	var state models.Workspace
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Delete existing workspace
	err := r.client.DeleteWorkspace(state.ID.ValueString())
	if err != nil {
		if errors.Is(err, api.ErrNotFound) {
			return
		}

		resp.Diagnostics.AddError(
			"Error Deleting WarpStream Workspace",
			"Could not delete WarpStream Workspace, unexpected error: "+err.Error(),
		)
		return
	}
}

func (r *workspaceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Retrieve import ID and save to id attribute
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
