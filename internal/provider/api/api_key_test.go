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
			name: "read-only application key",
			apiKey: APIKey{
				ID:   "key_4",
				Name: "aks_app_readonly",
				AccessGrants: AccessGrants{
					{
						PrincipalKind: PrincipalKindApplicationReadOnly,
						ResourceKind:  ResourceKindAny,
						ResourceID:    ResourceIDAny,
					},
				},
			},
			expected: true,
		},
		{
			name: "empty access grants",
			apiKey: APIKey{
				ID:           "key_5",
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

func TestAPIKey_ApplicationKeyClusterScope(t *testing.T) {
	tests := []struct {
		name                 string
		apiKey               APIKey
		wantVirtualClusterID string
		wantResourceKind     string
		wantOK               bool
	}{
		{
			name: "empty access grants",
			apiKey: APIKey{
				ID:           "key_1",
				AccessGrants: AccessGrants{},
			},
			wantOK: false,
		},
		{
			name: "workspace-scoped application key",
			apiKey: APIKey{
				ID: "key_2",
				AccessGrants: AccessGrants{
					{
						PrincipalKind: PrincipalKindApplication,
						ResourceKind:  ResourceKindAny,
						ResourceID:    ResourceIDAny,
						WorkspaceID:   "wi_test",
					},
				},
			},
			wantOK: false,
		},
		{
			name: "cluster-scoped topics",
			apiKey: APIKey{
				ID: "key_3",
				AccessGrants: AccessGrants{
					{
						PrincipalKind: PrincipalKindApplication,
						ResourceKind:  ResourceKindVirtualClusterTopics,
						ResourceID:    "vci_topics",
						WorkspaceID:   "wi_test",
					},
				},
			},
			wantVirtualClusterID: "vci_topics",
			wantResourceKind:     ResourceKindVirtualClusterTopics,
			wantOK:               true,
		},
		{
			name: "cluster-scoped credentials",
			apiKey: APIKey{
				ID: "key_4",
				AccessGrants: AccessGrants{
					{
						PrincipalKind: PrincipalKindApplication,
						ResourceKind:  ResourceKindVirtualClusterCredentials,
						ResourceID:    "vci_creds",
						WorkspaceID:   "wi_test",
					},
				},
			},
			wantVirtualClusterID: "vci_creds",
			wantResourceKind:     ResourceKindVirtualClusterCredentials,
			wantOK:               true,
		},
		{
			name: "cluster-scoped acls",
			apiKey: APIKey{
				ID: "key_5",
				AccessGrants: AccessGrants{
					{
						PrincipalKind: PrincipalKindApplication,
						ResourceKind:  ResourceKindVirtualClusterACLs,
						ResourceID:    "vci_acls",
						WorkspaceID:   "wi_test",
					},
				},
			},
			wantVirtualClusterID: "vci_acls",
			wantResourceKind:     ResourceKindVirtualClusterACLs,
			wantOK:               true,
		},
		{
			name: "agent-style virtual_cluster grant",
			apiKey: APIKey{
				ID: "key_6",
				AccessGrants: AccessGrants{
					{
						PrincipalKind: PrincipalKindAgent,
						ResourceKind:  ResourceKindVirtualCluster,
						ResourceID:    "vci_agent",
						WorkspaceID:   "wi_test",
					},
				},
			},
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotVCID, gotKind, gotOK := tt.apiKey.ApplicationKeyClusterScope()
			if gotOK != tt.wantOK {
				t.Errorf("ApplicationKeyClusterScope() ok = %v, want %v", gotOK, tt.wantOK)
			}
			if gotVCID != tt.wantVirtualClusterID {
				t.Errorf("ApplicationKeyClusterScope() virtualClusterID = %q, want %q", gotVCID, tt.wantVirtualClusterID)
			}
			if gotKind != tt.wantResourceKind {
				t.Errorf("ApplicationKeyClusterScope() resourceKind = %q, want %q", gotKind, tt.wantResourceKind)
			}
		})
	}
}
