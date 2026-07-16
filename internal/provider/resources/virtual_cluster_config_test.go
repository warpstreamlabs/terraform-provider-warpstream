package resources

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/require"
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

func TestTypedAttrsToInvalidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		planBroker  map[string]string
		stateBroker map[string]string
		want        []string
	}{
		{
			name:       "added typed-backed key invalidates its typed attr",
			planBroker: map[string]string{"log.retention.ms": "3600000"},
			want:       []string{"default_retention_millis"},
		},
		{
			name:        "unchanged value does not invalidate",
			planBroker:  map[string]string{"log.retention.ms": "3600000"},
			stateBroker: map[string]string{"log.retention.ms": "3600000"},
			want:        nil,
		},
		{
			name:        "changed value invalidates",
			planBroker:  map[string]string{"num.partitions": "16"},
			stateBroker: map[string]string{"num.partitions": "8"},
			want:        []string{"default_num_partitions"},
		},
		{
			name:       "key without a typed equivalent does not invalidate",
			planBroker: map[string]string{"message.max.bytes": "1048576"},
			want:       nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := typedAttrsToInvalidate(tt.planBroker, tt.stateBroker)
			require.Len(t, got, len(tt.want))
			for _, w := range tt.want {
				_, ok := got[w]
				require.True(t, ok, "expected %s to be invalidated", w)
			}
		})
	}
}
