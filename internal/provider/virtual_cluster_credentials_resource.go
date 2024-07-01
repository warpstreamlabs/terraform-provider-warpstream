package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/api"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/utils"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource              = &virtualClusterCredentialsResource{}
	_ resource.ResourceWithConfigure = &virtualClusterCredentialsResource{}
)

// NewVirtualClusterCredentialsResource is a helper function to simplify the provider implementation.
func NewVirtualClusterCredentialsResource() resource.Resource {
	return &virtualClusterCredentialsResource{}
}

// virtualClusterCredentialsResource is the resource implementation.
type virtualClusterCredentialsResource struct {
	client *api.Client
}

// virtualClusterCredentialsModel maps credentials schema data.
type virtualClusterCredentialsModel struct {
	ID                  types.String `tfsdk:"id"`
	Name                types.String `tfsdk:"name"`
	UserName            types.String `tfsdk:"username"`
	Password            types.String `tfsdk:"password"`
	CreatedAt           types.String `tfsdk:"created_at"`
	AgentPoolID         types.String `tfsdk:"agent_pool"`
	VirtualClusterID    types.String `tfsdk:"virtual_cluster_id"`
	VirtualClusterIDOld types.String `tfsdk:"virtual_cluster"`
	ClusterSuperuser    types.Bool   `tfsdk:"cluster_superuser"`
}

// Configure adds the provider configured client to the data source.
func (r *virtualClusterCredentialsResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (r *virtualClusterCredentialsResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_virtual_cluster_credentials"
}

// Schema defines the schema for the resource.
func (r *virtualClusterCredentialsResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: `
This resource allows you to create and delete virtual cluster credentials.
`,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Virtual Cluster Credentials ID.",
				Computed:    true,
			},
			"name": schema.StringAttribute{
				Description: "Virtual Cluster Credentials Name.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{utils.StartsWith("ccn_")},
			},
			"agent_pool": schema.StringAttribute{
				Description: "Agent Pool ID.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"virtual_cluster": schema.StringAttribute{
				Description:        "Virtual Cluster ID. Deprecated in favor of `virtual_cluster_id`.",
				DeprecationMessage: "Use `virtual_cluster_id` instead.",
				Optional:           true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"virtual_cluster_id": schema.StringAttribute{
				Description: "Virtual Cluster ID. Required unless `virtual_cluster` is set.",
				Optional:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.ExactlyOneOf(path.Expressions{
						// The ExactlyOneOf docstring suggest that "virtual_cluster_id" is implicitly included in
						// this array of expressions. In practice we see that the resulting error message reports
						// just the expressions that are passed to this array explicitly. So we include both.
						path.MatchRoot("virtual_cluster"), path.MatchRoot("virtual_cluster_id"),
					}...),
				},
			},
			"created_at": schema.StringAttribute{
				Description: "Virtual Cluster Credentials Creation Timestamp.",
				Computed:    true,
			},
			"username": schema.StringAttribute{
				Description: "Username.",
				Computed:    true,
			},
			"password": schema.StringAttribute{
				Description: "Password.",
				Computed:    true,
				Sensitive:   true,
			},
			"cluster_superuser": schema.BoolAttribute{
				Description: "Whether the user is cluster superuser. If `true`, the credentials will be created with superuser privileges which enables ACL management via the Kafka Admin APIs. If `false`, and cluster ACLs are enabled, and no `ALLOW` ACLs are set, then these credentials will not be able to access the cluster.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

// Create a new resource.
func (r *virtualClusterCredentialsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Retrieve values from plan
	var plan virtualClusterCredentialsModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	vci, found := getVirtualClusterIDWithDeprecation(
		plan.VirtualClusterID, plan.VirtualClusterIDOld, &resp.Diagnostics,
	)
	if !found {
		return // Diagnostics handled by helper.
	}

	// Obtain virtual cluster info
	cluster, err := r.client.GetVirtualCluster(vci)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading WarpStream Virtual Cluster",
			"Could not read WarpStream Virtual Cluster ID "+vci+": "+err.Error(),
		)
		return
	}

	// Create new virtual cluster credentials
	c, err := r.client.CreateCredentials(plan.Name.ValueString(), plan.ClusterSuperuser.ValueBool(), *cluster)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating WarpStream Virtual Cluster Credentials",
			"Could not create WarpStream Virtual Cluster Credentials, unexpected error: "+err.Error(),
		)
		return
	}

	// Map response body to schema and populate Computed attribute values
	newPlan := virtualClusterCredentialsModel{
		ID:               types.StringValue(c.ID),
		Name:             types.StringValue(c.Name),
		AgentPoolID:      types.StringValue(c.AgentPoolID),
		CreatedAt:        types.StringValue(c.CreatedAt), // null
		UserName:         types.StringValue(c.UserName),
		Password:         types.StringValue(c.Password),
		ClusterSuperuser: types.BoolValue(c.ClusterSuperuser),
	}

	setVirtualClusterIDWithDeprecation(plan, &newPlan)

	// Set state to fully populated data
	diags = resp.State.Set(ctx, newPlan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Read refreshes the Terraform state with the latest data.
func (r *virtualClusterCredentialsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state virtualClusterCredentialsModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Obtain virtual cluster info
	vci, found := getVirtualClusterIDWithDeprecation(
		state.VirtualClusterID, state.VirtualClusterIDOld, &resp.Diagnostics,
	)

	if !found {
		return // Diagnostics handled by helper.
	}

	cluster, err := r.client.GetVirtualCluster(vci)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading WarpStream Virtual Cluster",
			"Could not read WarpStream Virtual Cluster ID "+vci+": "+err.Error(),
		)
		return
	}

	creds, err := r.client.GetCredentials(*cluster)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading WarpStream Virtual Cluster Credentials",
			"Could not read WarpStream Virtual Cluster Credentials"+": "+err.Error(),
		)
		return
	}

	c, ok := creds[state.ID.ValueString()]
	if !ok {
		resp.Diagnostics.AddError(
			"Error Reading WarpStream Virtual Cluster Credentials",
			"Could not find WarpStream Virtual Cluster Credentials with ID "+state.ID.ValueString(),
		)
		return
	}

	// Overwrite Virtual Cluster Credentials with refreshed state
	newState := virtualClusterCredentialsModel{
		ID:               types.StringValue(c.ID),
		Name:             types.StringValue(c.Name),
		UserName:         types.StringValue(c.UserName),
		Password:         state.Password,
		AgentPoolID:      types.StringValue(c.AgentPoolID),
		CreatedAt:        types.StringValue(c.CreatedAt),
		ClusterSuperuser: types.BoolValue(c.ClusterSuperuser),
	}

	setVirtualClusterIDWithDeprecation(state, &newState)

	// Set new state
	diags = resp.State.Set(ctx, &newState)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Update updates the resource and sets the updated Terraform state on success.
