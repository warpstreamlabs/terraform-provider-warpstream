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

	// Every alias must point at a config the provider would actually accept, so the advice in the
	// error message names something usable. That means anything except another rejected alias —
	// not necessarily a mirrored config, since most configs have no typed attribute at all.
	for alias, canonical := range writeOnlyAliasKeys {
		require.NotEqual(t, alias, canonical, "alias %s points at itself", alias)
		require.NotContains(t, brokerKeyTypedAttr, alias, "alias %s must not also be mirrored", alias)
		require.NotContains(t, writeOnlyAliasKeys, canonical,
			"alias %s points at %s, which is itself rejected as an alias", alias, canonical)
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
		{name: "config with a typed twin", key: "log.retention.ms"},
		{name: "name the provider has never heard of", key: "some.brand.new.config"},
		{name: "typo is left to the API to reject", key: "messge.max.bytes"},

		// Aliases are the one exception: the API accepts them but never reports them back, so
		// Terraform could not track the result.
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

func TestBrokerConfigValueAs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		raw     string
		tmpl    attr.Value
		want    attr.Value
		wantErr string
	}{
		{name: "bool", raw: "true", tmpl: types.BoolValue(false), want: types.BoolValue(true)},
		// Parsing is lenient so that comparing a declared value against the typed attribute is
		// about the value, not its spelling. A spelling the API rewrites is caught after apply.
		{name: "lenient bool", raw: "TRUE", tmpl: types.BoolValue(false), want: types.BoolValue(true)},
		{name: "int", raw: "3600000", tmpl: types.Int64Value(0), want: types.Int64Value(3600000)},
		{name: "signed int", raw: "+3600000", tmpl: types.Int64Value(0), want: types.Int64Value(3600000)},
		{name: "negative int", raw: "-1", tmpl: types.Int64Value(0), want: types.Int64Value(-1)},
		{name: "string", raw: "lightning", tmpl: types.StringValue(""), want: types.StringValue("lightning")},
		{name: "not a bool", raw: "yes-please", tmpl: types.BoolValue(false), wantErr: "is not a boolean"},
		{name: "not an int", raw: "1MB", tmpl: types.Int64Value(0), wantErr: "is not an integer"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := brokerConfigValueAs(tt.raw, tt.tmpl)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
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

	// wantParse is the checkable part of a brokerConfigParseError. The error text is matched by
	// substring so the test does not have to restate strconv's exact wording.
	type wantParse struct {
		Key         string
		TypedAttr   string
		ErrContains string
	}

	tests := []struct {
		name          string
		entries       map[string]types.String
		declaredTyped map[string]attr.Value
		wantOverrides map[string]brokerConfigOverride
		wantConflicts []brokerConfigConflict
		wantParseErrs []wantParse
	}{
		{
			name:          "key without a typed twin produces no override",
			entries:       map[string]types.String{"message.max.bytes": types.StringValue("1048576")},
			wantOverrides: map[string]brokerConfigOverride{},
		},
		{
			name:    "typed-backed key overrides its attribute",
			entries: map[string]types.String{"log.retention.ms": types.StringValue("3600000")},
			wantOverrides: map[string]brokerConfigOverride{
				"default_retention_millis": {Key: "log.retention.ms", Value: types.StringValue("3600000")},
			},
		},
		{
			name: "several typed-backed keys each override their attribute",
			entries: map[string]types.String{
				"num.partitions":                types.StringValue("16"),
				"warpstream.default.topic.type": types.StringValue("lightning"),
				"message.max.bytes":             types.StringValue("1048576"),
			},
			wantOverrides: map[string]brokerConfigOverride{
				"default_num_partitions": {Key: "num.partitions", Value: types.StringValue("16")},
				"default_topic_type":     {Key: "warpstream.default.topic.type", Value: types.StringValue("lightning")},
			},
		},
		{
			name:    "an unknown value still overrides but cannot conflict",
			entries: map[string]types.String{"log.retention.ms": types.StringUnknown()},
			declaredTyped: map[string]attr.Value{
				"default_retention_millis": types.Int64Value(86400000),
			},
			wantOverrides: map[string]brokerConfigOverride{
				"default_retention_millis": {Key: "log.retention.ms", Value: types.StringUnknown()},
			},
		},
		{
			name:    "both surfaces agree",
			entries: map[string]types.String{"log.retention.ms": types.StringValue("3600000")},
			declaredTyped: map[string]attr.Value{
				"default_retention_millis": types.Int64Value(3600000),
			},
			wantOverrides: map[string]brokerConfigOverride{
				"default_retention_millis": {Key: "log.retention.ms", Value: types.StringValue("3600000")},
			},
		},
		{
			name:    "both surfaces disagree",
			entries: map[string]types.String{"log.retention.ms": types.StringValue("7200000")},
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
			entries: map[string]types.String{"auto.create.topics.enable": types.StringValue("false")},
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
			entries: map[string]types.String{"log.retention.ms": types.StringValue("7200000")},
			// Empty: the schema's default is not something the user wrote.
			declaredTyped: map[string]attr.Value{},
			wantOverrides: map[string]brokerConfigOverride{
				"default_retention_millis": {Key: "log.retention.ms", Value: types.StringValue("7200000")},
			},
		},
		{
			name:    "a value that cannot be compared names its key",
			entries: map[string]types.String{"log.retention.ms": types.StringValue("1MB")},
			declaredTyped: map[string]attr.Value{
				"default_retention_millis": types.Int64Value(3600000),
			},
			wantOverrides: map[string]brokerConfigOverride{
				"default_retention_millis": {Key: "log.retention.ms", Value: types.StringValue("1MB")},
			},
			wantParseErrs: []wantParse{{
				Key:         "log.retention.ms",
				TypedAttr:   "default_retention_millis",
				ErrContains: "is not an integer",
			}},
		},
		{
			// One bad value must not hide the next: the diagnostics name every offending key.
			name: "several unusable values are all reported",
			entries: map[string]types.String{
				"log.retention.ms":          types.StringValue("1MB"),
				"auto.create.topics.enable": types.StringValue("yes-please"),
			},
			declaredTyped: map[string]attr.Value{
				"default_retention_millis": types.Int64Value(3600000),
				"auto_create_topic":        types.BoolValue(true),
			},
			wantOverrides: map[string]brokerConfigOverride{
				"default_retention_millis": {Key: "log.retention.ms", Value: types.StringValue("1MB")},
				"auto_create_topic":        {Key: "auto.create.topics.enable", Value: types.StringValue("yes-please")},
			},
			// Sorted by key, so the output does not shuffle between runs.
			wantParseErrs: []wantParse{
				{
					Key:         "auto.create.topics.enable",
					TypedAttr:   "auto_create_topic",
					ErrContains: "is not a boolean",
				},
				{
					Key:         "log.retention.ms",
					TypedAttr:   "default_retention_millis",
					ErrContains: "is not an integer",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			overrides, conflicts, parseErrs := resolveBrokerConfigOverrides(tt.entries, tt.declaredTyped)
			require.Equal(t, tt.wantOverrides, overrides)
			require.Equal(t, tt.wantConflicts, conflicts)

			require.Len(t, parseErrs, len(tt.wantParseErrs))
			for i, want := range tt.wantParseErrs {
				require.Equal(t, want.Key, parseErrs[i].Key)
				require.Equal(t, want.TypedAttr, parseErrs[i].TypedAttr)
				require.ErrorContains(t, parseErrs[i].Err, want.ErrContains)
			}
		})
	}
}

func TestPlannedTypedConfiguration(t *testing.T) {
	t.Parallel()

	// The plan as Terraform hands it over: `default_retention_millis` is mirrored by a map entry,
	// the other two are not and hold their schema defaults because the user wrote neither.
	planAttrs := map[string]attr.Value{
		"default_retention_millis":   types.Int64Value(86400000),
		"enable_acls":                types.BoolValue(false),
		"enable_deletion_protection": types.BoolValue(false),
	}
	overrides := map[string]brokerConfigOverride{
		"default_retention_millis": {Key: "log.retention.ms", Value: types.StringValue("3600000")},
	}
	stateBroker := map[string]string{"log.retention.ms": "3600000"}

	t.Run("an attribute the user deleted reverts to its planned default", func(t *testing.T) {
		t.Parallel()

		// The user previously wrote `enable_acls = true`, applied, and has now deleted it. Prior
		// state therefore says true while the plan says the default false. Carrying prior state
		// over here would make the deletion a silent no-op and leave ACLs enabled.
		stateAttrs := map[string]attr.Value{
			"default_retention_millis":   types.Int64Value(3600000),
			"enable_acls":                types.BoolValue(true),
			"enable_deletion_protection": types.BoolValue(false),
		}

		got := plannedTypedConfiguration(planAttrs, overrides, map[string]attr.Value{}, stateAttrs, stateBroker)

		require.Equal(t, types.BoolValue(false), got["enable_acls"])
		require.Equal(t, types.BoolValue(false), got["enable_deletion_protection"])
		// The mirrored attribute is unchanged since the last apply, so it reuses prior state.
		require.Equal(t, types.Int64Value(3600000), got["default_retention_millis"])
	})

	t.Run("an attribute unknown until apply stays unknown", func(t *testing.T) {
		t.Parallel()

		// `enable_acls` derives from another resource, so the config value is unknown and the
		// plan carries that through. Replacing it with prior state's known value would make
		// Terraform abort with "planned value does not match config value".
		unknownPlan := map[string]attr.Value{
			"default_retention_millis": types.Int64Value(86400000),
			"enable_acls":              types.BoolUnknown(),
		}
		stateAttrs := map[string]attr.Value{
			"default_retention_millis": types.Int64Value(3600000),
			"enable_acls":              types.BoolValue(false),
		}

		// An unknown config value is not in declaredTypedAttrs, which is what used to let prior
		// state win.
		got := plannedTypedConfiguration(unknownPlan, overrides, map[string]attr.Value{}, stateAttrs, stateBroker)

		require.Equal(t, types.BoolUnknown(), got["enable_acls"])
	})

	t.Run("a mirrored attribute the user also wrote keeps the written value", func(t *testing.T) {
		t.Parallel()

		declaredTyped := map[string]attr.Value{"default_retention_millis": types.Int64Value(3600000)}
		written := map[string]attr.Value{"default_retention_millis": types.Int64Value(3600000)}

		got := plannedTypedConfiguration(written, overrides, declaredTyped, nil, nil)

		require.Equal(t, types.Int64Value(3600000), got["default_retention_millis"])
	})

	t.Run("a changed map entry makes its attribute known-after-apply", func(t *testing.T) {
		t.Parallel()

		changed := map[string]brokerConfigOverride{
			"default_retention_millis": {Key: "log.retention.ms", Value: types.StringValue("7200000")},
		}
		stateAttrs := map[string]attr.Value{"default_retention_millis": types.Int64Value(3600000)}

		got := plannedTypedConfiguration(planAttrs, changed, map[string]attr.Value{}, stateAttrs, stateBroker)

		require.Equal(t, types.Int64Unknown(), got["default_retention_millis"])
	})

	t.Run("every planned attribute is accounted for", func(t *testing.T) {
		t.Parallel()

		got := plannedTypedConfiguration(planAttrs, overrides, map[string]attr.Value{}, nil, nil)
		require.Len(t, got, len(planAttrs))
	})
}

func TestPlannedMirroredValue(t *testing.T) {
	t.Parallel()

	const key = "log.retention.ms"
	planVal := types.Int64Value(86400000) // the schema default, which must never win

	tests := []struct {
		name        string
		declared    types.String
		priorVal    attr.Value
		priorBroker map[string]string
		want        attr.Value
	}{
		// Nothing to carry over on a first apply, so the value comes from the API afterwards.
		{
			name:     "no prior state",
			declared: types.StringValue("3600000"),
			want:     types.Int64Unknown(),
		},
		// The declaration is unchanged, so prior state already holds whatever the API reported
		// and the plan can settle.
		{
			name:        "unchanged declaration reuses prior state",
			declared:    types.StringValue("3600000"),
			priorVal:    types.Int64Value(3600000),
			priorBroker: map[string]string{key: "3600000"},
			want:        types.Int64Value(3600000),
		},
		// Infinite is the case that motivates all of this: the map says -1 but the typed field
		// may report something else entirely, so prior state is the only reliable source.
		{
			name:        "infinite value reuses whatever the API reported",
			declared:    types.StringValue("-1"),
			priorVal:    types.Int64Value(3153600000000),
			priorBroker: map[string]string{key: "-1"},
			want:        types.Int64Value(3153600000000),
		},
		{
			name:        "changed declaration cannot reuse prior state",
			declared:    types.StringValue("7200000"),
			priorVal:    types.Int64Value(3600000),
			priorBroker: map[string]string{key: "3600000"},
			want:        types.Int64Unknown(),
		},
		{
			name:        "value unknown until apply",
			declared:    types.StringUnknown(),
			priorVal:    types.Int64Value(3600000),
			priorBroker: map[string]string{key: "3600000"},
			want:        types.Int64Unknown(),
		},
		{
			name:        "prior state null",
			declared:    types.StringValue("3600000"),
			priorVal:    types.Int64Null(),
			priorBroker: map[string]string{key: "3600000"},
			want:        types.Int64Unknown(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := plannedMirroredValue(
				brokerConfigOverride{Key: key, Value: tt.declared},
				planVal, tt.priorVal, tt.priorBroker,
			)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestUnknownLike(t *testing.T) {
	t.Parallel()

	require.Equal(t, types.BoolUnknown(), unknownLike(types.BoolValue(true)))
	require.Equal(t, types.Int64Unknown(), unknownLike(types.Int64Value(1)))
	require.Equal(t, types.StringUnknown(), unknownLike(types.StringValue("x")))
	// An already-unknown value stays unknown rather than becoming known.
	require.Equal(t, types.Int64Unknown(), unknownLike(types.Int64Unknown()))
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
