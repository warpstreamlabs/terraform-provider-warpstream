package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

type ACL struct {
	ID               string `json:"id"`
	VirtualClusterID string `json:"virtual_cluster_id"`
	Host             string `json:"host"`
	Principal        string `json:"principal"`
	Operation        string `json:"operation"`
	PermissionType   string `json:"permission_type"`
	ResourceType     string `json:"resource_type"`
	ResourceName     string `json:"resource_name"`
	PatternType      string `json:"pattern_type"`
	CreatedAt        string `json:"created_at"`
}

type ACLRequest struct {
	ResourceType   string `json:"resource_type"`
	ResourceName   string `json:"resource_name"`
	PatternType    string `json:"pattern_type"`
	Principal      string `json:"principal"`
	Host           string `json:"host"`
	Operation      string `json:"operation"`
	PermissionType string `json:"permission_type"`
}

type ACLCreateRequest struct {
	VirtualClusterID string     `json:"virtual_cluster_id"`
	ACL              ACLRequest `json:"acl"`
}

type ACLDescribeResponse struct {
	ACL ACL `json:"acl"`
}

type ACLListRequest struct {
	VirtualClusterID string `json:"virtual_cluster_id"`
}

type ACLListResponse struct {
	ACLs []ACL `json:"acls"`
}

type ACLDeleteRequest struct {
	VirtualClusterID string       `json:"virtual_cluster_id"`
	ACLs             []ACLRequest `json:"acls"`
}

type ACLDeleteResponse struct {
	ACLs []ACL `json:"acls"`
}

// CreateACL creates a new ACL in the specified virtual cluster.
func (c *Client) CreateACL(vcID string, acl ACLRequest) (*ACL, error) {
	payload, err := json.Marshal(ACLCreateRequest{VirtualClusterID: vcID, ACL: acl})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/acls/create", c.HostURL), bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	body, err := c.doRequest(req, nil)
	if err != nil {
		return nil, err
	}

	var aclCreateResp ACLDescribeResponse
	if err := json.Unmarshal(body, &aclCreateResp); err != nil {
		return nil, err
	}

	return &aclCreateResp.ACL, nil
}

// GetACL retrieves a specific ACL by its ID within the specified virtual cluster.
func (c *Client) GetACL(vcID, aclID string) (*ACL, error) {
	acls, err := c.ListACLs(vcID)
	if err != nil {
		return nil, fmt.Errorf("failed to get ACLs list: %w", err)
	}

	for _, acl := range acls {
		if acl.ID == aclID {
			return &acl, nil
		}
	}

	return nil, ErrNotFound
}

// ListACLs retrieves all ACLs for a given virtual cluster.
func (c *Client) ListACLs(vcID string) ([]ACL, error) {
	payload, err := json.Marshal(ACLListRequest{VirtualClusterID: vcID})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/acls/list", c.HostURL), bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, nil)
	if err != nil {
		return nil, err
	}

	res := ACLListResponse{}
	err = json.Unmarshal(body, &res)
	if err != nil {
		return nil, err
	}

	return res.ACLs, nil
}

// DeleteACL deletes an ACL by its ID within the specified virtual cluster.
func (c *Client) DeleteACL(vcID string, acl ACLRequest) error {
	payload, err := json.Marshal(ACLDeleteRequest{VirtualClusterID: vcID, ACLs: []ACLRequest{acl}})
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/virtual_clusters/%s/acls/delete", c.HostURL, vcID), bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	body, err := c.doRequest(req, nil)
	if err != nil {
		return err
	}

	if string(body) != "{}" {
		return errors.New(string(body))
	}

	var deleteResp ACLDeleteResponse
	err = json.Unmarshal(body, &deleteResp)
	if err != nil {
		return err
	}

	// assert that one ACL was deleted
	if len(deleteResp.ACLs) != 1 {
		return fmt.Errorf("expected 1 ACL to be deleted, got %d", len(deleteResp.ACLs))
	}

	return nil
}
