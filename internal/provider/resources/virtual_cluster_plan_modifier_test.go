package resources

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stretchr/testify/require"
)

func TestUseStateForUnknownIncludingNull(t *testing.T) {
	t.Parallel()

	nullResourceState := tfsdk.State{
		Raw: tftypes.NewValue(tftypes.String, nil),
	}
	existingResourceState := tfsdk.State{
		Raw: tftypes.NewValue(tftypes.String, "existing"),
	}

	tests := []struct {
		name          string
		resourceState tfsdk.State
		configValue   types.String
		planValue     types.String
		stateValue    types.String
		want          types.String
	}{
		{
			name:          "create leaves value unknown",
			resourceState: nullResourceState,
			configValue:   types.StringNull(),
			planValue:     types.StringUnknown(),
			stateValue:    types.StringNull(),
			want:          types.StringUnknown(),
		},
		{
			name:          "update preserves null state",
			resourceState: existingResourceState,
			configValue:   types.StringNull(),
			planValue:     types.StringUnknown(),
			stateValue:    types.StringNull(),
			want:          types.StringNull(),
		},
		{
			name:          "update preserves known state",
			resourceState: existingResourceState,
			configValue:   types.StringNull(),
			planValue:     types.StringUnknown(),
			stateValue:    types.StringValue("classic"),
			want:          types.StringValue("classic"),
		},
		{
			name:          "explicit plan remains unchanged",
			resourceState: existingResourceState,
			configValue:   types.StringValue("lightning"),
			planValue:     types.StringValue("lightning"),
			stateValue:    types.StringNull(),
			want:          types.StringValue("lightning"),
		},
		{
			name:          "unknown configuration remains unknown",
			resourceState: existingResourceState,
			configValue:   types.StringUnknown(),
			planValue:     types.StringUnknown(),
			stateValue:    types.StringValue("classic"),
			want:          types.StringUnknown(),
		},
	}

	modifier := useStateForUnknownIncludingNull()
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			request := planmodifier.StringRequest{
				ConfigValue: test.configValue,
				PlanValue:   test.planValue,
				State:       test.resourceState,
				StateValue:  test.stateValue,
			}
			response := planmodifier.StringResponse{PlanValue: test.planValue}

			modifier.PlanModifyString(context.Background(), request, &response)

			require.True(t, test.want.Equal(response.PlanValue),
				"expected %s, got %s", test.want, response.PlanValue)
		})
	}
}
