package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

type CredentialsListResponse struct {
	Credentials []VirtualClusterCredentials `json:"credentials"`
}

type CredentialsListRequest struct {
	VirtualClusterID string `json:"virtual_cluster_id"`
}

type CredentialsCreateResponse struct {
	ID       string `json:"id"`
	UserName string `json:"username"`
	Password string `json:"password"`
}

type CredentialsCreateRequest struct {
	Name             string `json:"credentials_name"`
	AgentPoolID      string `json:"agent_pool_id"`
	VirtualClusterID string `json:"virtual_cluster_id"`
}

type CredentialsDeleteRequest struct {
	ID               string `json:"id"`
	VirtualClusterID string `json:"virtual_cluster_id"`
}

// CreateCredentials - Create new virtual cluster credentials.
func (c *Client) CreateCredentials(name string, vc VirtualCluster) (*VirtualClusterCredentials, error) {
	payload, err := json.Marshal(CredentialsCreateRequest{
		Name:             strings.TrimPrefix(name, "ccn_"),
		AgentPoolID:      vc.AgentPoolID,
		VirtualClusterID: vc.ID,
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("GET", fmt.Sprintf("%s/create_virtual_cluster_credentials", c.HostURL), bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, nil)
	if err != nil {
		return nil, err
	}

	res := CredentialsCreateResponse{}
	err = json.Unmarshal(body, &res)
	if err != nil {
		return nil, err
	}

	vcc := VirtualClusterCredentials{
		ID:            res.ID,
		Name:          name,
		AgentPoolID:   vc.AgentPoolID,
		AgentPoolName: vc.AgentPoolName,
		UserName:      res.UserName,
		Password:      res.Password,
	}
	return &vcc, nil
}

// DeleteCredentials - Delete virtual cluster credentials.
func (c *Client) DeleteCredentials(id string, vc VirtualCluster) error {
	payload, err := json.Marshal(CredentialsDeleteRequest{ID: id, VirtualClusterID: vc.ID})
	if err != nil {
		return err
	}

	req, err := http.NewRequest("GET", fmt.Sprintf("%s/delete_virtual_cluster_credentials", c.HostURL), bytes.NewReader(payload))
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

// GetCredentials - Returns all virtual clusters credentials of a given Virtual Cluster (indexed by ID).
func (c *Client) GetCredentials(vc VirtualCluster) (map[string]VirtualClusterCredentials, error) {
	payload, err := json.Marshal(CredentialsListRequest{VirtualClusterID: vc.ID})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/list_virtual_cluster_credentials", c.HostURL), bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, nil)
	if err != nil {
		return nil, err
	}

	res := CredentialsListResponse{}
	err = json.Unmarshal(body, &res)
	if err != nil {
		return nil, err
	}

	creds := map[string]VirtualClusterCredentials{}
	for _, c := range res.Credentials {
		creds[c.ID] = c
	}

	return creds, nil
}
