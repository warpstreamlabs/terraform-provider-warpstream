package resources

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/resourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
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

var (
	_ resource.Resource                     = &clientMetricsSubscriptionResource{}
	_ resource.ResourceWithConfigure        = &clientMetricsSubscriptionResource{}
	_ resource.ResourceWithImportState      = &clientMetricsSubscriptionResource{}
	_ resource.ResourceWithConfigValidators = &clientMetricsSubscriptionResource{}
)

func NewClientMetricsSubscriptionResource() resource.Resource {
	return &clientMetricsSubscriptionResource{}
}

type clientMetricsSubscriptionResource struct {
	client *api.Client
}

func (r *clientMetricsSubscriptionResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *clientMetricsSubscriptionResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_client_metrics_subscription"
}

func (r *clientMetricsSubscriptionResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: `
This resource manages a client metrics subscription on a WarpStream Virtual Cluster.

A subscription tells WarpStream which Kafka clients should push metrics, which metric prefixes to collect, and how often to push them. Subscriptions are cluster-scoped and identified by ` + "`name`" + `; at least one of ` + "`interval_ms`, `metrics`, or `match`" + ` must be set. The WarpStream API performs a whole-subscription replace on update, so removing a field from the Terraform configuration clears it on the server.

The WarpStream provider must be authenticated with an application key to consume this resource.
`,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Composite identifier in the form `<virtual_cluster_id>/<name>`.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"virtual_cluster_id": schema.StringAttribute{
				Description: "ID of the Virtual Cluster the subscription belongs to.",
				Required:    true,
				Validators:  []validator.String{utils.ValidClusterID()},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Description: "Name of the subscription. Unique within a Virtual Cluster.",
				Required:    true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"interval_ms": schema.Int64Attribute{
				Description: fmt.Sprintf(
					"Push interval in milliseconds. Must be between %d and %d (inclusive).",
					utils.ClientMetricsPushIntervalMsMin,
					utils.ClientMetricsPushIntervalMsMax,
				),
				Optional: true,
				Validators: []validator.Int64{
					int64validator.Between(utils.ClientMetricsPushIntervalMsMin, utils.ClientMetricsPushIntervalMsMax),
				},
			},
			"metrics": schema.StringAttribute{
				Description: "Comma-separated list of metric name prefixes that subscribed clients should push (for example `org.apache.kafka.producer.`), or `*` to subscribe to all metrics.",
				Optional:    true,
			},
			"match": schema.StringAttribute{
				Description: fmt.Sprintf(
					"Comma-separated list of `<key>=<regex>` pairs identifying which clients are subscribed (for example `client_id=^app-.*`). Valid keys are %s.",
					utils.ClientMetricsMatchAllowedParamsDescription,
				),
				Optional: true,
				Validators: []validator.String{
					utils.ValidClientMetricsMatchPattern(),
				},
			},
		},
	}
}

func (r *clientMetricsSubscriptionResource) ConfigValidators(_ context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		resourcevalidator.AtLeastOneOf(
			path.MatchRoot("interval_ms"),
			path.MatchRoot("metrics"),
			path.MatchRoot("match"),
		),
	}
}

