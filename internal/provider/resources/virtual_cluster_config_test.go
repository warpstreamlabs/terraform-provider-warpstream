package resources

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stretchr/testify/require"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/models"
)

// brokerConfigMapOf builds a `broker_configuration` value from string values.
func brokerConfigMapOf(t *testing.T, kv map[string]string) types.Map {
	t.Helper()
	elems := make(map[string]attr.Value, len(kv))
	for k, v := range kv {
		elems[k] = types.StringValue(v)
	}
	return types.MapValueMust(types.StringType, elems)
}

// virtualClusterSchema returns the resource's own schema, so plan fixtures cannot drift from it.
func virtualClusterSchema(t *testing.T) schema.Schema {
	t.Helper()
	var resp resource.SchemaResponse
	(&virtualClusterResource{}).Schema(context.Background(), resource.SchemaRequest{}, &resp)
	require.False(t, resp.Diagnostics.HasError(), "schema: %v", resp.Diagnostics)
	return resp.Schema
}

func planObjectType(t *testing.T) tftypes.Type {
	t.Helper()
	return virtualClusterSchema(t).Type().TerraformType(context.Background())
}

// planWithBrokerConfiguration builds a plan whose attributes are all null except
// `broker_configuration`, which is the only one ModifyPlan looks at.
func planWithBrokerConfiguration(t *testing.T, declared types.Map) tfsdk.Plan {
	t.Helper()

	ctx := context.Background()
	s := virtualClusterSchema(t)
	objType, ok := planObjectType(t).(tftypes.Object)
	require.True(t, ok)

	attrs := make(map[string]tftypes.Value, len(objType.AttributeTypes))
	for name, ty := range objType.AttributeTypes {
		attrs[name] = tftypes.NewValue(ty, nil)
	}
	raw, err := declared.ToTerraformValue(ctx)
	require.NoError(t, err)
	attrs["broker_configuration"] = raw

	return tfsdk.Plan{Schema: s, Raw: tftypes.NewValue(objType, attrs)}
}

func TestBrokerConfigTablesAgree(t *testing.T) {
	t.Parallel()

	// Every entry must name a distinct config name and a distinct typed attribute.
	// Two entries claiming either would mean one silently overwrites the other.
	seenKey := make(map[string]bool, len(typedAttrConfigs))
	seenAttr := make(map[string]string, len(typedAttrConfigs))
	for _, m := range typedAttrConfigs {
		require.False(t, seenKey[m.key], "config %q is listed twice", m.key)
		seenKey[m.key] = true
		if other, dup := seenAttr[m.typedAttr]; dup {
			t.Fatalf("configs %q and %q both claim attribute %q", other, m.key, m.typedAttr)
		}
		seenAttr[m.typedAttr] = m.key
	}

	// Every listed setting must be renderable, or brokerConfigsPayload would silently stop
	// writing it when a configuration sets it.
	populated := models.VirtualClusterConfiguration{
		AutoCreateTopic:         types.BoolValue(true),
		DefaultNumPartitions:    types.Int64Value(4),
		DefaultRetention:        types.Int64Value(86400000),
		DefaultTopicType:        types.StringValue("lightning"),
		EnableSoftTopicDeletion: types.BoolValue(false),
		SoftTopicDeletionTTL:    types.Int64Value(172800000),
	}
	for _, m := range typedAttrConfigs {
		_, ok := renderConfigValue(m.planValue(populated))
		require.True(t, ok, "config %q has no renderable plan value", m.key)
	}

	// Every alias must resolve to advice that names something usable: either a canonical key
	// the map accepts, or — when the canonical key is itself owned by a typed attribute — that
	// attribute. An alias pointing at another rejected alias would send users in a circle.
	for alias, canonical := range WriteOnlyAliasKeys {
		require.NotEqual(t, alias, canonical, "alias %s points at itself", alias)
		require.False(t, seenKey[alias], "alias %s must not also have a typed attribute", alias)
		require.NotContains(t, WriteOnlyAliasKeys, canonical,
			"alias %s points at %s, which is itself rejected as an alias", alias, canonical)
		if typedAttr, hasTypedAttr := typedAttrFor(canonical); hasTypedAttr {
			// The alias's error must redirect users to the typed attribute, not to the
			// canonical key, which the map also rejects.
			require.ErrorContains(t, validateBrokerConfigKey(alias), "configuration."+typedAttr,
				"alias %s must redirect to the typed attribute", alias)
			continue
		}
		require.NoError(t, validateBrokerConfigKey(canonical),
			"alias %s points at %s, which the provider would reject", alias, canonical)
	}
}

