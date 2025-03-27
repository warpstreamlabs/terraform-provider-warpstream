package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/datasourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/api"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/utils"
)

var (
	_ datasource.DataSource              = &schemaRegistryDataSource{}
	_ datasource.DataSourceWithConfigure = &schemaRegistryDataSource{}
)

func NewSchemaRegistryDataSource() datasource.DataSource {
	return &schemaRegistryDataSource{}
}

type schemaRegistryDataSource struct {
	client *api.Client
}

// Metadata returns the data source type name.
func (d *schemaRegistryDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_schema_registry"
}

// Schema defines the schema for the data source.
func (d *schemaRegistryDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:   true,
				Optional:   true,
				Validators: []validator.String{utils.StartsWithAndAlphanumeric("vci_sr_")},
			},
			"name": schema.StringAttribute{
				Computed:   true,
				Optional:   true,
				Validators: []validator.String{utils.StartsWithAndAlphanumeric("vcn_sr_")},
			},
			"agent_keys": schema.ListNestedAttribute{
				Description:  "List of keys to authenticate an agent with this cluster.",
				Computed:     true,
				NestedObject: agentKeyDataSourceSchema,
			},
			"created_at": schema.StringAttribute{
				Computed: true,
			},
			"cloud": schema.SingleNestedAttribute{
				Attributes: map[string]schema.Attribute{
					"region": schema.StringAttribute{
						Computed: true,
					},
					"provider": schema.StringAttribute{
						Computed: true,
					},
				},
				Computed: true,
			},
			"bootstrap_url": schema.StringAttribute{
				Description: "Bootstrap URL to connect to the Schema Registry.",
				Computed:    true,
			},
			"workspace_id": virtualClusterWorkspaceIDSchema,
		},
	}
}

func (d *schemaRegistryDataSource) ConfigValidators(ctx context.Context) []datasource.ConfigValidator {
	return []datasource.ConfigValidator{
		datasourcevalidator.ExactlyOneOf(
			path.MatchRoot("id"),
			path.MatchRoot("name"),
		),
	}
}

// Read refreshes the Terraform state with the latest data.
func (d *schemaRegistryDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data schemaRegistryDataSourceModel
	diags := req.Config.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var vc *api.VirtualCluster
	var err error

	if data.Name.ValueString() != "" {
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
	tflog.Debug(ctx, fmt.Sprintf("Schema Registry: %+v", *vc))

	agentKeys, ok := mapToAgentKeyModels(vc.AgentKeys, &diags)
	if !ok {
		return // Diagnostics handled inside helper.
	}

	state := schemaRegistryDataSourceModel{
		ID:          types.StringValue(vc.ID),
		Name:        types.StringValue(vc.Name),
		AgentKeys:   agentKeys,
		CreatedAt:   types.StringValue(vc.CreatedAt),
		Cloud:       data.Cloud,
		WorkspaceID: types.StringValue(vc.WorkspaceID),
	}

	if vc.BootstrapURL != nil {
		state.BootstrapURL = types.StringValue(*vc.BootstrapURL)
	}

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	cldState := virtualClusterRegistryCloudModel{
		Provider: types.StringValue(vc.CloudProvider),
		// schema registry is always single region
		Region: types.StringValue(vc.ClusterRegion.Region.Name),
	}

	diags = resp.State.SetAttribute(ctx, path.Root("cloud"), cldState)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (d *schemaRegistryDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
