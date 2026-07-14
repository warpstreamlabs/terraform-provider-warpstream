package resources

import (
	"testing"

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

	declared := []models.VirtualClusterConfig{
		{Name: types.StringValue("message.max.bytes"), Value: types.StringValue("ignored")},
		// Declared but not returned by the API -> dropped.
		{Name: types.StringValue("not.returned"), Value: types.StringValue("x")},
	}

	got := filterClusterConfigsToDeclared(apiConfigs, declared)

	require.NotNil(t, got, "result must be non-nil so an absent block reads back as empty set")
	require.Len(t, got, 1)
	require.Equal(t, "message.max.bytes", got[0].Name.ValueString())
	require.Equal(t, "1048576", got[0].Value.ValueString(), "value comes from the API, not the declaration")
}

func TestFilterClusterConfigsToDeclared_EmptyIsNonNil(t *testing.T) {
	t.Parallel()

	got := filterClusterConfigsToDeclared(map[string]*string{"a": nil}, nil)
	require.NotNil(t, got)
	require.Empty(t, got)
}