// TestTypedAttrConfigsIsClosed asserts the exact contents of typedAttrConfigs, because growing that
// table is a breaking change rather than an addition.
func TestTypedAttrConfigsIsClosed(t *testing.T) {
	t.Parallel()

	want := []string{
		"auto.create.topics.enable",
		"log.retention.ms",
		"num.partitions",
		"warpstream.default.topic.type",
		"warpstream.soft.delete.topic.enable",
		"warpstream.soft.delete.topic.ttl.ms",
	}

	got := make([]string, 0, len(typedAttrConfigs))
	for _, c := range typedAttrConfigs {
		got = append(got, c.key)
	}
	require.ElementsMatch(t, want, got)
}

// TestEveryConfigurationAttributeIsWritten asserts that every attribute under `configuration` is written to the API
// in some way.
func TestEveryConfigurationAttributeIsWritten(t *testing.T) {
	t.Parallel()

	// sentAsOwnField names the attributes ConfigurationUpdate carries as their own JSON field,
	// because the API has no Kafka-style config name for them. Everything else must be in
	// typedAttrConfigs.
	sentAsOwnField := map[string]bool{
		"enable_acls":                true,
		"enable_acl_shadowing":       true,
		"enable_deletion_protection": true,
	}

	cfgAttr, ok := virtualClusterSchema(t).Attributes["configuration"].(schema.SingleNestedAttribute)
	require.True(t, ok, "`configuration` is no longer a single nested attribute")

	keyByTypedAttr := make(map[string]string, len(typedAttrConfigs))
	for _, m := range typedAttrConfigs {
		keyByTypedAttr[m.typedAttr] = m.key
	}

	for name := range cfgAttr.Attributes {
		key, hasTypedAttr := keyByTypedAttr[name]
		if sentAsOwnField[name] {
			require.False(t, hasTypedAttr,
				"configuration.%s is sent as its own field but is also listed as %q; it would be "+
					"written through both representations, which the API rejects when they disagree", name, key)
			continue
		}
		require.True(t, hasTypedAttr,
			"configuration.%s is written nowhere: add it to typedAttrConfigs, or to sentAsOwnField "+
				"in this test if ConfigurationUpdate carries it as its own field", name)
	}

	// The reverse, so neither table can outlive the attribute it names.
	for _, m := range typedAttrConfigs {
		require.Contains(t, cfgAttr.Attributes, m.typedAttr,
			"listed config %q names attribute configuration.%s, which is not in the schema", m.key, m.typedAttr)
	}
	for name := range sentAsOwnField {
		require.Contains(t, cfgAttr.Attributes, name,
			"sentAsOwnField names configuration.%s, which is not in the schema", name)
	}
}

func TestValidateBrokerConfigKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		key     string
		wantErr string
	}{
		// Unknown names are deliberately accepted: the API decides what exists, so a config
		// added there must not need a provider release to become usable.
		{name: "config with no typed twin", key: "message.max.bytes"},
		{name: "name the provider has never heard of", key: "some.brand.new.config"},
		{name: "typo is left to the API to reject", key: "messge.max.bytes"},

		// Settings owned by a typed `configuration` attribute are rejected with a pointer to
		// that attribute: the two surfaces are disjoint.
		{name: "retention", key: "log.retention.ms", wantErr: "`configuration.default_retention_millis`"},
		{name: "auto create", key: "auto.create.topics.enable", wantErr: "`configuration.auto_create_topic`"},
		{name: "partitions", key: "num.partitions", wantErr: "`configuration.default_num_partitions`"},
		{name: "topic type", key: "warpstream.default.topic.type", wantErr: "`configuration.default_topic_type`"},
		{name: "soft delete", key: "warpstream.soft.delete.topic.enable", wantErr: "`configuration.enable_soft_topic_deletion`"},
		{name: "soft delete ttl", key: "warpstream.soft.delete.topic.ttl.ms", wantErr: "`configuration.soft_topic_deletion_ttl_millis`"},

		// Aliases the API accepts but never reports back. An alias for a typed-owned setting
		// redirects to the typed attribute; any other alias redirects to its canonical key.
		{name: "retention minutes alias", key: "log.retention.minutes", wantErr: "`configuration.default_retention_millis`"},
		{name: "retention hours alias", key: "log.retention.hours", wantErr: "`configuration.default_retention_millis`"},
		{
			name:    "ttl hours alias",
			key:     "warpstream.soft.delete.topic.ttl.hours",
			wantErr: "`configuration.soft_topic_deletion_ttl_millis`",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateBrokerConfigKey(tt.key)
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestBrokerConfigMapSkipsUnresolvedValues(t *testing.T) {
	t.Parallel()

	// A value derived from another resource is not known at plan time. Extracting it must skip
	// the entry rather than fail, or `terraform plan` breaks with a "report this to the provider
	// developer" error.
	m := types.MapValueMust(types.StringType, map[string]attr.Value{
		"message.max.bytes":   types.StringValue("1048576"),
		"log.retention.ms":    types.StringUnknown(),
		"delete.topic.enable": types.StringNull(),
	})

	require.Equal(t, map[string]string{"message.max.bytes": "1048576"}, brokerConfigMap(m))
}

func TestBrokerConfigMapNullOrUnknownMap(t *testing.T) {
	t.Parallel()

	require.Nil(t, brokerConfigMap(types.MapNull(types.StringType)))
	require.Nil(t, brokerConfigMap(types.MapUnknown(types.StringType)))
}

// TestModifyPlanRejectsInvalidKeys drives the plan-time validation end to end: every problem in
// the map is reported, each against its own key, so a configuration with several mistakes takes
// one round trip to fix.
func TestModifyPlanRejectsInvalidKeys(t *testing.T) {
	t.Parallel()

	declared := types.MapValueMust(types.StringType, map[string]attr.Value{
		"message.max.bytes":     types.StringValue("1048576"),
		"log.retention.ms":      types.StringValue("3600000"),
		"log.retention.hours":   types.StringValue("24"),
		"delete.topic.enable":   types.StringNull(),
		"some.brand.new.config": types.StringValue("on"),
	})

	resp := &resource.ModifyPlanResponse{}
	(&virtualClusterResource{}).ModifyPlan(
		context.Background(),
		resource.ModifyPlanRequest{Plan: planWithBrokerConfiguration(t, declared)},
		resp,
	)

	require.Len(t, resp.Diagnostics.Errors(), 3)
	byPath := map[string]string{}
	for _, d := range resp.Diagnostics.Errors() {
		withPath, ok := d.(diag.DiagnosticWithPath)
		require.True(t, ok, "every problem must name the key it is about")
		byPath[withPath.Path().String()] = d.Detail()
	}
	require.Contains(t, byPath[`broker_configuration["log.retention.ms"]`], "configuration.default_retention_millis")
	require.Contains(t, byPath[`broker_configuration["log.retention.hours"]`], "alternate unit")
	require.Contains(t, byPath[`broker_configuration["delete.topic.enable"]`], "null is not a valid value")
}

// TestModifyPlanSkipsUnvalidatableMaps covers the plans ModifyPlan must leave alone: a destroy
// carries no plan at all, and a map that is unknown as a whole has no keys to look at yet.
func TestModifyPlanSkipsUnvalidatableMaps(t *testing.T) {
	t.Parallel()

	for name, plan := range map[string]tfsdk.Plan{
		"destroy":     {Schema: virtualClusterSchema(t), Raw: tftypes.NewValue(planObjectType(t), nil)},
		"unknown map": planWithBrokerConfiguration(t, types.MapUnknown(types.StringType)),
		"absent map":  planWithBrokerConfiguration(t, types.MapNull(types.StringType)),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			resp := &resource.ModifyPlanResponse{}
			(&virtualClusterResource{}).ModifyPlan(
				context.Background(), resource.ModifyPlanRequest{Plan: plan}, resp,
			)
			require.Empty(t, resp.Diagnostics, "nothing to validate yet")
		})
	}
}

