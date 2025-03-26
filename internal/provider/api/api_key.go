package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
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

type APIKeyListResponse struct {
	APIKeys []APIKey `json:"api_keys"`
}

type APIKeyCreateRequest struct {
	Name         string              `json:"name"` // No `akn_` prefix.
	AccessGrants []map[string]string `json:"access_grants"`
}

type APIKeyDeleteRequest struct {
	ID string `json:"api_key_id"`
}

// CreateAgentKey - Create new Agent Key. Supports creating keys with just one access grant for now.
func (c *Client) CreateAgentKey(name, virtualClusterID string) (*APIKey, error) {
	accessGrant := map[string]string{
		"principal_kind": PrincipalKindAgent,
		"resource_kind":  ResourceKindVirtualCluster,
		"resource_id":    virtualClusterID,
	}

	return c.createAPIKey(name, accessGrant)
}

func (c *Client) CreateApplicationKey(name, workspaceID string) (*APIKey, error) {
	accessGrant := map[string]string{
		"principal_kind": PrincipalKindApplication,
		"resource_kind":  ResourceKindAny,
		"resource_id":    ResourceIDAny,
		"workspace_id":   workspaceID, // Can be empty.
	}

	return c.createAPIKey(name, accessGrant)
}

func (c *Client) createAPIKey(
	name string,
	accessGrant map[string]string,
) (*APIKey, error) {
	payload, err := json.Marshal(APIKeyCreateRequest{
		Name:         strings.TrimPrefix(name, "akn_"),
		AccessGrants: []map[string]string{accessGrant},
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
