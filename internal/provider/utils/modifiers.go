package utils

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// IgnoreDiffPlanModifier is a custom plan modifier for string attributes that
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

// yamlMapSemanticEqualsPlanModifier suppresses diffs for a Map(String)
// attribute when every element is YAML-semantically equal between the plan and
// the prior state. This avoids perpetual diffs caused by whitespace or
// formatting differences in YAML values.
type yamlMapSemanticEqualsPlanModifier struct{}

func YamlMapSemanticEquals() planmodifier.Map {
	return yamlMapSemanticEqualsPlanModifier{}
}

func (m yamlMapSemanticEqualsPlanModifier) Description(_ context.Context) string {
	return "Compares map string values as YAML for semantic equality, suppressing whitespace-only diffs."
}

func (m yamlMapSemanticEqualsPlanModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m yamlMapSemanticEqualsPlanModifier) PlanModifyMap(_ context.Context, req planmodifier.MapRequest, resp *planmodifier.MapResponse) {
	if req.StateValue.IsNull() || req.StateValue.IsUnknown() ||
		req.PlanValue.IsNull() || req.PlanValue.IsUnknown() {
		return
	}

	if yamlMapsSemanticEqual(req.PlanValue, req.StateValue) {
		resp.PlanValue = req.StateValue
	}
}

// yamlMapsSemanticEqual returns true when two Map(String) values have the same
// keys and every corresponding value pair is YAML-semantically equal.
func yamlMapsSemanticEqual(a, b types.Map) bool {
	aElems := a.Elements()
	bElems := b.Elements()

	if len(aElems) != len(bElems) {
		return false
	}

	for key, aVal := range aElems {
		bVal, ok := bElems[key]
		if !ok {
			return false
		}

		aStr, ok1 := aVal.(types.String)
		bStr, ok2 := bVal.(types.String)
		if !ok1 || !ok2 || aStr.IsUnknown() || bStr.IsUnknown() {
			return false
		}

		normalizedA, err1 := NormalizeYAML(aStr.ValueString())
		normalizedB, err2 := NormalizeYAML(bStr.ValueString())
		if err1 != nil || err2 != nil || normalizedA != normalizedB {
			return false
		}
	}

	return true
}

// configurationIDPlanModifier preserves the prior state value for
// configuration_id when the pipeline YAML configuration has not semantically
// changed. Unlike UseStateForUnknown it allows the value to be recomputed
// (unknown) when configuration_inputs or configuration_yaml actually differ.
type configurationIDPlanModifier struct{}

func ConfigurationIDUseStateForUnchanged() planmodifier.String {
	return configurationIDPlanModifier{}
}

func (m configurationIDPlanModifier) Description(_ context.Context) string {
	return "Preserves configuration_id when the pipeline configuration is semantically unchanged."
}

func (m configurationIDPlanModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m configurationIDPlanModifier) PlanModifyString(ctx context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if !resp.PlanValue.IsUnknown() || req.StateValue.IsNull() {
		return
	}

	// Read configuration_inputs from Config (raw user values, unaffected by
	// other plan modifiers) and State.
	var configInputs, stateInputs types.Map
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("configuration_inputs"), &configInputs)...)
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("configuration_inputs"), &stateInputs)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !configInputs.IsNull() || !stateInputs.IsNull() {
		if !configInputs.IsNull() && !configInputs.IsUnknown() && !stateInputs.IsNull() {
			if yamlMapsSemanticEqual(configInputs, stateInputs) {
				resp.PlanValue = req.StateValue
			}
		}
		return
	}

	// Both configuration_inputs are null — compare configuration_yaml.
	var configYAML, stateYAML YamlValue
	diags1 := req.Config.GetAttribute(ctx, path.Root("configuration_yaml"), &configYAML)
	diags2 := req.State.GetAttribute(ctx, path.Root("configuration_yaml"), &stateYAML)
	if diags1.HasError() || diags2.HasError() {
		return
	}

	if configYAML.IsNull() || stateYAML.IsNull() {
		if configYAML.IsNull() && stateYAML.IsNull() {
			resp.PlanValue = req.StateValue
		}
		return
	}

	normalizedConfig, err1 := NormalizeYAML(configYAML.ValueString())
	normalizedState, err2 := NormalizeYAML(stateYAML.ValueString())
	if err1 == nil && err2 == nil && normalizedConfig == normalizedState {
		resp.PlanValue = req.StateValue
	}
}
