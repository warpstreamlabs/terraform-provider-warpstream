package api

// AccessGrant represents a single access grant as found in API key and user role API responses.
type AccessGrant struct {
	PrincipalKind   string `json:"principal_kind"`
	ResourceKind    string `json:"resource_kind"`
	ResourceID      string `json:"resource_id"`
	WorkspaceID     string `json:"workspace_id"`
	ManagedGrantKey string `json:"managed_grant_name"`
}
