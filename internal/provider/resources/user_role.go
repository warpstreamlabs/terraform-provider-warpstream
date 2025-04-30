package resources

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
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
	_ resource.Resource              = &userRoleResource{}
	_ resource.ResourceWithConfigure = &userRoleResource{}
	// Names of the managed grants that can be assigned to a role inside a workspace.
	managedGrantNames = []string{"admin", "read_only"}
)

// NewUserRoleResource is a helper function to simplify the provider implementation.
func NewUserRoleResource() resource.Resource {
	return &userRoleResource{}
}

// userRoleResource is the resource implementation.
type userRoleResource struct {
	client *api.Client
}

// Configure adds the provider configured client to the data source.
func (r *userRoleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (r *userRoleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user_role"
}

var grantSchema = schema.NestedAttributeObject{
	Attributes: map[string]schema.Attribute{
		"workspace_id": schema.StringAttribute{
			Description: "ID of a workspace that the role has access to.",
			Validators: []validator.String{
				stringvalidator.Any(utils.StartsWith("wi_"), stringvalidator.OneOf("*")),
			},
			PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			Required:      true,
		},
		"grant_type": schema.StringAttribute{
			Description:   "Level of access inside the workspace. Current options are: " + strings.Join(managedGrantNames, " and "),
			Validators:    []validator.String{stringvalidator.OneOf(managedGrantNames...)},
			PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			// For now a grant is defined using a grant type only.
			// In the future we may loosen the schema to so that grants can be defined using either a just grant type or more detailed grant attributes.
			Required: true,
		},
	},
}

// Schema defines the schema for the resource.
func (r *userRoleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: `
This resource allows you to create and delete user roles.

The WarpStream provider must be authenticated with an account key to consume this resource.
`,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "User Role ID.",
				Computed:    true,
				Required:    false,
				Optional:    false,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "User Role Name. " +
					"Must be unique across WarpStream account. " +
					"Must contain spaces, hyphens, underscores and alphanumeric characters only. " +
					"Must be between 3 and 128 characters in length.",
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{utils.ValidWorkspaceName()}, // Same rule as for workspace names.
			},
			"access_grants": schema.ListNestedAttribute{
				Description:  "List of grants defining the role's access level inside each workspace.",
				Required:     true,
				NestedObject: grantSchema,
			},
			"created_at": schema.StringAttribute{
				Description: "User Role Creation Timestamp.",
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
func (r *userRoleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Retrieve values from plan
	var plan models.UserRole
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	grants := make([]api.AccessGrant, 0, len(plan.AccessGrants))
	for _, grant := range plan.AccessGrants {
		grants = append(grants, api.AccessGrant{
			WorkspaceID:     grant.WorkspaceID.ValueString(),
			ManagedGrantKey: grant.GrantType.ValueString(),
			ResourceID:      "*", // For now, always create roles with access to any resource in its workspaces.
		})
	}

	// Create the new role
	newRoleID, err := r.client.CreateUserRole(plan.Name.ValueString(), grants)
	if err != nil {
		details := "Could not create WarpStream User Role, unexpected error: " + err.Error()
		// TODO: Make the API client return more specific errors so that we know for sure when the 404 is due to a missing workspace.
		if errors.Is(err, api.ErrNotFound) {
			details += fmt.Sprintf("\nAre you sure the workspace with ID %s exists?", grants[0].WorkspaceID)
		}

		resp.Diagnostics.AddError(
			"Error creating WarpStream User Role",
			details,
		)

		return
	}

	// Describe created role
	role, err := r.client.GetUserRole(newRoleID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading WarpStream User Role",
			"Could not read WarpStream User Role ID "+newRoleID+": "+err.Error(),
		)
		return
	}

	// Map response body to schema and populate Computed attribute values
	state := models.UserRole{
		ID:        types.StringValue(role.ID),
		Name:      types.StringValue(role.Name),
		CreatedAt: types.StringValue(role.CreatedAt),
	}
	grantModels := make([]models.UserRoleGrant, 0, len(role.AccessGrants))
	for _, grant := range role.AccessGrants {
		grantModels = append(grantModels, models.UserRoleGrant{
			WorkspaceID: types.StringValue(grant.WorkspaceID),
			GrantType:   types.StringValue(grant.ManagedGrantKey),
		})
	}
	state.AccessGrants = grantModels

	// Set state to fully populated data
	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Read refreshes the Terraform state with the latest data.
func (r *userRoleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state models.UserRole
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	role, err := r.client.GetUserRole(state.ID.ValueString())
	if err != nil {
		if errors.Is(err, api.ErrNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError(
			"Error Reading WarpStream User Role",
			"Could not read WarpStream User Role ID "+state.ID.ValueString()+": "+err.Error(),
		)
		return
	}

	// Overwrite Role with refreshed state
	state = models.UserRole{
		ID:        types.StringValue(role.ID),
		Name:      types.StringValue(role.Name),
		CreatedAt: types.StringValue(role.CreatedAt),
	}

	grants := make([]models.UserRoleGrant, 0, len(role.AccessGrants))
	for _, grant := range role.AccessGrants {
		grants = append(grants, models.UserRoleGrant{
			WorkspaceID: types.StringValue(grant.WorkspaceID),
			GrantType:   types.StringValue(grant.ManagedGrantKey),
		})
	}
	state.AccessGrants = grants

	// Set state
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Update updates the resource and sets the updated Terraform state on success.
// Roles can't be modified yet but we define this to implement resource.Resource.
func (r *userRoleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *userRoleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Retrieve values from state
	var state models.UserRole
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Delete existing role
	err := r.client.DeleteUserRole(state.ID.ValueString())
	if err != nil {
		if errors.Is(err, api.ErrNotFound) {
			return
		}

		resp.Diagnostics.AddError(
			"Error Deleting WarpStream Role",
			"Could not delete WarpStream Role, unexpected error: "+err.Error(),
		)
		return
	}
}
