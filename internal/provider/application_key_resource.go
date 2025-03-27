package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/api"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/utils"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource              = &applicationKeyResource{}
	_ resource.ResourceWithConfigure = &applicationKeyResource{}
)

// NewApplicationKeyResource is a helper function to simplify the provider implementation.
func NewApplicationKeyResource() resource.Resource {
	return &applicationKeyResource{}
}

// applicationKeyResource is the resource implementation.
type applicationKeyResource struct {
	client *api.Client
}

// Configure adds the provider configured client to the data source.
func (r *applicationKeyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (r *applicationKeyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_application_key"
}

// Schema defines the schema for the resource.
func (r *applicationKeyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: `
This resource allows you to create, update and delete application keys.
`,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Application Key ID.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "Application Key Name. " +
					"Must be unique across WarpStream account. " +
					"Must start with 'akn_' and contain underscores and alphanumeric characters only. " +
					"Cannot be changed after creation.",
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{utils.StartsWithAndAlphanumeric("akn_")},
			},
			"key": schema.StringAttribute{
				Description: "Application Key Secret Value.",
				Computed:    true,
				Sensitive:   true,
			},
			"workspace_id": schema.StringAttribute{
				Description: "Workspace ID. " +
					"ID of the workspace in which the application key is authorized to manage resources " +
					"Must be a valid workspace ID starting with 'wi_'. " +
					"If empty, defaults to the oldest workspace that the provided WarpStream API key is authorized to access. " +
					"Cannot be changed after creation.",
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					utils.StartsWithAndAlphanumeric("wi_"),
				},
			},
			"created_at": schema.StringAttribute{
				Description: "Application Key Creation Timestamp.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

// Create a new resource.
func (r *applicationKeyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Retrieve values from plan
	var plan applicationKeyModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Create new application key
	apiKey, err := r.client.CreateApplicationKey(
		plan.Name.ValueString(),
		plan.WorkspaceID.ValueString(),
	)

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
	apiKey, err = r.client.GetAPIKey(apiKey.ID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading WarpStream Application Key",
			"Could not read WarpStream Application Key ID "+apiKey.ID+": "+err.Error(),
		)
		return
	}

	// Map response body to schema and populate Computed attribute values
	state := applicationKeyModel{
		ID:          types.StringValue(apiKey.ID),
		Name:        types.StringValue(apiKey.Name),
		Key:         types.StringValue(apiKey.Key),
		WorkspaceID: types.StringValue(readWorkspaceIDSafe(apiKey.AccessGrants)),
		CreatedAt:   types.StringValue(apiKey.CreatedAt),
	}

	// Set state to fully populated data
	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Read refreshes the Terraform state with the latest data.
func (r *applicationKeyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state applicationKeyModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiKey, err := r.client.GetAPIKey(state.ID.ValueString())
	if err != nil {
		if errors.Is(err, api.ErrNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError(
			"Error Reading WarpStream Application Key",
			"Could not read WarpStream Application Key ID "+state.ID.ValueString()+": "+err.Error(),
		)
		return
	}

	// Overwrite Application Key with refreshed state
	state = applicationKeyModel{
		ID:          types.StringValue(apiKey.ID),
		Name:        types.StringValue(apiKey.Name),
		Key:         types.StringValue(apiKey.Key),
		WorkspaceID: types.StringValue(readWorkspaceIDSafe(apiKey.AccessGrants)),
		// WorkspaceID: types.StringUnknown(),
		CreatedAt: types.StringValue(apiKey.CreatedAt),
	}

	// Set state
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func readWorkspaceIDSafe(grants []api.AccessGrant) string {
	if len(grants) == 0 {
		return ""
	}

	return grants[0].WorkspaceID
}

// Update updates the resource and sets the updated Terraform state on success.
// Application keys are immutable but we define this to implement resource.Resource.
func (r *applicationKeyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *applicationKeyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Retrieve values from state
	var state applicationKeyModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Delete existing application key
	err := r.client.DeleteAPIKey(state.ID.ValueString())
	if err != nil {
		if errors.Is(err, api.ErrNotFound) {
			return
		}

		resp.Diagnostics.AddError(
			"Error Deleting WarpStream Application Key",
			"Could not delete WarpStream Application Key, unexpected error: "+err.Error(),
		)
		return
	}
}
