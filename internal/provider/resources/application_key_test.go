package resources

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stretchr/testify/require"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/api"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/models"
)

func applicationKeySchema(t *testing.T) schema.Schema {
	t.Helper()

	var resp resource.SchemaResponse
	(&applicationKeyResource{}).Schema(context.Background(), resource.SchemaRequest{}, &resp)
	require.False(t, resp.Diagnostics.HasError(), "schema: %v", resp.Diagnostics)
	return resp.Schema
}

func applicationKeyPlan(t *testing.T, model models.ApplicationKey) tfsdk.Plan {
	t.Helper()

	plan := tfsdk.Plan{Schema: applicationKeySchema(t)}
	require.False(t, plan.Set(context.Background(), &model).HasError())
	return plan
}

func applicationKeyConfig(t *testing.T, model models.ApplicationKey) tfsdk.Config {
	t.Helper()

	plan := applicationKeyPlan(t, model)
	return tfsdk.Config(plan)
}

func applicationKeyState(t *testing.T, model models.ApplicationKey) tfsdk.State {
	t.Helper()

	state := tfsdk.State{Schema: applicationKeySchema(t)}
	require.False(t, state.Set(context.Background(), &model).HasError())
	return state
}

func nullApplicationKeyPlan(t *testing.T) tfsdk.Plan {
	t.Helper()

	resourceSchema := applicationKeySchema(t)
	objectType := resourceSchema.Type().TerraformType(context.Background())
	return tfsdk.Plan{
		Schema: resourceSchema,
		Raw:    tftypes.NewValue(objectType, nil),
	}
}

func nullApplicationKeyState(t *testing.T) tfsdk.State {
	t.Helper()

	resourceSchema := applicationKeySchema(t)
	objectType := resourceSchema.Type().TerraformType(context.Background())
	return tfsdk.State{
		Schema: resourceSchema,
		Raw:    tftypes.NewValue(objectType, nil),
	}
}

func applicationKeyPlanModel(virtualClusterID, resourceKind types.String, readOnly types.Bool) models.ApplicationKey {
	return models.ApplicationKey{
		ID:               types.StringUnknown(),
		Name:             types.StringValue("akn_test"),
		Key:              types.StringUnknown(),
		WorkspaceID:      types.StringUnknown(),
		VirtualClusterID: virtualClusterID,
		ResourceKind:     resourceKind,
		CreatedAt:        types.StringUnknown(),
		ReadOnly:         readOnly,
	}
}

func applicationKeyConfigModel(virtualClusterID, resourceKind types.String, readOnly types.Bool) models.ApplicationKey {
	return models.ApplicationKey{
		ID:               types.StringNull(),
		Name:             types.StringValue("akn_test"),
		Key:              types.StringNull(),
		WorkspaceID:      types.StringNull(),
		VirtualClusterID: virtualClusterID,
		ResourceKind:     resourceKind,
		CreatedAt:        types.StringNull(),
		ReadOnly:         readOnly,
	}
}

func applicationKeyStateModel(readOnly bool) models.ApplicationKey {
	return models.ApplicationKey{
		ID:               types.StringValue("aki_test"),
		Name:             types.StringValue("akn_test"),
		Key:              types.StringValue("aks_test"),
		WorkspaceID:      types.StringValue("wi_test"),
		VirtualClusterID: types.StringNull(),
		ResourceKind:     types.StringNull(),
		CreatedAt:        types.StringValue("2026-08-12T00:00:00Z"),
		ReadOnly:         types.BoolValue(readOnly),
	}
}

func TestApplicationKeyValidateConfigClusterScopedReadOnly(t *testing.T) {
	t.Parallel()

	knownClusterID := types.StringValue("vci_test")
	knownResourceKind := types.StringValue(api.ResourceKindVirtualClusterTopics)

	tests := []struct {
		name      string
		config    tfsdk.Config
		wantError bool
	}{
		{
			name: "explicit conflict",
			config: applicationKeyConfig(t, applicationKeyConfigModel(
				knownClusterID, knownResourceKind, types.BoolValue(true),
			)),
			wantError: true,
		},
		{
			name: "unknown referenced cluster ID with explicit conflict",
			config: applicationKeyConfig(t, applicationKeyConfigModel(
				types.StringUnknown(), knownResourceKind, types.BoolValue(true),
			)),
			wantError: true,
		},
		{
			name: "explicit false",
			config: applicationKeyConfig(t, applicationKeyConfigModel(
				knownClusterID, knownResourceKind, types.BoolValue(false),
			)),
		},
		{
			name: "unknown read only",
			config: applicationKeyConfig(t, applicationKeyConfigModel(
				knownClusterID, knownResourceKind, types.BoolUnknown(),
			)),
		},
		{
			name: "omitted read only",
			config: applicationKeyConfig(t, applicationKeyConfigModel(
				knownClusterID, knownResourceKind, types.BoolNull(),
			)),
		},
		{
			name: "workspace scoped read only",
			config: applicationKeyConfig(t, applicationKeyConfigModel(
				types.StringNull(), types.StringNull(), types.BoolValue(true),
			)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			resp := &resource.ValidateConfigResponse{}
			(&applicationKeyResource{}).ValidateConfig(
				context.Background(),
				resource.ValidateConfigRequest{Config: tt.config},
				resp,
			)

			if !tt.wantError {
				require.Empty(t, resp.Diagnostics)
				return
			}

			require.Len(t, resp.Diagnostics.Errors(), 1)
			require.Contains(t, resp.Diagnostics.Errors()[0].Detail(), clusterScopedReadOnlyError)
			withPath, ok := resp.Diagnostics.Errors()[0].(diag.DiagnosticWithPath)
			require.True(t, ok)
			require.Equal(t, "read_only", withPath.Path().String())
		})
	}
}

