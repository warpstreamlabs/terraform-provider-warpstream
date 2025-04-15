package utils

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
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
