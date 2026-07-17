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

func set(keys ...string) map[string]struct{} {
	out := make(map[string]struct{}, len(keys))
	for _, k := range keys {
		out[k] = struct{}{}
	}
	return out
}

func mapOf(t *testing.T, kv map[string]string) types.Map {
	t.Helper()
	elems := make(map[string]attr.Value, len(kv))
	for k, v := range kv {
		elems[k] = types.StringValue(v)
	}
	m, diags := types.MapValue(types.StringType, elems)
	require.False(t, diags.HasError())
	return m
}

func TestFindConfigCollisions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		typedAttrs  map[string]struct{}
		genericKeys map[string]struct{}
		wantPairs   [][2]string // {typedAttr, genericKey}
	}{
		{
			name:        "no typed attrs set",
			typedAttrs:  set(),
			genericKeys: set("log.retention.ms"),
		},
		{
			name:        "no generic keys",
			typedAttrs:  set("default_retention_millis"),
			genericKeys: set(),
		},
		{
			name:        "disjoint keys do not collide",
			typedAttrs:  set("default_retention_millis"),
			genericKeys: set("message.max.bytes"),
		},
		{
			name:        "typed attr with no generic equivalent never collides",
			typedAttrs:  set("enable_acls", "enable_deletion_protection"),
			genericKeys: set("auto.create.topics.enable"),
		},
		{
			name:        "direct collision",
			typedAttrs:  set("auto_create_topic"),
			genericKeys: set("auto.create.topics.enable"),
			wantPairs:   [][2]string{{"auto_create_topic", "auto.create.topics.enable"}},
		},
		{
			name:        "retention alias collides",
			typedAttrs:  set("default_retention_millis"),
			genericKeys: set("log.retention.hours"),
			wantPairs:   [][2]string{{"default_retention_millis", "log.retention.hours"}},
		},
		{
			name:        "collision only counts when typed attr explicitly set",
			typedAttrs:  set("default_num_partitions"),
			genericKeys: set("num.partitions", "log.retention.ms"),
			wantPairs:   [][2]string{{"default_num_partitions", "num.partitions"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := findConfigCollisions(tt.typedAttrs, tt.genericKeys)
			require.Len(t, got, len(tt.wantPairs))

			gotSet := make(map[[2]string]bool, len(got))
			for _, c := range got {
				gotSet[[2]string{c.TypedAttr, c.GenericKey}] = true
			}
			for _, want := range tt.wantPairs {
				require.True(t, gotSet[want], "expected collision %v", want)
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

	declared := mapOf(t, map[string]string{
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

func TestTypedAttrOverrides(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		planBroker map[string]string
		want       map[string]string
	}{
		{
			name:       "typed-backed key pins its typed attr to the map value",
			planBroker: map[string]string{"log.retention.ms": "3600000"},
			want:       map[string]string{"default_retention_millis": "3600000"},
		},
		{
			name:       "key without a typed equivalent produces no override",
			planBroker: map[string]string{"message.max.bytes": "1048576"},
			want:       map[string]string{},
		},
		{
			name: "multiple typed-backed keys each pin their typed attr",
			planBroker: map[string]string{
				"num.partitions":                "16",
				"warpstream.default.topic.type": "lightning",
				"message.max.bytes":             "1048576",
			},
			want: map[string]string{
				"default_num_partitions": "16",
				"default_topic_type":     "lightning",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := typedAttrOverrides(tt.planBroker)
			require.Equal(t, tt.want, got)
		})
	}
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
