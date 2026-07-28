package resources

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
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
	m, diags := types.MapValue(types.StringType, elems)
	require.False(t, diags.HasError())
	return m
}

// brokerConfigEntriesOf builds the extracted form of a `broker_configuration` map, so tests
// can include values that are not known until apply.
func brokerConfigEntriesOf(kv map[string]types.String) map[string]types.String {
	out := make(map[string]types.String, len(kv))
	for k, v := range kv {
		out[k] = v
	}
	return out
}

func TestBrokerConfigTableIsConsistent(t *testing.T) {
	t.Parallel()

	for key, spec := range brokerConfigs {
		if spec.AliasOf != "" {
			// An alias must point at a key that exists and is itself canonical, otherwise the
			// error message we hand the user would send them somewhere invalid.
			target, ok := brokerConfigs[spec.AliasOf]
			require.True(t, ok, "%s aliases unknown key %s", key, spec.AliasOf)
			require.Empty(t, target.AliasOf, "%s aliases %s, which is itself an alias", key, spec.AliasOf)
			require.Empty(t, spec.TypedAttr, "alias %s must not claim a typed attribute", key)
			continue
		}
		if spec.Kind == brokerConfigEnum {
			require.NotEmpty(t, spec.EnumValues, "enum config %s declares no valid values", key)
		}
	}

	// Every typed attribute must resolve back to exactly one canonical key.
	require.Equal(t, map[string]string{
		"auto_create_topic":              "auto.create.topics.enable",
		"default_num_partitions":         "num.partitions",
		"default_retention_millis":       "log.retention.ms",
		"default_topic_type":             "warpstream.default.topic.type",
		"enable_soft_topic_deletion":     "warpstream.soft.delete.topic.enable",
		"soft_topic_deletion_ttl_millis": "warpstream.soft.delete.topic.ttl.ms",
	}, typedAttrBrokerKey)
}

func TestValidateBrokerConfigKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		key     string
		wantErr string
	}{
		{name: "supported generic key", key: "message.max.bytes"},
		{name: "supported typed-backed key", key: "log.retention.ms"},
		{name: "typo", key: "messge.max.bytes", wantErr: "is not a supported cluster broker config"},
		{name: "unsupported entirely", key: "some.other.setting", wantErr: "is not a supported cluster broker config"},
		{name: "retention minutes alias", key: "log.retention.minutes", wantErr: `specify this setting as "log.retention.ms"`},
		{name: "retention hours alias", key: "log.retention.hours", wantErr: `specify this setting as "log.retention.ms"`},
		{
			name:    "ttl hours alias",
			key:     "warpstream.soft.delete.topic.ttl.hours",
			wantErr: `specify this setting as "warpstream.soft.delete.topic.ttl.ms"`,
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

func TestValidateBrokerConfigValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		key     string
		value   string
		wantErr string
	}{
		// Canonical values are accepted.
		{name: "bool true", key: "delete.topic.enable", value: "true"},
		{name: "bool false", key: "delete.topic.enable", value: "false"},
		{name: "int", key: "message.max.bytes", value: "1048576"},
		{name: "int zero", key: "message.max.bytes", value: "0"},
		{name: "retention", key: "log.retention.ms", value: "604800000"},
		{name: "retention infinite", key: "log.retention.ms", value: "-1"},
		{name: "ttl infinite", key: "warpstream.soft.delete.topic.ttl.ms", value: "-1"},
		{name: "enum", key: "warpstream.default.topic.type", value: "lightning"},

		// The API accepts these, but rewrites them, so state would not match the config.
		{name: "bool T", key: "delete.topic.enable", value: "T", wantErr: `write this value as "true", not "T"`},
		{name: "bool TRUE", key: "delete.topic.enable", value: "TRUE", wantErr: `write this value as "true"`},
		{name: "bool 1", key: "delete.topic.enable", value: "1", wantErr: `write this value as "true"`},
		{name: "other negative retention", key: "log.retention.ms", value: "-5", wantErr: `write this value as "-1", not "-5"`},
		{name: "other negative ttl", key: "warpstream.soft.delete.topic.ttl.ms", value: "-100", wantErr: `write this value as "-1"`},
		{name: "int with plus sign", key: "message.max.bytes", value: "+1048576", wantErr: `write this value as "1048576"`},
		{name: "enum mixed case", key: "warpstream.default.topic.type", value: "Lightning", wantErr: `write this value as "lightning"`},
		{name: "enum padded", key: "warpstream.default.topic.type", value: " lightning ", wantErr: `write this value as "lightning"`},

		// Unparsable for the config's type.
		{name: "bool garbage", key: "delete.topic.enable", value: "yes-please", wantErr: "is not a boolean"},
		{name: "int garbage", key: "message.max.bytes", value: "1MB", wantErr: "is not an integer"},
		{name: "enum invalid", key: "warpstream.default.topic.type", value: "turbo", wantErr: "must be one of"},

		// A negative value on a config where negative has no special meaning stays as-is.
		{name: "plain negative int", key: "message.max.bytes", value: "-1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateBrokerConfigValue(tt.key, tt.value)
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

func TestResolveBrokerConfigOverrides(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		entries       map[string]types.String
		declaredTyped map[string]attr.Value
		wantOverrides map[string]brokerConfigOverride
		wantConflicts []brokerConfigConflict
	}{
		{
			name:          "key without a typed twin produces no override",
			entries:       brokerConfigEntriesOf(map[string]types.String{"message.max.bytes": types.StringValue("1048576")}),
			wantOverrides: map[string]brokerConfigOverride{},
		},
		{
			name:    "typed-backed key overrides its attribute",
			entries: brokerConfigEntriesOf(map[string]types.String{"log.retention.ms": types.StringValue("3600000")}),
			wantOverrides: map[string]brokerConfigOverride{
				"default_retention_millis": {Key: "log.retention.ms", Value: types.StringValue("3600000")},
			},
		},
		{
			name: "several typed-backed keys each override their attribute",
			entries: brokerConfigEntriesOf(map[string]types.String{
				"num.partitions":                types.StringValue("16"),
				"warpstream.default.topic.type": types.StringValue("lightning"),
				"message.max.bytes":             types.StringValue("1048576"),
			}),
			wantOverrides: map[string]brokerConfigOverride{
				"default_num_partitions": {Key: "num.partitions", Value: types.StringValue("16")},
				"default_topic_type":     {Key: "warpstream.default.topic.type", Value: types.StringValue("lightning")},
			},
		},
		{
			name:    "an unknown value still overrides but cannot conflict",
			entries: brokerConfigEntriesOf(map[string]types.String{"log.retention.ms": types.StringUnknown()}),
			declaredTyped: map[string]attr.Value{
				"default_retention_millis": types.Int64Value(86400000),
			},
			wantOverrides: map[string]brokerConfigOverride{
				"default_retention_millis": {Key: "log.retention.ms", Value: types.StringUnknown()},
			},
		},
		{
			name:    "both surfaces agree",
			entries: brokerConfigEntriesOf(map[string]types.String{"log.retention.ms": types.StringValue("3600000")}),
			declaredTyped: map[string]attr.Value{
				"default_retention_millis": types.Int64Value(3600000),
			},
			wantOverrides: map[string]brokerConfigOverride{
				"default_retention_millis": {Key: "log.retention.ms", Value: types.StringValue("3600000")},
			},
		},
		{
			name:    "both surfaces disagree",
			entries: brokerConfigEntriesOf(map[string]types.String{"log.retention.ms": types.StringValue("7200000")}),
			declaredTyped: map[string]attr.Value{
				"default_retention_millis": types.Int64Value(3600000),
			},
			wantOverrides: map[string]brokerConfigOverride{
				"default_retention_millis": {Key: "log.retention.ms", Value: types.StringValue("7200000")},
			},
			wantConflicts: []brokerConfigConflict{{
				Key:        "log.retention.ms",
				TypedAttr:  "default_retention_millis",
				MapValue:   "7200000",
				TypedValue: types.Int64Value(3600000),
			}},
		},
		{
			name:    "booleans compare by value, not string",
			entries: brokerConfigEntriesOf(map[string]types.String{"auto.create.topics.enable": types.StringValue("false")}),
			declaredTyped: map[string]attr.Value{
				"auto_create_topic": types.BoolValue(true),
			},
			wantOverrides: map[string]brokerConfigOverride{
				"auto_create_topic": {Key: "auto.create.topics.enable", Value: types.StringValue("false")},
			},
			wantConflicts: []brokerConfigConflict{{
				Key:        "auto.create.topics.enable",
				TypedAttr:  "auto_create_topic",
				MapValue:   "false",
				TypedValue: types.BoolValue(true),
			}},
		},
		{
			name:    "a typed attribute the user did not write never conflicts",
			entries: brokerConfigEntriesOf(map[string]types.String{"log.retention.ms": types.StringValue("7200000")}),
			// Empty: the schema's default is not something the user wrote.
			declaredTyped: map[string]attr.Value{},
			wantOverrides: map[string]brokerConfigOverride{
				"default_retention_millis": {Key: "log.retention.ms", Value: types.StringValue("7200000")},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			overrides, conflicts, err := resolveBrokerConfigOverrides(tt.entries, tt.declaredTyped)
			require.NoError(t, err)
			require.Equal(t, tt.wantOverrides, overrides)
			require.Equal(t, tt.wantConflicts, conflicts)
		})
	}
}

func TestPlannedTypedValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		override brokerConfigOverride
		want     attr.Value
	}{
		{
			name:     "known int",
			override: brokerConfigOverride{Key: "log.retention.ms", Value: types.StringValue("3600000")},
			want:     types.Int64Value(3600000),
		},
		{
			name:     "known bool",
			override: brokerConfigOverride{Key: "auto.create.topics.enable", Value: types.StringValue("false")},
			want:     types.BoolValue(false),
		},
		{
			name:     "known enum",
			override: brokerConfigOverride{Key: "warpstream.default.topic.type", Value: types.StringValue("lightning")},
			want:     types.StringValue("lightning"),
		},
		// An unresolved value must plan as known-after-apply of the matching type, so the
		// apply is free to write whatever the value turns out to be.
		{
			name:     "unknown int",
			override: brokerConfigOverride{Key: "log.retention.ms", Value: types.StringUnknown()},
			want:     types.Int64Unknown(),
		},
		{
			name:     "unknown bool",
			override: brokerConfigOverride{Key: "auto.create.topics.enable", Value: types.StringUnknown()},
			want:     types.BoolUnknown(),
		},
		{
			name:     "unknown enum",
			override: brokerConfigOverride{Key: "warpstream.default.topic.type", Value: types.StringUnknown()},
			want:     types.StringUnknown(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := plannedTypedValue(tt.override)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
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

func TestFilterClusterConfigsToDeclared_EmptyIsNull(t *testing.T) {
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
