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

// valueKind abstracts a value's shape so one table drives every typed variant of the modifier.
type valueKind int

const (
	kindNull valueKind = iota
	kindUnknown
	kindKnown
)

func TestUseStateForUnknownIncludingNull(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		state       tfsdk.State
		stateValue  valueKind
		planValue   valueKind
		configValue valueKind
		want        valueKind
	}{
		{
			name:        "empty prior value is reused (diverges from the built-in)",
			state:       priorStateExists(),
			stateValue:  kindNull,
			planValue:   kindUnknown,
			configValue: kindNull,
			want:        kindNull,
		},
		{
			name:        "populated prior value is reused, as the built-in also does",
			state:       priorStateExists(),
			stateValue:  kindKnown,
			planValue:   kindUnknown,
			configValue: kindNull,
			want:        kindKnown,
		},
		{
			// Nothing to reuse while creating, and known-after-apply is the honest answer.
			name:        "creating leaves the value unknown",
			state:       noPriorState(),
			stateValue:  kindNull,
			planValue:   kindUnknown,
			configValue: kindNull,
			want:        kindUnknown,
		},
		{
			name:        "a known planned value is left alone",
			state:       priorStateExists(),
			stateValue:  kindNull,
			planValue:   kindKnown,
			configValue: kindKnown,
			want:        kindKnown,
		},
		{
			// The configuration supplies this value but has not resolved it, so it is not ours.
			name:        "an unresolved configuration value is left alone",
			state:       priorStateExists(),
			stateValue:  kindKnown,
			planValue:   kindUnknown,
			configValue: kindUnknown,
			want:        kindUnknown,
		},
	}

	stringOf := func(k valueKind) types.String {
		switch k {
		case kindUnknown:
			return types.StringUnknown()
		case kindKnown:
			return types.StringValue("lightning")
		}
		return types.StringNull()
	}
	someMap := types.MapValueMust(types.StringType, map[string]attr.Value{
		"topic_created": types.StringValue("on"),
	})
	mapOf := func(k valueKind) types.Map {
		switch k {
		case kindUnknown:
			return types.MapUnknown(types.StringType)
		case kindKnown:
			return someMap
		}
		return types.MapNull(types.StringType)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			strResp := &planmodifier.StringResponse{PlanValue: stringOf(tt.planValue)}
			UseStateForUnknownIncludingNullString().PlanModifyString(context.Background(), planmodifier.StringRequest{
				State:       tt.state,
				StateValue:  stringOf(tt.stateValue),
				PlanValue:   stringOf(tt.planValue),
				ConfigValue: stringOf(tt.configValue),
			}, strResp)
			require.False(t, strResp.Diagnostics.HasError())
			require.Equal(t, stringOf(tt.want), strResp.PlanValue, "string variant")

			mapResp := &planmodifier.MapResponse{PlanValue: mapOf(tt.planValue)}
			UseStateForUnknownIncludingNullMap().PlanModifyMap(context.Background(), planmodifier.MapRequest{
				State:       tt.state,
				StateValue:  mapOf(tt.stateValue),
				PlanValue:   mapOf(tt.planValue),
				ConfigValue: mapOf(tt.configValue),
			}, mapResp)
			require.False(t, mapResp.Diagnostics.HasError())
			require.Equal(t, mapOf(tt.want), mapResp.PlanValue, "map variant")
		})
	}
}
