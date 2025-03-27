package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/datasourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/api"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/utils"
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

// Metadata returns the data source type name.
func (d *virtualClusterDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_virtual_cluster"
}

var agentKeyDataSourceSchema = schema.NestedAttributeObject{
	Attributes: map[string]schema.Attribute{
		"id": schema.StringAttribute{
			Computed: true,
		},
		"name": schema.StringAttribute{
			Computed: true,
		},
		"key": schema.StringAttribute{
			Computed:  true,
			Sensitive: true,
		},
		"virtual_cluster_id": schema.StringAttribute{
			Computed: true,
		},
		"created_at": schema.StringAttribute{
			Computed: true,
		},
	},
}

// Schema defines the schema for the data source.
func (d *virtualClusterDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: `
This data source allows you to look up a virtual cluster and its agent keys.

The WarpStream provider must be authenticated with an application key to read this data source.
`,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				Optional: true,
				Validators: []validator.String{
					// Cannot read from a schema registry
					utils.NotStartWith("vci_sr_"),
				},
			},
			"name": schema.StringAttribute{
				Computed: true,
				Optional: true,
				Validators: []validator.String{
					// Cannot read from a schema registry
					utils.NotStartWith("vcn_sr_"),
				},
			},
			"type": schema.StringAttribute{
				Computed: true,
			},
			"agent_keys": schema.ListNestedAttribute{
				Description:  "List of keys to authenticate an agent with this cluster.",
				Computed:     true,
				NestedObject: agentKeyDataSourceSchema,
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
			"tags": schema.MapAttribute{
				Description: "Tags associated with the virtual cluster.",
				Computed:    true,
				ElementType: types.StringType,
			},
			"configuration": schema.SingleNestedAttribute{
				Attributes: map[string]schema.Attribute{
					"auto_create_topic": schema.BoolAttribute{
						Computed: true,
					},
					"default_num_partitions": schema.Int64Attribute{
						Computed: true,
					},
					"default_retention_millis": schema.Int64Attribute{
						Computed: true,
					},
					"enable_acls": schema.BoolAttribute{
						Computed: true,
					},
					"enable_deletion_protection": schema.BoolAttribute{
						Computed: true,
					},
				},
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
					"region_group": schema.StringAttribute{
						Computed: true,
					},
				},
				Computed: true,
			},
			"bootstrap_url": schema.StringAttribute{
				Description: "Bootstrap URL to connect to the Virtual Cluster.",
				Computed:    true,
			},
			"workspace_id": virtualClusterWorkspaceIDSchema,
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
	var data virtualClusterDataSourceModel
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
	tflog.Debug(ctx, fmt.Sprintf("Virtual Cluster: %+v", *vc))

	agentKeys, ok := mapToAgentKeyModels(vc.AgentKeys, &diags)
	if !ok {
		return // Diagnostics handled inside helper.
	}

	// Map response body to model
	state := virtualClusterDataSourceModel{
		ID:            types.StringValue(vc.ID),
		Name:          types.StringValue(vc.Name),
		Type:          types.StringValue(vc.Type),
		AgentKeys:     agentKeys,
		AgentPoolID:   types.StringValue(vc.AgentPoolID),
		AgentPoolName: types.StringValue(vc.AgentPoolName),
		CreatedAt:     types.StringValue(vc.CreatedAt),
		Configuration: data.Configuration,
		Cloud:         data.Cloud,
		Tags:          data.Tags,
		WorkspaceID:   types.StringValue(vc.WorkspaceID),
	}

	if vc.BootstrapURL != nil {
		state.BootstrapURL = types.StringValue(*vc.BootstrapURL)
	}

	// Set state
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	cldState := virtualClusterCloudModel{
		Provider:    types.StringValue(vc.CloudProvider),
		Region:      types.StringNull(),
		RegionGroup: types.StringNull(),
	}
	if vc.ClusterRegion.IsMultiRegion {
		cldState.RegionGroup = types.StringValue(vc.ClusterRegion.RegionGroup.Name)
	} else {
		cldState.Region = types.StringValue(vc.ClusterRegion.Region.Name)
	}

	diags = resp.State.SetAttribute(ctx, path.Root("cloud"), cldState)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tags, err := d.client.GetTags(*vc)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Read tags of Virtual Cluster with ID="+vc.ID,
			err.Error(),
		)
		return
	}

	tagsMap := make(map[string]attr.Value)
	for k, v := range tags {
		tagsMap[k] = types.StringValue(v)
	}

	tagsValue, diags := types.MapValue(types.StringType, tagsMap)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	diags = resp.State.SetAttribute(ctx, path.Root("tags"), tagsValue)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Read virtual cluster configuration
	cfg, err := d.client.GetConfiguration(*vc)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Read configuration of Virtual Cluster with ID="+vc.ID,
			err.Error(),
		)
		return
	}

	cfgState := virtualClusterConfigurationModel{
		AclsEnabled:              types.BoolValue(cfg.AclsEnabled),
		AutoCreateTopic:          types.BoolValue(cfg.AutoCreateTopic),
		DefaultNumPartitions:     types.Int64Value(cfg.DefaultNumPartitions),
		DefaultRetention:         types.Int64Value(cfg.DefaultRetentionMillis),
		EnableDeletionProtection: types.BoolValue(cfg.EnableDeletionProtection),
	}

	// Set configuration state
	diags = resp.State.SetAttribute(ctx, path.Root("configuration"), cfgState)
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
