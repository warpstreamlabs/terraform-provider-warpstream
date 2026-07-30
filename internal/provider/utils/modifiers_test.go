package utils

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stretchr/testify/require"
)

// The modifier only asks whether prior state exists at all, so these carry no attributes.
var emptyObject = tftypes.Object{AttributeTypes: map[string]tftypes.Type{}}

// priorStateExists is the state of a resource that has already been applied.
func priorStateExists() tfsdk.State {
	return tfsdk.State{Raw: tftypes.NewValue(emptyObject, map[string]tftypes.Value{})}
}

// noPriorState is the state of a resource that is being created.
func noPriorState() tfsdk.State {
	return tfsdk.State{Raw: tftypes.NewValue(emptyObject, nil)}
}

func TestUseStateForUnknownIncludingNullMap(t *testing.T) {
	t.Parallel()

	someMap := types.MapValueMust(types.StringType, map[string]attr.Value{
		"topic_created": types.StringValue("on"),
	})

	tests := []struct {
		name        string
		state       tfsdk.State
		stateValue  types.Map
		planValue   types.Map
		configValue types.Map
		want        types.Map
	}{
		{
			// The reason this modifier exists. The built-in bails here because the prior value is
			// null, so nothing undoes the framework marking the attribute known-after-apply and
			// the plan never settles.
			name:        "empty prior value is reused",
			state:       priorStateExists(),
			stateValue:  types.MapNull(types.StringType),
			planValue:   types.MapUnknown(types.StringType),
			configValue: types.MapNull(types.StringType),
			want:        types.MapNull(types.StringType),
		},
		{
			name:        "populated prior value is reused, as the built-in also does",
			state:       priorStateExists(),
			stateValue:  someMap,
			planValue:   types.MapUnknown(types.StringType),
			configValue: types.MapNull(types.StringType),
			want:        someMap,
		},
		{
			// Nothing to reuse while creating, and known-after-apply is the honest answer.
			name:        "creating leaves the value unknown",
			state:       noPriorState(),
			stateValue:  types.MapNull(types.StringType),
			planValue:   types.MapUnknown(types.StringType),
			configValue: types.MapNull(types.StringType),
			want:        types.MapUnknown(types.StringType),
		},
		{
			name:        "a known planned value is left alone",
			state:       priorStateExists(),
			stateValue:  types.MapNull(types.StringType),
			planValue:   someMap,
			configValue: someMap,
			want:        someMap,
		},
		{
			// The configuration supplies this value but has not resolved it, so it is not ours.
			name:        "an unresolved configuration value is left alone",
			state:       priorStateExists(),
			stateValue:  someMap,
			planValue:   types.MapUnknown(types.StringType),
			configValue: types.MapUnknown(types.StringType),
			want:        types.MapUnknown(types.StringType),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			resp := &planmodifier.MapResponse{PlanValue: tt.planValue}
			UseStateForUnknownIncludingNullMap().PlanModifyMap(context.Background(), planmodifier.MapRequest{
				State:       tt.state,
				StateValue:  tt.stateValue,
				PlanValue:   tt.planValue,
				ConfigValue: tt.configValue,
			}, resp)

			require.False(t, resp.Diagnostics.HasError())
			require.Equal(t, tt.want, resp.PlanValue)
		})
	}
}

func TestUseStateForUnknownIncludingNullString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		state       tfsdk.State
		stateValue  types.String
		planValue   types.String
		configValue types.String
		want        types.String
	}{
		{
			// configuration.default_topic_type is empty whenever nobody chose a topic type.
			name:        "empty prior value is reused",
			state:       priorStateExists(),
			stateValue:  types.StringNull(),
			planValue:   types.StringUnknown(),
			configValue: types.StringNull(),
			want:        types.StringNull(),
		},
		{
			name:        "populated prior value is reused, as the built-in also does",
			state:       priorStateExists(),
			stateValue:  types.StringValue("lightning"),
			planValue:   types.StringUnknown(),
			configValue: types.StringNull(),
			want:        types.StringValue("lightning"),
		},
		{
			name:        "creating leaves the value unknown",
			state:       noPriorState(),
			stateValue:  types.StringNull(),
			planValue:   types.StringUnknown(),
			configValue: types.StringNull(),
			want:        types.StringUnknown(),
		},
		{
			name:        "a known planned value is left alone",
			state:       priorStateExists(),
			stateValue:  types.StringNull(),
			planValue:   types.StringValue("classic"),
			configValue: types.StringValue("classic"),
			want:        types.StringValue("classic"),
		},
		{
			name:        "an unresolved configuration value is left alone",
			state:       priorStateExists(),
			stateValue:  types.StringValue("classic"),
			planValue:   types.StringUnknown(),
			configValue: types.StringUnknown(),
			want:        types.StringUnknown(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			resp := &planmodifier.StringResponse{PlanValue: tt.planValue}
			UseStateForUnknownIncludingNullString().PlanModifyString(context.Background(), planmodifier.StringRequest{
				State:       tt.state,
				StateValue:  tt.stateValue,
				PlanValue:   tt.planValue,
				ConfigValue: tt.configValue,
			}, resp)

			require.False(t, resp.Diagnostics.HasError())
			require.Equal(t, tt.want, resp.PlanValue)
		})
	}
}

// TestUseStateForUnknownIncludingNullDivergesFromBuiltIn pins the one difference from the upstream
// modifier, so that swapping this back for the built-in fails here rather than silently
// reintroducing a permanent phantom diff.
func TestUseStateForUnknownIncludingNullDivergesFromBuiltIn(t *testing.T) {
	t.Parallel()

	// Prior state exists but the value in it is empty. The built-in returns early on
	// StateValue.IsNull(); this modifier asks State.Raw.IsNull() instead, so it acts.
	resp := &planmodifier.StringResponse{PlanValue: types.StringUnknown()}
	UseStateForUnknownIncludingNullString().PlanModifyString(context.Background(), planmodifier.StringRequest{
		State:       priorStateExists(),
		StateValue:  types.StringNull(),
		PlanValue:   types.StringUnknown(),
		ConfigValue: types.StringNull(),
	}, resp)

	require.True(t, resp.PlanValue.IsNull(), "an empty prior value must be reused, not left unknown")
	require.False(t, resp.PlanValue.IsUnknown())
}