func TestCheckDeclaredConfigsApplied(t *testing.T) {
	t.Parallel()

	strPtr := func(s string) *string { return &s }

	tests := []struct {
		name       string
		declared   map[string]string
		apiConfigs map[string]*string
		wantErrs   []string
	}{
		{
			name:       "value stored verbatim",
			declared:   map[string]string{"message.max.bytes": "1048576"},
			apiConfigs: map[string]*string{"message.max.bytes": strPtr("1048576")},
		},
		{
			name:       "nothing declared",
			declared:   nil,
			apiConfigs: map[string]*string{"message.max.bytes": strPtr("1048576")},
		},
		{
			name:       "api did not report the config back",
			declared:   map[string]string{"message.max.bytes": "1048576"},
			apiConfigs: map[string]*string{},
			wantErrs:   []string{"did not report cluster config"},
		},
		{
			name:       "api reported a null value",
			declared:   map[string]string{"message.max.bytes": "1048576"},
			apiConfigs: map[string]*string{"message.max.bytes": nil},
			wantErrs:   []string{"did not report cluster config"},
		},
		{
			name:       "api changed the value",
			declared:   map[string]string{"log.retention.ms": "30"},
			apiConfigs: map[string]*string{"log.retention.ms": strPtr("60000")},
			wantErrs:   []string{`written as "30" but the API reports it as "60000"`},
		},
		{
			name: "several problems are all reported",
			declared: map[string]string{
				"message.max.bytes": "1048576",
				"log.retention.ms":  "30",
			},
			apiConfigs: map[string]*string{"log.retention.ms": strPtr("60000")},
			wantErrs: []string{
				`written as "30" but the API reports it as "60000"`,
				"did not report cluster config",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var diags diag.Diagnostics
			checkDeclaredConfigsApplied(tt.declared, tt.apiConfigs, &diags)

			if len(tt.wantErrs) == 0 {
				require.False(t, diags.HasError(), "unexpected diagnostics: %v", diags)
				return
			}
			require.True(t, diags.HasError())
			require.Len(t, diags.Errors(), len(tt.wantErrs))
			joined := ""
			for _, d := range diags.Errors() {
				joined += d.Detail() + "\n"
			}
			for _, want := range tt.wantErrs {
				require.Contains(t, joined, want)
			}
		})
	}
}

func TestFilterClusterConfigsToDeclared(t *testing.T) {
	t.Parallel()

	strPtr := func(s string) *string { return &s }

	apiConfigs := map[string]*string{
		"message.max.bytes":   strPtr("1048576"),
		"delete.topic.enable": strPtr("true"),
		"log.retention.ms":    strPtr("86400000"),
	}

	declared := brokerConfigMapOf(t, map[string]string{
		// Declared; value must come from the API, not the declaration.
		"message.max.bytes": "ignored",
		// Declared but not returned by the API -> dropped.
		"not.returned": "x",
	})

	got := filterClusterConfigsToDeclared(apiConfigs, declared)

	require.False(t, got.IsNull())
	elems := got.Elements()
	require.Len(t, elems, 1)
	require.Equal(t, types.StringValue("1048576"), elems["message.max.bytes"])
}

func TestFilterClusterConfigsToDeclared_AbsentStaysNull(t *testing.T) {
	t.Parallel()

	// Nothing declared -> null map, so an absent attribute round-trips to null.
	got := filterClusterConfigsToDeclared(map[string]*string{"a": nil}, types.MapNull(types.StringType))
	require.True(t, got.IsNull())
}

// TestFilterClusterConfigsToDeclared_DeclaredEmptyStaysEmpty pins the distinction between an
// absent `broker_configuration` and one declared as `{}`. The attribute is Optional and not
// Computed, so Terraform requires state after an apply to equal the configured value exactly:
// turning a declared empty map into null aborts the apply with "Provider produced inconsistent
// result after apply". `broker_configuration = var.configs` with a `{}` default hits this.
func TestFilterClusterConfigsToDeclared_DeclaredEmptyStaysEmpty(t *testing.T) {
	t.Parallel()

	got := filterClusterConfigsToDeclared(
		map[string]*string{"message.max.bytes": nil},
		brokerConfigMapOf(t, map[string]string{}),
	)
	require.False(t, got.IsNull(), "a declared empty map must not round-trip to null")
	require.Empty(t, got.Elements())
}

