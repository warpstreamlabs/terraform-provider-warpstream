package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/diag"
)

const (
	PrincipalKindAny                      = "*"
	PrincipalKindAgent                    = "agent"
	PrincipalKindAgentReadOnly            = "agent_r"
	PrincipalKindApplication              = "app"
	PrincipalKindApplicationReadOnly      = "app_r"
	ResourceKindVirtualCluster            = "virtual_cluster"
	ResourceKindVirtualClusterTopics      = "virtual_cluster_topics"
	ResourceKindVirtualClusterCredentials = "virtual_cluster_credentials"
	ResourceKindVirtualClusterACLs        = "virtual_cluster_acls"
	ResourceKindAny                       = "*"
	ResourceIDAny                         = "*"
	WorkspaceIDAny                        = "*"
)

// VirtualClusterSubResourceKinds are the resource kinds used for cluster-scoped application keys.
var VirtualClusterSubResourceKinds = []string{
	ResourceKindVirtualClusterTopics,
	ResourceKindVirtualClusterCredentials,
	ResourceKindVirtualClusterACLs,
}

// IsVirtualClusterSubResourceKind reports whether kind is a cluster-scoped application key resource kind.
func IsVirtualClusterSubResourceKind(kind string) bool {
	return slices.Contains(VirtualClusterSubResourceKinds, kind)
}

type AccessGrants []AccessGrant

// ReadWorkspaceIDSafe returns the workspace ID of the first access grant if the slice isn't empty.
// This is useful for application keys, which are restricted to a single workspace.
// Access grants on a role can point to more than one workspace.
func (a AccessGrants) ReadWorkspaceIDSafe() string {
	if len(a) == 0 {
		return ""
	}

	return a[0].WorkspaceID
}

type APIKey struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	Key          string       `json:"key"`
	AccessGrants AccessGrants `json:"access_grants"`
	CreatedAt    string       `json:"created_at"`
}

func (a APIKey) GetVirtualClusterID(diags *diag.Diagnostics) (string, bool) {
	if len(a.AccessGrants) == 0 {
		diags.AddError(
			"Error Reading WarpStream Agent Key",
			"API returned invalid Agent Key with ID "+a.ID+": no access grants found",
		)
		return "", false
	}

	return a.AccessGrants[0].ResourceID, true
}

func (a APIKey) IsReadOnly() bool {
	if len(a.AccessGrants) == 0 {
		return false
	}

	principalKind := a.AccessGrants[0].PrincipalKind
	return principalKind == PrincipalKindAgentReadOnly || principalKind == PrincipalKindApplicationReadOnly
}

// ApplicationKeyClusterScope returns the virtual cluster ID and resource kind from the
// first cluster-scoped application grant. Otherwise ok is false.
func (a APIKey) ApplicationKeyClusterScope() (virtualClusterID, resourceKind string, ok bool) {
	for _, grant := range a.AccessGrants {
		if grant.PrincipalKind != PrincipalKindApplication {
			continue
		}
		if !IsVirtualClusterSubResourceKind(grant.ResourceKind) {
			continue
		}

		return grant.ResourceID, grant.ResourceKind, true
	}

	return "", "", false
}

type APIKeyListResponse struct {
	APIKeys []APIKey `json:"api_keys"`
}

type APIKeyCreateRequest struct {
	Name         string              `json:"name"` // No `akn_` prefix.
	AccessGrants []map[string]string `json:"access_grants"`
	// Optional. Defaults to `byoc` if left empty.
	VirtualClusterTypeOverride string `json:"virtual_cluster_type"`
}

type APIKeyDeleteRequest struct {
	ID string `json:"api_key_id"`
}

