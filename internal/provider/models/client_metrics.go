package models

import "github.com/hashicorp/terraform-plugin-framework/types"

// ClientMetricsSubscription is the shared tfsdk model for both the
// warpstream_client_metrics_subscription resource and data source. The
// attributes are identical between the two: the data source treats the
// inputs as required and everything else as computed, while the resource
// uses the inputs as the plan and reads back the rest from the API.
type ClientMetricsSubscription struct {
	ID               types.String `tfsdk:"id"`
	VirtualClusterID types.String `tfsdk:"virtual_cluster_id"`
	Name             types.String `tfsdk:"name"`
	IntervalMs       types.Int64  `tfsdk:"interval_ms"`
	Metrics          types.String `tfsdk:"metrics"`
	Match            types.String `tfsdk:"match"`
}
