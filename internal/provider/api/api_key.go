package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
)

const (
	PrincipalKindAny           = "*"
	PrincipalKindAgent         = "agent"
	PrincipalKindApplication   = "app"
	ResourceKindVirtualCluster = "virtual_cluster"
	ResourceKindAny            = "*"
	ResourceIDAny              = "*"
	WorkspaceIDAny             = "*"
)

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
func (c *Client) CreateAgentKey(name, virtualClusterID string) (*APIKey, error) {
	virtualClusterTypeOverride := ""
	if strings.HasPrefix(virtualClusterID, "vci_sr_") {
		virtualClusterTypeOverride = VirtualClusterTypeSchemaRegistry
	}
	accessGrant := map[string]string{
		"principal_kind": PrincipalKindAgent,
		"resource_kind":  ResourceKindVirtualCluster,
		"resource_id":    virtualClusterID,
	}

	return c.createAPIKey(name, accessGrant, virtualClusterTypeOverride)
}

func (c *Client) CreateApplicationKey(name, workspaceID string) (*APIKey, error) {
	accessGrant := map[string]string{
		"principal_kind": PrincipalKindApplication,
		"resource_kind":  ResourceKindAny,
		"resource_id":    ResourceIDAny,
		"workspace_id":   workspaceID, // Can be empty.
	}

	return c.createAPIKey(name, accessGrant, "")
}

func (c *Client) createAPIKey(
	name string,
	accessGrant map[string]string,
	virtualClusterTypeOverride string,
) (*APIKey, error) {
	payload, err := json.Marshal(APIKeyCreateRequest{
		Name:                       strings.TrimPrefix(name, "akn_"),
		AccessGrants:               []map[string]string{accessGrant},
		VirtualClusterTypeOverride: virtualClusterTypeOverride,
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/create_api_key", c.HostURL), bytes.NewReader(payload))
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

// DeleteAPIKey - Delete an API Key.
func (c *Client) DeleteAPIKey(id string) error {
	payload, err := json.Marshal(APIKeyDeleteRequest{ID: id})
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/delete_api_key", c.HostURL), bytes.NewReader(payload))
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
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/list_api_keys", c.HostURL), nil)
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
		return nil, fmt.Errorf("Failed to get API keys list: %w", err)
	}

	for _, key := range keys {
		if key.ID == apiKeyID {
			return &key, nil
		}
	}

	return nil, ErrNotFound
}