// CreateAgentKey - Create new Agent Key. Supports creating keys with just one access grant for now.
func (c *Client) CreateAgentKey(name, virtualClusterID string, readOnly bool) (*APIKey, error) {
	virtualClusterTypeOverride := virtualClusterTypeForID(virtualClusterID)

	principalKind := PrincipalKindAgent
	if readOnly {
		principalKind = PrincipalKindAgentReadOnly
	}

	accessGrant := map[string]string{
		"principal_kind": principalKind,
		"resource_kind":  ResourceKindVirtualCluster,
		"resource_id":    virtualClusterID,
	}

	return c.createAPIKey(name, accessGrant, virtualClusterTypeOverride, c.HashedAPIKeys)
}

// virtualClusterTypeForID returns the type override an agent key needs for the cluster, empty for BYOC.
func virtualClusterTypeForID(virtualClusterID string) string {
	if strings.HasPrefix(virtualClusterID, "vci_sr_") {
		return VirtualClusterTypeSchemaRegistry
	} else if strings.HasPrefix(virtualClusterID, "vci_dl_") {
		return VirtualClusterTypeTableFlow
	}
	return ""
}

func (c *Client) CreateApplicationKey(name, workspaceID string, readOnly bool) (*APIKey, error) {
	principalKind := PrincipalKindApplication
	if readOnly {
		principalKind = PrincipalKindApplicationReadOnly
	}

	accessGrant := map[string]string{
		"principal_kind": principalKind,
		"resource_kind":  ResourceKindAny,
		"resource_id":    ResourceIDAny,
		"workspace_id":   workspaceID, // Can be empty.
	}

	return c.createAPIKey(name, accessGrant, "", c.HashedAPIKeys)
}

// CreateHashedAgentKey creates a hashed agent key for the cluster regardless of HashedAPIKeys.
func (c *Client) CreateHashedAgentKey(name, virtualClusterID string) (*APIKey, error) {
	accessGrant := map[string]string{
		"principal_kind": PrincipalKindAgent,
		"resource_kind":  ResourceKindVirtualCluster,
		"resource_id":    virtualClusterID,
	}
	return c.createAPIKey(name, accessGrant, virtualClusterTypeForID(virtualClusterID), true)
}

// CreateHashedApplicationKey creates a hashed application key for the workspace regardless of HashedAPIKeys.
func (c *Client) CreateHashedApplicationKey(name, workspaceID string) (*APIKey, error) {
	accessGrant := map[string]string{
		"principal_kind": PrincipalKindApplication,
		"resource_kind":  ResourceKindAny,
		"resource_id":    ResourceIDAny,
		"workspace_id":   workspaceID,
	}
	return c.createAPIKey(name, accessGrant, "", true)
}

// CreateClusterScopedApplicationKey creates an application key scoped to one
// virtual cluster sub-resource: topics, credentials, or ACLs.
func (c *Client) CreateClusterScopedApplicationKey(name, workspaceID, virtualClusterID, resourceKind string) (*APIKey, error) {
	if !IsVirtualClusterSubResourceKind(resourceKind) {
		return nil, fmt.Errorf("unsupported resource_kind %q for cluster-scoped application key", resourceKind)
	}

	accessGrant := map[string]string{
		"principal_kind": PrincipalKindApplication,
		"resource_kind":  resourceKind,
		"resource_id":    virtualClusterID,
		"workspace_id":   workspaceID, // Can be empty.
	}

	return c.createAPIKey(name, accessGrant, "", c.HashedAPIKeys)
}

func (c *Client) createAPIKey(
	name string,
	accessGrant map[string]string,
	virtualClusterTypeOverride string,
	hashed bool,
) (*APIKey, error) {
	payload, err := json.Marshal(APIKeyCreateRequest{
		Name:                       strings.TrimPrefix(name, "akn_"),
		AccessGrants:               []map[string]string{accessGrant},
		VirtualClusterTypeOverride: virtualClusterTypeOverride,
	})
	if err != nil {
		return nil, err
	}

	if hashed {
		return c.createHashedAPIKey(payload, "akn_"+strings.TrimPrefix(name, "akn_"), accessGrant)
	}

	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/create_api_key", c.HostURL), bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, nil)
	if err != nil {
		return nil, err
	}

	res := APIKey{}
	err = json.Unmarshal(body, &res)
	if err != nil {
		return nil, err
	}

	return &res, nil
}

