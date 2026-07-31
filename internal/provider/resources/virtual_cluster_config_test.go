package resources

import (
	"context"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/require"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/api"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/models"
)

// brokerConfigMapOf builds a `broker_configuration` value from string values.
func brokerConfigMapOf(t *testing.T, kv map[string]string) types.Map {
	t.Helper()
	elems := make(map[string]attr.Value, len(kv))
	for k, v := range kv {
		elems[k] = types.StringValue(v)
	}
	m, diags := types.MapValue(types.StringType, elems)
	require.False(t, diags.HasError())
	return m
}

func TestBrokerConfigTablesAgree(t *testing.T) {
	t.Parallel()

	// Every mirrored config must name a distinct typed attribute. Two configs claiming the same
	// attribute would mean one silently overwrites the other.
	seen := make(map[string]string, len(brokerKeyTypedAttr))
	for key, attrName := range brokerKeyTypedAttr {
		if other, dup := seen[attrName]; dup {
			t.Fatalf("configs %q and %q both claim attribute %q", other, key, attrName)
		}
		seen[attrName] = key
	}

	// Every alias must resolve to advice that names something usable: either a canonical key
	// the map accepts, or — when the canonical key is itself owned by a typed attribute — that
	// attribute. An alias pointing at another rejected alias would send users in a circle.
	for alias, canonical := range writeOnlyAliasKeys {
		require.NotEqual(t, alias, canonical, "alias %s points at itself", alias)
		require.NotContains(t, brokerKeyTypedAttr, alias, "alias %s must not also be mirrored", alias)
		require.NotContains(t, writeOnlyAliasKeys, canonical,
			"alias %s points at %s, which is itself rejected as an alias", alias, canonical)
		if typedAttr, mirrored := brokerKeyTypedAttr[canonical]; mirrored {
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

func TestBrokerConfigEntriesKeepsUnknownValues(t *testing.T) {
	t.Parallel()

	// A value derived from another resource is not known at plan time. Extracting it must not
	// fail, or `terraform plan` breaks with a "report this to the provider developer" error.
	m, diags := types.MapValue(types.StringType, map[string]attr.Value{
		"message.max.bytes": types.StringValue("1048576"),
		"log.retention.ms":  types.StringUnknown(),
	})
	require.False(t, diags.HasError())

	var extractDiags diag.Diagnostics
	entries := brokerConfigEntries(context.Background(), m, &extractDiags)

	require.False(t, extractDiags.HasError(), "unknown value must not produce a diagnostic")
	require.Len(t, entries, 2)
	require.Equal(t, types.StringValue("1048576"), entries["message.max.bytes"])
	require.True(t, entries["log.retention.ms"].IsUnknown())
}

func TestBrokerConfigEntriesNullOrUnknownMap(t *testing.T) {
	t.Parallel()

	var diags diag.Diagnostics
	require.Nil(t, brokerConfigEntries(context.Background(), types.MapNull(types.StringType), &diags))
	require.Nil(t, brokerConfigEntries(context.Background(), types.MapUnknown(types.StringType), &diags))
	require.False(t, diags.HasError())
}

func TestBrokerConfigMapSkipsUnresolvedValues(t *testing.T) {
	t.Parallel()

	m, diags := types.MapValue(types.StringType, map[string]attr.Value{
		"message.max.bytes":   types.StringValue("1048576"),
		"log.retention.ms":    types.StringUnknown(),
		"delete.topic.enable": types.StringNull(),
	})
	require.False(t, diags.HasError())

	var extractDiags diag.Diagnostics
	got := brokerConfigMap(context.Background(), m, &extractDiags)

	require.False(t, extractDiags.HasError())
	require.Equal(t, map[string]string{"message.max.bytes": "1048576"}, got)
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

func TestCheckAPIConfigConsistency(t *testing.T) {
	t.Parallel()

	strPtr := func(s string) *string { return &s }
	boolPtr := func(b bool) *bool { return &b }
	i64Ptr := func(i int64) *int64 { return &i }
	durPtr := func(d time.Duration) *time.Duration { return &d }

	tests := []struct {
		name        string
		cfg         api.VirtualClusterConfiguration
		wantWarning string
	}{
		{
			name: "no broker configs means nothing to compare",
			cfg: api.VirtualClusterConfiguration{
				DefaultRetentionMillis: i64Ptr(86400000),
			},
		},
		{
			name: "both representations agree",
			cfg: api.VirtualClusterConfiguration{
				DefaultRetentionMillis: i64Ptr(86400000),
				AutoCreateTopic:        boolPtr(true),
				BrokerConfigs: map[string]*string{
					"log.retention.ms":          strPtr("86400000"),
					"auto.create.topics.enable": strPtr("true"),
				},
			},
		},
		{
			name: "a config absent from the map is on the built-in default and not compared",
			cfg: api.VirtualClusterConfiguration{
				DefaultRetentionMillis: i64Ptr(86400000),
				BrokerConfigs:          map[string]*string{"message.max.bytes": strPtr("1048576")},
			},
		},
		{
			name: "representations disagree",
			cfg: api.VirtualClusterConfiguration{
				DefaultRetentionMillis: i64Ptr(86400000),
				BrokerConfigs:          map[string]*string{"log.retention.ms": strPtr("3600000")},
			},
			wantWarning: `"log.retention.ms"`,
		},
		{
			name: "boolean disagreement",
			cfg: api.VirtualClusterConfiguration{
				AutoCreateTopic: boolPtr(true),
				BrokerConfigs:   map[string]*string{"auto.create.topics.enable": strPtr("false")},
			},
			wantWarning: `"auto.create.topics.enable"`,
		},
		// Infinite is encoded differently by the two representations, so a negative value on
		// either side is not a real disagreement.
		{
			name: "infinite retention is not a disagreement",
			cfg: api.VirtualClusterConfiguration{
				DefaultRetentionMillis: i64Ptr(3153600000000),
				BrokerConfigs:          map[string]*string{"log.retention.ms": strPtr("-1")},
			},
		},
		{
			name: "infinite soft-delete ttl is not a disagreement",
			cfg: api.VirtualClusterConfiguration{
				SoftTopicDeletionTTL: durPtr(100 * 365 * 24 * time.Hour),
				BrokerConfigs:        map[string]*string{"warpstream.soft.delete.topic.ttl.ms": strPtr("-1")},
			},
		},
		{
			name: "soft-delete ttl is compared in milliseconds",
			cfg: api.VirtualClusterConfiguration{
				SoftTopicDeletionTTL: durPtr(48 * time.Hour),
				BrokerConfigs:        map[string]*string{"warpstream.soft.delete.topic.ttl.ms": strPtr("172800000")},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var diags diag.Diagnostics
			checkAPIConfigConsistency(&tt.cfg, &diags)

			require.False(t, diags.HasError(), "must never be fatal")
			if tt.wantWarning == "" {
				require.Empty(t, diags.Warnings(), "unexpected warnings: %v", diags.Warnings())
				return
			}
			require.Len(t, diags.Warnings(), 1)
			require.Contains(t, diags.Warnings()[0].Detail(), tt.wantWarning)
			require.Contains(t, diags.Warnings()[0].Detail(), "report this issue to the provider developers")
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

	var diags diag.Diagnostics
	got := filterClusterConfigsToDeclared(context.Background(), apiConfigs, declared, &diags)

	require.False(t, diags.HasError())
	require.False(t, got.IsNull())
	elems := got.Elements()
	require.Len(t, elems, 1)
	require.Equal(t, types.StringValue("1048576"), elems["message.max.bytes"])
}

func TestFilterClusterConfigsToDeclared_AbsentStaysNull(t *testing.T) {
	t.Parallel()

	var diags diag.Diagnostics
	// Nothing declared -> null map, so an absent attribute round-trips to null.
	got := filterClusterConfigsToDeclared(
		context.Background(),
		map[string]*string{"a": nil},
		types.MapNull(types.StringType),
		&diags,
	)
	require.False(t, diags.HasError())
	require.True(t, got.IsNull())
}

// TestFilterClusterConfigsToDeclared_DeclaredEmptyStaysEmpty pins the distinction between an
// absent `broker_configuration` and one declared as `{}`. The attribute is Optional and not
// Computed, so Terraform requires state after an apply to equal the configured value exactly:
// turning a declared empty map into null aborts the apply with "Provider produced inconsistent
// result after apply". `broker_configuration = var.configs` with a `{}` default hits this.
func TestFilterClusterConfigsToDeclared_DeclaredEmptyStaysEmpty(t *testing.T) {
	t.Parallel()

	var diags diag.Diagnostics
	got := filterClusterConfigsToDeclared(
		context.Background(),
		map[string]*string{"message.max.bytes": nil},
		brokerConfigMapOf(t, map[string]string{}),
		&diags,
	)
	require.False(t, diags.HasError())
	require.False(t, got.IsNull(), "a declared empty map must not round-trip to null")
	require.Empty(t, got.Elements())
}

// TestFilterClusterConfigsToDeclared_NoneReturnedStaysEmpty is the same requirement for a
// non-empty declaration the API answers with nothing: the result is empty, not null. The apply
// still fails, but on checkDeclaredConfigsApplied's specific message rather than Terraform's
// generic inconsistent-result error.
func TestFilterClusterConfigsToDeclared_NoneReturnedStaysEmpty(t *testing.T) {
	t.Parallel()

	var diags diag.Diagnostics
	got := filterClusterConfigsToDeclared(
		context.Background(),
		map[string]*string{"something.else": nil},
		brokerConfigMapOf(t, map[string]string{"message.max.bytes": "1048576"}),
		&diags,
	)
	require.False(t, diags.HasError())
	require.False(t, got.IsNull())
	require.Empty(t, got.Elements())
}

func TestBrokerConfigsPayload(t *testing.T) {
	t.Parallel()

	deref := func(m map[string]*string) map[string]string {
		out := make(map[string]string, len(m))
		for k, v := range m {
			require.NotNil(t, v, "unexpected nil value for %s", k)
			out[k] = *v
		}
		return out
	}

	t.Run("nil plan and empty map returns nil", func(t *testing.T) {
		t.Parallel()
		require.Nil(t, brokerConfigsPayload(nil, nil))
	})

	t.Run("generic entries pass through", func(t *testing.T) {
		t.Parallel()
		got := brokerConfigsPayload(nil, map[string]string{"message.max.bytes": "1048576"})
		require.Equal(t, map[string]string{"message.max.bytes": "1048576"}, deref(got))
	})

	t.Run("typed attributes translate to canonical keys", func(t *testing.T) {
		t.Parallel()
		cfg := &models.VirtualClusterConfiguration{
			AutoCreateTopic:         types.BoolValue(true),
			DefaultNumPartitions:    types.Int64Value(4),
			DefaultRetention:        types.Int64Value(86400000),
			EnableSoftTopicDeletion: types.BoolValue(false),
			DefaultTopicType:        types.StringValue("lightning"),
			SoftTopicDeletionTTL:    types.Int64Value(172800000),
		}
		got := brokerConfigsPayload(cfg, nil)
		require.Equal(t, map[string]string{
			"auto.create.topics.enable":           "true",
			"num.partitions":                      "4",
			"log.retention.ms":                    "86400000",
			"warpstream.soft.delete.topic.enable": "false",
			"warpstream.default.topic.type":       "lightning",
			"warpstream.soft.delete.topic.ttl.ms": "172800000",
		}, deref(got))
	})

	t.Run("null and unknown typed attributes are skipped", func(t *testing.T) {
		t.Parallel()
		cfg := &models.VirtualClusterConfiguration{
			AutoCreateTopic:         types.BoolNull(),
			DefaultNumPartitions:    types.Int64Unknown(),
			DefaultRetention:        types.Int64Unknown(),
			EnableSoftTopicDeletion: types.BoolNull(),
			DefaultTopicType:        types.StringNull(),
			SoftTopicDeletionTTL:    types.Int64Null(),
		}
		require.Nil(t, brokerConfigsPayload(cfg, nil))
	})

	t.Run("generic map entry wins over typed attribute", func(t *testing.T) {
		t.Parallel()
		cfg := &models.VirtualClusterConfiguration{
			DefaultRetention: types.Int64Value(86400000),
		}
		got := brokerConfigsPayload(cfg, map[string]string{"log.retention.ms": "3600000"})
		require.Equal(t, map[string]string{"log.retention.ms": "3600000"}, deref(got))
	})
}
