package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

// CreateAgentKeyV2 creates a v2 agent key for the cluster through create_api_key_v2. Its secret
// is only in the response.
func (c *Client) CreateAgentKeyV2(name, virtualClusterID string, readOnly bool) (*APIKey, error) {
	typeOverride := ""
	if strings.HasPrefix(virtualClusterID, "vci_sr_") {
		typeOverride = VirtualClusterTypeSchemaRegistry
	} else if strings.HasPrefix(virtualClusterID, "vci_dl_") {
		typeOverride = VirtualClusterTypeTableFlow
	}

	principalKind := PrincipalKindAgent
	if readOnly {
		principalKind = PrincipalKindAgentReadOnly
	}
	return c.createAPIKeyV2(name, map[string]string{
		"principal_kind": principalKind,
		"resource_kind":  ResourceKindVirtualCluster,
		"resource_id":    virtualClusterID,
	}, typeOverride)
}

// CreateApplicationKeyV2 creates a v2 application key for the workspace through
// create_api_key_v2. Its secret is only in the response.
func (c *Client) CreateApplicationKeyV2(name, workspaceID string, readOnly bool) (*APIKey, error) {
	principalKind := PrincipalKindApplication
	if readOnly {
		principalKind = PrincipalKindApplicationReadOnly
	}
	return c.createAPIKeyV2(name, map[string]string{
		"principal_kind": principalKind,
		"resource_kind":  ResourceKindAny,
		"resource_id":    ResourceIDAny,
		"workspace_id":   workspaceID,
	}, "")
}

// CreateClusterScopedApplicationKeyV2 creates a v2 application key scoped to one virtual cluster
// sub-resource (topics, credentials, or ACLs) through create_api_key_v2. Its secret is only in the response.
func (c *Client) CreateClusterScopedApplicationKeyV2(name, workspaceID, virtualClusterID, resourceKind string) (*APIKey, error) {
	if !IsVirtualClusterSubResourceKind(resourceKind) {
		return nil, fmt.Errorf("unsupported resource_kind %q for cluster-scoped application key", resourceKind)
	}
	return c.createAPIKeyV2(name, map[string]string{
		"principal_kind": PrincipalKindApplication,
		"resource_kind":  resourceKind,
		"resource_id":    virtualClusterID,
		"workspace_id":   workspaceID,
	}, "")
}

// createAPIKeyV2 calls create_api_key_v2. After an ambiguous failure, a key with this name and grant
// created since the attempt started is ours.
func (c *Client) createAPIKeyV2(name string, accessGrant map[string]string, typeOverride string) (*APIKey, error) {
	name = "akn_" + strings.TrimPrefix(name, "akn_")
	payload, err := json.Marshal(APIKeyCreateRequest{
		Name:                       strings.TrimPrefix(name, "akn_"),
		AccessGrants:               []map[string]string{accessGrant},
		VirtualClusterTypeOverride: typeOverride,
	})
	if err != nil {
		return nil, err
	}

	return createWithRecovery(
		"API key "+name,
		func() (*APIKey, error) {
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
		},
		func(startedAt time.Time) error { return c.deleteOrphanedAPIKey(name, accessGrant, startedAt) },
		fmt.Sprintf("delete any key named %s with list_api_keys and delete_api_key", name),
	)
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
