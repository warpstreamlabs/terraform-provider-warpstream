package datasources

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/api"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/utils"
)

var (
	_ datasource.DataSource              = &clientMetricsSubscriptionsDataSource{}
	_ datasource.DataSourceWithConfigure = &clientMetricsSubscriptionsDataSource{}
)

func NewClientMetricsSubscriptionsDataSource() datasource.DataSource {
	return &clientMetricsSubscriptionsDataSource{}
}

type clientMetricsSubscriptionsDataSource struct {
	client *api.Client
}

// clientMetricsSubscriptionsDataSourceModel maps the data source schema data.
// virtual_cluster_id is required input (subscriptions are cluster-scoped),
// subscriptions is computed.
type clientMetricsSubscriptionsDataSourceModel struct {
	VirtualClusterID types.String                          `tfsdk:"virtual_cluster_id"`
	Subscriptions    []clientMetricsSubscriptionsElemModel `tfsdk:"subscriptions"`
}

// clientMetricsSubscriptionsElemModel is the per-subscription row returned
// by the list data source. Mirrors the resource attributes minus
// virtual_cluster_id (which would be redundant on every row).
type clientMetricsSubscriptionsElemModel struct {
	ID         types.String `tfsdk:"id"`
	Name       types.String `tfsdk:"name"`
	IntervalMs types.Int64  `tfsdk:"interval_ms"`
	Metrics    types.String `tfsdk:"metrics"`
	Match      types.String `tfsdk:"match"`
}

func (d *clientMetricsSubscriptionsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_client_metrics_subscriptions"
}

func (d *clientMetricsSubscriptionsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: `
This data source lists all client metrics subscriptions on a WarpStream Virtual Cluster.

The WarpStream provider must be authenticated with an application key to read this data source.
`,
		Attributes: map[string]schema.Attribute{
			"virtual_cluster_id": schema.StringAttribute{
				Description: "ID of the Virtual Cluster whose subscriptions to list.",
				Required:    true,
				Validators:  []validator.String{utils.ValidClusterID()},
			},
			"subscriptions": schema.ListNestedAttribute{
				Description: "All client metrics subscriptions in the Virtual Cluster, sorted by `name`.",
				Computed:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Description: "Composite identifier in the form `<virtual_cluster_id>/<name>`.",
							Computed:    true,
						},
						"name": schema.StringAttribute{
							Description: "Name of the subscription.",
							Computed:    true,
						},
						"interval_ms": schema.Int64Attribute{
							Description: "Push interval in milliseconds, or null if not set on the subscription.",
							Computed:    true,
						},
						"metrics": schema.StringAttribute{
							Description: "Comma-separated list of metric name prefixes, or null if not set on the subscription.",
							Computed:    true,
						},
						"match": schema.StringAttribute{
							Description: "Comma-separated list of `<key>=<regex>` client match selectors, or null if not set on the subscription.",
							Computed:    true,
						},
					},
				},
			},
		},
	}
}

func (d *clientMetricsSubscriptionsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config clientMetricsSubscriptionsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	vcID := config.VirtualClusterID.ValueString()

	subs, err := d.client.ListClientMetricsSubscriptions(vcID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to List WarpStream Client Metrics Subscriptions",
			fmt.Sprintf("Could not list subscriptions in virtual cluster %q: %s", vcID, err.Error()),
		)
		return
	}

	state := clientMetricsSubscriptionsDataSourceModel{
		VirtualClusterID: types.StringValue(vcID),
		Subscriptions:    make([]clientMetricsSubscriptionsElemModel, 0, len(subs)),
	}
	for _, sub := range subs {
		elem := clientMetricsSubscriptionsElemModel{
			ID:         types.StringValue(fmt.Sprintf("%s/%s", vcID, sub.Name)),
			Name:       types.StringValue(sub.Name),
			IntervalMs: types.Int64Null(),
			Metrics:    types.StringNull(),
			Match:      types.StringNull(),
		}
		if sub.IntervalMs != nil {
			elem.IntervalMs = types.Int64Value(int64(*sub.IntervalMs))
		}
		if sub.Metrics != nil {
			elem.Metrics = types.StringValue(*sub.Metrics)
		}
		if sub.Match != nil {
			elem.Match = types.StringValue(*sub.Match)
		}
		state.Subscriptions = append(state.Subscriptions, elem)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (d *clientMetricsSubscriptionsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