// createHashedAPIKey calls create_api_key_v2, whose secret is only in the create response. After an
// ambiguous failure, a key with this name and grant created since the attempt started is ours.
func (c *Client) createHashedAPIKey(payload []byte, name string, accessGrant map[string]string) (*APIKey, error) {
	return createWithRecovery(
		"API key "+name,
		func() (*APIKey, error) { return c.createHashedAPIKeyOnce(payload) },
		func(startedAt time.Time) error { return c.deleteOrphanedAPIKey(name, accessGrant, startedAt) },
		fmt.Sprintf("delete any key named %s with list_api_keys and delete_api_key", name),
	)
}

func (c *Client) createHashedAPIKeyOnce(payload []byte) (*APIKey, error) {
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/create_api_key_v2", c.HostURL), bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}

	body, err := c.doRequestOnce(req)
	if err != nil {
		return nil, err
	}

	res := APIKey{}
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, fmt.Errorf("%w: invalid create response: %w", ErrAmbiguous, err)
	}
	return &res, nil
}

// deleteOrphanedAPIKey deletes the key a failed create attempt left behind, if any. Matching the
// unique name, the grant and the creation time keeps it from deleting a key that existed before.
func (c *Client) deleteOrphanedAPIKey(name string, accessGrant map[string]string, startedAt time.Time) error {
	keys, err := c.GetAPIKeys()
	if err != nil {
		return err
	}

	for _, key := range keys {
		if key.Name != name {
			continue
		}
		if !hasAccessGrant(key, accessGrant) {
			return fmt.Errorf("key %s (%s) exists with different access grants", name, key.ID)
		}
		if err := requireCreatedSince("key", name, key.ID, key.CreatedAt, startedAt); err != nil {
			return err
		}

		log.Printf("deleting API key %s (%s) left behind by the failed create", name, key.ID)
		if err := c.DeleteAPIKey(key.ID); err != nil && !errors.Is(err, ErrNotFound) {
			return err
		}
		return nil
	}
	return nil
}

func hasAccessGrant(key APIKey, accessGrant map[string]string) bool {
	if len(key.AccessGrants) != 1 {
		return false
	}
	grant := key.AccessGrants[0]
	return grant.PrincipalKind == accessGrant["principal_kind"] &&
		grant.ResourceKind == accessGrant["resource_kind"] &&
		grant.ResourceID == accessGrant["resource_id"] &&
		(accessGrant["workspace_id"] == "" || grant.WorkspaceID == accessGrant["workspace_id"])
}

// DeleteAPIKey - Delete an API Key.
func (c *Client) DeleteAPIKey(id string) error {
	payload, err := json.Marshal(APIKeyDeleteRequest{ID: id})
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/delete_api_key", c.HostURL), bytes.NewReader(payload))
	if err != nil {
		return err
	}

	body, err := c.doRequest(req, nil)
	if err != nil {
		return err
	}

	if string(body) != "{}" {
		return errors.New(string(body))
	}

	return nil
}

// GetAPIKeys - Returns list of API keys.
func (c *Client) GetAPIKeys() ([]APIKey, error) {
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/list_api_keys", c.HostURL), nil)
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, nil)
	if err != nil {
		return nil, err
	}

	res := APIKeyListResponse{}
	err = json.Unmarshal(body, &res)
	if err != nil {
		return nil, err
	}

	return res.APIKeys, nil
}

// GetAPIKey - Returns one API key.
func (c *Client) GetAPIKey(apiKeyID string) (*APIKey, error) {
	keys, err := c.GetAPIKeys()

	if err != nil {
		return nil, fmt.Errorf("failed to get API keys list: %w", err)
	}

	for _, key := range keys {
		if key.ID == apiKeyID {
			return &key, nil
		}
	}

	return nil, ErrNotFound
}
