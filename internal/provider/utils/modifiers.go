package utils

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// ignoreDiffPlanModifier is a custom plan modifier for string attributes that
// always sets the planned value to the state value, effectively suppressing
// differences.
type IgnoreDiffPlanModifier struct{}

func (m IgnoreDiffPlanModifier) PlanModifyString(ctx context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if !req.StateValue.IsUnknown() && !req.StateValue.IsNull() {
		resp.PlanValue = req.StateValue
	}
}

func (m IgnoreDiffPlanModifier) Description(ctx context.Context) string {
	return "Always use the state value to suppress differences for this attribute."
}

func (m IgnoreDiffPlanModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

// SoftDeleteTTLPlanModifier sets soft_topic_deletion_ttl_millis to 0 when
// enable_soft_topic_deletion is false, matching the API behavior.
type SoftDeleteTTLPlanModifier struct{}

func (m SoftDeleteTTLPlanModifier) PlanModifyInt64(ctx context.Context, req planmodifier.Int64Request, resp *planmodifier.Int64Response) {
	var enable types.Bool
	attrPath := path.Root("configuration").AtName("enable_soft_topic_deletion")

	if diags := req.Plan.GetAttribute(ctx, attrPath, &enable); diags.HasError() {
		if diags := req.Config.GetAttribute(ctx, attrPath, &enable); diags.HasError() {
			return
		}
	}

	if enable.IsNull() || enable.IsUnknown() {
		return
	}

	// If soft deletion is disabled, set TTL to 0
	if !enable.ValueBool() {
		resp.PlanValue = types.Int64Value(0)
	}
}

func (m SoftDeleteTTLPlanModifier) Description(ctx context.Context) string {
	return "Sets soft_topic_deletion_ttl_millis to 0 when enable_soft_topic_deletion is false."
}

func (m SoftDeleteTTLPlanModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}