func TestApplicationKeyModifyPlanClusterScopedReadOnly(t *testing.T) {
	t.Parallel()

	knownClusterID := types.StringValue("vci_test")
	knownResourceKind := types.StringValue(api.ResourceKindVirtualClusterTopics)

	tests := []struct {
		name      string
		plan      tfsdk.Plan
		config    tfsdk.Config
		state     tfsdk.State
		wantError bool
	}{
		{
			name: "explicit conflict",
			plan: applicationKeyPlan(t, applicationKeyPlanModel(
				knownClusterID, knownResourceKind, types.BoolValue(true),
			)),
			config: applicationKeyConfig(t, applicationKeyConfigModel(
				knownClusterID, knownResourceKind, types.BoolValue(true),
			)),
			state:     nullApplicationKeyState(t),
			wantError: true,
		},
		{
			name: "unknown cluster ID with explicit conflict",
			plan: applicationKeyPlan(t, applicationKeyPlanModel(
				types.StringUnknown(), knownResourceKind, types.BoolValue(true),
			)),
			config: applicationKeyConfig(t, applicationKeyConfigModel(
				types.StringUnknown(), knownResourceKind, types.BoolValue(true),
			)),
			state:     nullApplicationKeyState(t),
			wantError: true,
		},
		{
			name: "omitted read only on create",
			plan: applicationKeyPlan(t, applicationKeyPlanModel(
				types.StringUnknown(), knownResourceKind, types.BoolUnknown(),
			)),
			config: applicationKeyConfig(t, applicationKeyConfigModel(
				types.StringUnknown(), knownResourceKind, types.BoolNull(),
			)),
			state: nullApplicationKeyState(t),
		},
		{
			name: "inherited read only state",
			plan: applicationKeyPlan(t, applicationKeyPlanModel(
				knownClusterID, knownResourceKind, types.BoolUnknown(),
			)),
			config: applicationKeyConfig(t, applicationKeyConfigModel(
				knownClusterID, knownResourceKind, types.BoolNull(),
			)),
			state:     applicationKeyState(t, applicationKeyStateModel(true)),
			wantError: true,
		},
		{
			name: "explicit false",
			plan: applicationKeyPlan(t, applicationKeyPlanModel(
				knownClusterID, knownResourceKind, types.BoolValue(false),
			)),
			config: applicationKeyConfig(t, applicationKeyConfigModel(
				knownClusterID, knownResourceKind, types.BoolValue(false),
			)),
			state: applicationKeyState(t, applicationKeyStateModel(true)),
		},
		{
			name: "workspace scoped read only",
			plan: applicationKeyPlan(t, applicationKeyPlanModel(
				types.StringNull(), types.StringNull(), types.BoolValue(true),
			)),
			config: applicationKeyConfig(t, applicationKeyConfigModel(
				types.StringNull(), types.StringNull(), types.BoolValue(true),
			)),
			state: nullApplicationKeyState(t),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			resp := &resource.ModifyPlanResponse{}
			(&applicationKeyResource{}).ModifyPlan(
				context.Background(),
				resource.ModifyPlanRequest{
					Config: tt.config,
					Plan:   tt.plan,
					State:  tt.state,
				},
				resp,
			)

			if !tt.wantError {
				require.Empty(t, resp.Diagnostics)
				return
			}

			require.Len(t, resp.Diagnostics.Errors(), 1)
			require.Contains(t, resp.Diagnostics.Errors()[0].Detail(), clusterScopedReadOnlyError)
			withPath, ok := resp.Diagnostics.Errors()[0].(diag.DiagnosticWithPath)
			require.True(t, ok)
			require.Equal(t, "read_only", withPath.Path().String())
		})
	}
}

func TestApplicationKeyModifyPlanSkipsDestroy(t *testing.T) {
	t.Parallel()

	resp := &resource.ModifyPlanResponse{}
	(&applicationKeyResource{}).ModifyPlan(
		context.Background(),
		resource.ModifyPlanRequest{Plan: nullApplicationKeyPlan(t)},
		resp,
	)

	require.Empty(t, resp.Diagnostics)
}