// TestFilterClusterConfigsToDeclared_NoneReturnedStaysEmpty is the same requirement for a
// non-empty declaration the API answers with nothing: the result is empty, not null. The apply
// still fails, but on checkDeclaredConfigsApplied's specific message rather than Terraform's
// generic inconsistent-result error.
func TestFilterClusterConfigsToDeclared_NoneReturnedStaysEmpty(t *testing.T) {
	t.Parallel()

	got := filterClusterConfigsToDeclared(
		map[string]*string{"something.else": nil},
		brokerConfigMapOf(t, map[string]string{"message.max.bytes": "1048576"}),
	)
	require.False(t, got.IsNull())
	require.Empty(t, got.Elements())
}

func TestBrokerConfigsPayload(t *testing.T) {
	t.Parallel()

	// An unset `configuration`: every attribute is null, as it would be if the schema stopped
	// defaulting the object.
	var unset models.VirtualClusterConfiguration

	t.Run("nothing set on either surface returns nil", func(t *testing.T) {
		t.Parallel()
		got, err := brokerConfigsPayload(unset, nil)
		require.NoError(t, err)
		require.Nil(t, got)
	})

	t.Run("generic entries pass through", func(t *testing.T) {
		t.Parallel()
		got, err := brokerConfigsPayload(unset, map[string]string{"message.max.bytes": "1048576"})
		require.NoError(t, err)
		require.Equal(t, map[string]string{"message.max.bytes": "1048576"}, got)
	})

	t.Run("typed attributes translate to canonical keys", func(t *testing.T) {
		t.Parallel()
		cfg := models.VirtualClusterConfiguration{
			AutoCreateTopic:         types.BoolValue(true),
			DefaultNumPartitions:    types.Int64Value(4),
			DefaultRetention:        types.Int64Value(86400000),
			EnableSoftTopicDeletion: types.BoolValue(false),
			DefaultTopicType:        types.StringValue("lightning"),
			SoftTopicDeletionTTL:    types.Int64Value(172800000),
		}
		got, err := brokerConfigsPayload(cfg, nil)
		require.NoError(t, err)
		require.Equal(t, map[string]string{
			"auto.create.topics.enable":           "true",
			"num.partitions":                      "4",
			"log.retention.ms":                    "86400000",
			"warpstream.soft.delete.topic.enable": "false",
			"warpstream.default.topic.type":       "lightning",
			"warpstream.soft.delete.topic.ttl.ms": "172800000",
		}, got)
	})

	t.Run("null and unknown typed attributes are skipped", func(t *testing.T) {
		t.Parallel()
		cfg := models.VirtualClusterConfiguration{
			AutoCreateTopic:         types.BoolNull(),
			DefaultNumPartitions:    types.Int64Unknown(),
			DefaultRetention:        types.Int64Unknown(),
			EnableSoftTopicDeletion: types.BoolNull(),
			DefaultTopicType:        types.StringNull(),
			SoftTopicDeletionTTL:    types.Int64Null(),
		}
		got, err := brokerConfigsPayload(cfg, nil)
		require.NoError(t, err)
		require.Nil(t, got)
	})

	t.Run("a setting on both surfaces is an error, not a precedence question", func(t *testing.T) {
		t.Parallel()
		cfg := models.VirtualClusterConfiguration{DefaultRetention: types.Int64Value(86400000)}
		got, err := brokerConfigsPayload(cfg, map[string]string{"log.retention.ms": "3600000"})
		require.Nil(t, got, "no payload may be built from a conflicting configuration")
		require.ErrorContains(t, err, "log.retention.ms")
		require.ErrorContains(t, err, "configuration.default_retention_millis")
		require.ErrorContains(t, err, "3600000")
		require.ErrorContains(t, err, "86400000")
	})
}
