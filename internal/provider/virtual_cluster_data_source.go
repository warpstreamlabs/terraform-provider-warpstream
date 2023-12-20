package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/datasourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/api"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ datasource.DataSource              = &virtualClusterDataSource{}
	_ datasource.DataSourceWithConfigure = &virtualClusterDataSource{}
)

// helper function to simplify the provider implementation.
func NewVirtualClusterDataSource() datasource.DataSource {
	return &virtualClusterDataSource{}
}

// virtualClusterDataSource is the data source implementation.
type virtualClusterDataSource struct {
	client *api.Client
}

// virtualClusterModel maps virtual cluster schema data.
type virtualClusterModel struct {
	ID            types.String `tfsdk:"id"`
	Name          types.String `tfsdk:"name"`
	AgentPoolID   types.String `tfsdk:"agent_pool_id"`
	AgentPoolName types.String `tfsdk:"agent_pool_name"`
	CreatedAt     types.String `tfsdk:"created_at"`
	Default       types.Bool   `tfsdk:"default"`
}

// Metadata returns the data source type name.
func (d *virtualClusterDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_virtual_cluster"
}

// Schema defines the schema for the data source.
func (d *virtualClusterDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				Optional: true,
			},
			"name": schema.StringAttribute{
				Computed: true,
				Optional: true,
			},
			"agent_pool_id": schema.StringAttribute{
				Computed: true,
			},
			"agent_pool_name": schema.StringAttribute{
				Computed: true,
			},
			"created_at": schema.StringAttribute{
				Computed: true,
			},
			"default": schema.BoolAttribute{
				Optional: true,
			},
		},
	}
}

func (d *virtualClusterDataSource) ConfigValidators(ctx context.Context) []datasource.ConfigValidator {
	return []datasource.ConfigValidator{
		datasourcevalidator.ExactlyOneOf(
			path.MatchRoot("id"),
			path.MatchRoot("name"),
			path.MatchRoot("default"),
		),
	}
}

// Read refreshes the Terraform state with the latest data.
func (d *virtualClusterDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data virtualClusterModel
	diags := req.Config.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var vc *api.VirtualCluster
	var err error

	if data.Default.ValueBool() {
		vc, err = d.client.GetDefaultCluster()
	} else if data.Name.ValueString() != "" {
		vc, err = d.client.FindVirtualCluster(data.Name.ValueString())
	} else {
		vc, err = d.client.GetVirtualCluster(data.ID.ValueString())
	}

	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Read WarpStream Virtual Cluster",
			err.Error(),
		)
		return
	}

	// Map response body to model
	state := virtualClusterModel{
		ID:            types.StringValue(vc.ID),
		Name:          types.StringValue(vc.Name),
		AgentPoolID:   types.StringValue(vc.AgentPoolID),
		AgentPoolName: types.StringValue(vc.AgentPoolName),
		CreatedAt:     types.StringValue(vc.CreatedAt),
	}

	// Set state
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Configure adds the provider configured client to the data source.
func (d *virtualClusterDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*api.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *api.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	d.client = client
}
