package api

import (
	"testing"
)

func TestAPIKey_IsReadOnly(t *testing.T) {
	tests := []struct {
		name     string
		apiKey   APIKey
		expected bool
	}{
		{
			name: "regular agent key",
			apiKey: APIKey{
				ID:   "key_1",
				Name: "akn_regular",
				AccessGrants: AccessGrants{
					{
						PrincipalKind: PrincipalKindAgent,
						ResourceKind:  ResourceKindVirtualCluster,
						ResourceID:    "vci_test",
					},
				},
			},
			expected: false,
		},
		{
			name: "read-only agent key",
			apiKey: APIKey{
				ID:   "key_2",
				Name: "akn_readonly",
				AccessGrants: AccessGrants{
					{
						PrincipalKind: PrincipalKindAgentReadOnly,
						ResourceKind:  ResourceKindVirtualCluster,
						ResourceID:    "vci_test",
					},
				},
			},
			expected: true,
		},
		{
			name: "application key",
			apiKey: APIKey{
				ID:   "key_3",
				Name: "aks_app",
				AccessGrants: AccessGrants{
					{
						PrincipalKind: PrincipalKindApplication,
						ResourceKind:  ResourceKindAny,
						ResourceID:    ResourceIDAny,
					},
				},
			},
			expected: false,
		},
		{
			name: "empty access grants",
			apiKey: APIKey{
				ID:           "key_4",
				Name:         "akn_empty",
				AccessGrants: AccessGrants{},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.apiKey.IsReadOnly()
			if result != tt.expected {
				t.Errorf("IsReadOnly() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

