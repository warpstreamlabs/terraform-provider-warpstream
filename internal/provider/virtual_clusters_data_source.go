package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/api"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ datasource.DataSource              = &virtualClustersDataSource{}
	_ datasource.DataSourceWithConfigure = &virtualClustersDataSource{}
)

// helper function to simplify the provider implementation.
func NewVirtualClustersDataSource() datasource.DataSource {
	return &virtualClustersDataSource{}
}

// virtualClustersDataSource is the data source implementation.
type virtualClustersDataSource struct {
	client *api.Client
}

// virtualClustersDataSourceModel maps the data source schema data.
type virtualClustersDataSourceModel struct {
	VirtualClusters []virtualClustersModel `tfsdk:"virtual_clusters"`
}

// virtualClustersModel maps virtual clusters schema data.
type virtualClustersModel struct {
	ID            types.String `tfsdk:"id"`
	Name          types.String `tfsdk:"name"`
	Type          types.String `tfsdk:"type"`
	AgentPoolID   types.String `tfsdk:"agent_pool_id"`
	AgentPoolName types.String `tfsdk:"agent_pool_name"`
	CreatedAt     types.String `tfsdk:"created_at"`
}

// Metadata returns the data source type name.
func (d *virtualClustersDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_virtual_clusters"
}

// Schema defines the schema for the data source.
func (d *virtualClustersDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"virtual_clusters": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed: true,
						},
						"name": schema.StringAttribute{
							Computed: true,
						},
						"type": schema.StringAttribute{
							Computed: true,
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
					},
				},
			},
		},
	}
}

// Read refreshes the Terraform state with the latest data.
func (d *virtualClustersDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state virtualClustersDataSourceModel

	virtual_clusters, err := d.client.GetVirtualClusters()
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Read WarpStream Virtual Clusters",
			err.Error(),
		)
		return
	}

	// Map response body to model
	for _, vcn := range virtual_clusters {
		vcnState := virtualClustersModel{
			ID:            types.StringValue(vcn.ID),
			Name:          types.StringValue(vcn.Name),
			Type:          types.StringValue(vcn.Type),
			AgentPoolID:   types.StringValue(vcn.AgentPoolID),
			AgentPoolName: types.StringValue(vcn.AgentPoolName),
			CreatedAt:     types.StringValue(vcn.CreatedAt),
		}

		state.VirtualClusters = append(state.VirtualClusters, vcnState)
	}

	// Set state
	diags := resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Configure adds the provider configured client to the data source.
func (d *virtualClustersDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