func (r *virtualClusterCredentialsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *virtualClusterCredentialsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Retrieve values from state
	var state virtualClusterCredentialsModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Obtain virtual cluster info
	vci, found := getVirtualClusterIDWithDeprecation(
		state.VirtualClusterID, state.VirtualClusterIDOld, &resp.Diagnostics,
	)
	if !found {
		return // Diagnostics handled by helper.
	}

	cluster, err := r.client.GetVirtualCluster(vci)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading WarpStream Virtual Cluster",
			"Could not read WarpStream Virtual Cluster ID "+vci+": "+err.Error(),
		)
		return
	}

	// Delete existing credentials
	err = r.client.DeleteCredentials(state.ID.ValueString(), *cluster)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting WarpStream Virtual Cluster Credentials",
			"Could not delete WarpStream Virtual Cluster Credentials, unexpected error: "+err.Error(),
		)
		return
	}
}

// getVirtualClusterIDWithDeprecation is a helper to read virtual cluster ID from the new or old field,
// whichever was set in the plan or state.
func getVirtualClusterIDWithDeprecation(new, old types.String, diags *diag.Diagnostics) (string, bool) {
	if new.IsNull() && old.IsNull() {
		diags.AddError(
			"Error Reading WarpStream Virtual Cluster",
			"Either `virtual_cluster` or `virtual_cluster_id` must be set",
		)
		return "", false
	}

	var vci string
	if !new.IsNull() {
		vci = new.ValueString()
	} else {
		vci = old.ValueString()
	}
	return vci, true
}

// getVirtualClusterIDWithDeprecation is a helper to set virtual cluster ID on the new or old field,
// whichever was set in the plan or state.
func setVirtualClusterIDWithDeprecation(
	state virtualClusterCredentialsModel,
	newState *virtualClusterCredentialsModel,
) {
	if !state.VirtualClusterID.IsNull() {
		newState.VirtualClusterID = state.VirtualClusterID
	}
	if !state.VirtualClusterIDOld.IsNull() {
		newState.VirtualClusterIDOld = state.VirtualClusterIDOld
	}
}
