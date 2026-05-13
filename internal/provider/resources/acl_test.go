package resources

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseACLImportID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		id   string
		want aclImportID
	}{
		{
			name: "standard principal",
			id:   "vci_123/TOPIC/orders/LITERAL/User:alice/*/READ/ALLOW",
			want: aclImportID{
				virtualClusterID: "vci_123",
				resourceType:     "TOPIC",
				resourceName:     "orders",
				patternType:      "LITERAL",
				principal:        "User:alice",
				host:             "*",
				operation:        "READ",
				permissionType:   "ALLOW",
			},
		},
		{
			name: "principal containing slashes",
			id:   "vci_123/GROUP/consumer-group-/PREFIXED/User:spiffe://example.test/ns/default/sa/service-account/*/READ/ALLOW",
			want: aclImportID{
				virtualClusterID: "vci_123",
				resourceType:     "GROUP",
				resourceName:     "consumer-group-",
				patternType:      "PREFIXED",
				principal:        "User:spiffe://example.test/ns/default/sa/service-account",
				host:             "*",
				operation:        "READ",
				permissionType:   "ALLOW",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := parseACLImportID(tt.id)
			require.True(t, ok)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestParseACLImportIDInvalid(t *testing.T) {
	t.Parallel()

	_, ok := parseACLImportID("vci_123/TOPIC/orders/LITERAL/User:alice")
	require.False(t, ok)
}