func (r *clientMetricsSubscriptionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan models.ClientMetricsSubscription
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	vcID := plan.VirtualClusterID.ValueString()
	name := plan.Name.ValueString()

	if err := r.client.UpdateClientMetricsSubscriptions(vcID, []api.ClientMetricsSubscription{planToSubscription(plan)}); err != nil {
		resp.Diagnostics.AddError(
			"Error Creating WarpStream Client Metrics Subscription",
			fmt.Sprintf("Could not create subscription %q in virtual cluster %q: %s", name, vcID, err.Error()),
		)
		return
	}

	sub, err := r.client.DescribeClientMetricsSubscription(vcID, name)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading WarpStream Client Metrics Subscription After Create",
			fmt.Sprintf("Could not read subscription %q in virtual cluster %q after creation: %s", name, vcID, err.Error()),
		)
		return
	}

	state := subscriptionToModel(vcID, sub)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *clientMetricsSubscriptionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state models.ClientMetricsSubscription
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	vcID := state.VirtualClusterID.ValueString()
	name := state.Name.ValueString()

	sub, err := r.client.DescribeClientMetricsSubscription(vcID, name)
	if err != nil {
		if errors.Is(err, api.ErrNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error Reading WarpStream Client Metrics Subscription",
			fmt.Sprintf("Could not read subscription %q in virtual cluster %q: %s", name, vcID, err.Error()),
		)
		return
	}

	newState := subscriptionToModel(vcID, sub)
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *clientMetricsSubscriptionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan models.ClientMetricsSubscription
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	vcID := plan.VirtualClusterID.ValueString()
	name := plan.Name.ValueString()

	if err := r.client.UpdateClientMetricsSubscriptions(vcID, []api.ClientMetricsSubscription{planToSubscription(plan)}); err != nil {
		resp.Diagnostics.AddError(
			"Error Updating WarpStream Client Metrics Subscription",
			fmt.Sprintf("Could not update subscription %q in virtual cluster %q: %s", name, vcID, err.Error()),
		)
		return
	}

	sub, err := r.client.DescribeClientMetricsSubscription(vcID, name)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading WarpStream Client Metrics Subscription After Update",
			fmt.Sprintf("Could not read subscription %q in virtual cluster %q after update: %s", name, vcID, err.Error()),
		)
		return
	}

	state := subscriptionToModel(vcID, sub)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *clientMetricsSubscriptionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state models.ClientMetricsSubscription
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	vcID := state.VirtualClusterID.ValueString()
	name := state.Name.ValueString()

	if err := r.client.DeleteClientMetricsSubscriptions(vcID, []string{name}); err != nil {
		if errors.Is(err, api.ErrNotFound) {
			return
		}
		resp.Diagnostics.AddError(
			"Error Deleting WarpStream Client Metrics Subscription",
			fmt.Sprintf("Could not delete subscription %q in virtual cluster %q: %s", name, vcID, err.Error()),
		)
		return
	}
}

// ImportState parses an ID of the form `<virtual_cluster_id>/<name>`. We
// split on the first `/` only so that subscription names containing slashes
// can still be imported (the WarpStream backend does not document a slash
// restriction on names).
func (r *clientMetricsSubscriptionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			"Expected an ID in the format `<virtual_cluster_id>/<name>`, e.g. `vci_XXXXXXXXXX/my-subscription`.",
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("virtual_cluster_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), parts[1])...)
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// planToSubscription converts the tfsdk model into the wire subscription
// struct, mapping null/unset Terraform values to nil pointers (which the API
// treats as "leave this field unset on the stored subscription").
func planToSubscription(plan models.ClientMetricsSubscription) api.ClientMetricsSubscription {
	sub := api.ClientMetricsSubscription{
		Name: plan.Name.ValueString(),
	}
	if !plan.IntervalMs.IsNull() && !plan.IntervalMs.IsUnknown() {
		v := int32(plan.IntervalMs.ValueInt64())
		sub.IntervalMs = &v
	}
	if !plan.Metrics.IsNull() && !plan.Metrics.IsUnknown() {
		v := plan.Metrics.ValueString()
		sub.Metrics = &v
	}
	if !plan.Match.IsNull() && !plan.Match.IsUnknown() {
		v := plan.Match.ValueString()
		sub.Match = &v
	}
	return sub
}

// subscriptionToModel maps an API response subscription back into the tfsdk
// model. nil pointer fields on the wire become null Terraform values.
func subscriptionToModel(vcID string, sub *api.ClientMetricsSubscription) models.ClientMetricsSubscription {
	state := models.ClientMetricsSubscription{
		ID:               types.StringValue(fmt.Sprintf("%s/%s", vcID, sub.Name)),
		VirtualClusterID: types.StringValue(vcID),
		Name:             types.StringValue(sub.Name),
		IntervalMs:       types.Int64Null(),
		Metrics:          types.StringNull(),
		Match:            types.StringNull(),
	}
	if sub.IntervalMs != nil {
		state.IntervalMs = types.Int64Value(int64(*sub.IntervalMs))
	}
	if sub.Metrics != nil {
		state.Metrics = types.StringValue(*sub.Metrics)
	}
	if sub.Match != nil {
		state.Match = types.StringValue(*sub.Match)
	}
	return state
}
