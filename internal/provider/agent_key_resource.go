package provider

import (
	"context"
	"fmt"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/api"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource              = &agentKeyResource{}
	_ resource.ResourceWithConfigure = &agentKeyResource{}
)

// NewVirtualClusterResource is a helper function to simplify the provider implementation.
func NewAgentKeyResource() resource.Resource {
	return &agentKeyResource{}
}

// agentKeyResource is the resource implementation.
type agentKeyResource struct {
	client *api.Client
}

// Configure adds the provider configured client to the data source.
func (r *agentKeyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (r *agentKeyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_agent_key"
}

// Schema defines the schema for the resource.
func (r *agentKeyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: `
This resource allows you to create, update and delete agent keys.
`,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Agent Key ID.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "Agent Key Name. " +
					"Must be unique across WarpStream account. " +
					"Must start with 'akn_' and contain underscores and alphanumeric characters only. " +
					"Cannot be changed after creation.",
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^akn_[a-zA-Z0-9_]+$`),
						"must start with 'akn_' and must contain underscores and alphanumeric characters only",
					),
				},
			},
			"key": schema.StringAttribute{
				Description: "Agent Key Secret Value.",
				Computed:    true,
				Sensitive:   true,
			},
			"virtual_cluster_id": schema.StringAttribute{
				Description: "Virtual Cluster ID associated with the Agent Key.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"created_at": schema.StringAttribute{
				Description: "Agent Key Creation Timestamp.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

// Create a new resource.
func (r *agentKeyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Retrieve values from plan
	var plan agentKeyModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Create new agent key
	apiKey, err := r.client.CreateAgentKey(
		plan.Name.ValueString(),
		plan.VirtualClusterID.ValueString(),
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating WarpStream Agent Key",
			"Could not create WarpStream Agent Key, unexpected error: "+err.Error(),
		)
		return
	}

	// Describe created agent key
	apiKey, err = r.client.GetAPIKey(apiKey.ID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading WarpStream Agent Key",
			"Could not read WarpStream Agent Key ID "+apiKey.ID+": "+err.Error(),
		)
		return
	}

	virtualClusterID, found := getVirtualClusterID(*apiKey, &resp.Diagnostics)
	if !found { // Diagnostics handled inside helper.
		return
	}

	// Map response body to schema and populate Computed attribute values
	state := agentKeyModel{
		ID:               types.StringValue(apiKey.ID),
		Name:             types.StringValue(apiKey.Name),
		VirtualClusterID: types.StringValue(virtualClusterID),
		Key:              types.StringValue(apiKey.Key),
		CreatedAt:        types.StringValue(apiKey.CreatedAt),
	}

	// Set state to fully populated data
	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func getVirtualClusterID(apiKey api.APIKey, diags *diag.Diagnostics) (string, bool) {
	if len(apiKey.AccessGrants) == 0 {
		diags.AddError(
			"Error Reading WarpStream Agent Key",
			"API returned invalid Agent Key with ID "+apiKey.ID+": no access grants found",
		)
		return "", false
	}

	return apiKey.AccessGrants[0].ResourceID, true
}

// Read refreshes the Terraform state with the latest data.
func (r *agentKeyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state agentKeyModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiKey, err := r.client.GetAPIKey(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading WarpStream Agent Key",
			"Could not read WarpStream Agent Key ID "+state.ID.ValueString()+": "+err.Error(),
		)
		return
	}

	// Overwrite Agent Key with refreshed state
	virtualClusterID, found := getVirtualClusterID(*apiKey, &resp.Diagnostics)
	if !found { // Diagnostics handled inside helper.
		return
	}

	state = agentKeyModel{
		ID:               types.StringValue(apiKey.ID),
		Name:             types.StringValue(apiKey.Name),
		Key:              types.StringValue(apiKey.Key),
		VirtualClusterID: types.StringValue(virtualClusterID),
		CreatedAt:        types.StringValue(apiKey.CreatedAt),
	}

	// Set state
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Update updates the resource and sets the updated Terraform state on success.
// Agent keys are immutable but we define this to implement resource.Resource.
func (r *agentKeyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *agentKeyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Retrieve values from state
	var state agentKeyModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Delete existing agent key
	err := r.client.DeleteAPIKey(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting WarpStream Agent Key",
			"Could not delete WarpStream Agent Key, unexpected error: "+err.Error(),
		)
		return
	}
}
