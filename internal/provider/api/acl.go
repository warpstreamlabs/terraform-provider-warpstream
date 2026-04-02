package api

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
)

type ACLRequest struct {
	ResourceType   string `json:"resource_type"`
	ResourceName   string `json:"resource_name"`
	PatternType    string `json:"pattern_type"`
	Principal      string `json:"principal"`
	Host           string `json:"host"`
	Operation      string `json:"operation"`
	PermissionType string `json:"permission_type"`
}

type ACLResponse struct {
	ResourceType   string `json:"resource_type"`
	ResourceName   string `json:"resource_name"`
	PatternType    string `json:"pattern_type"`
	Principal      string `json:"principal"`
	Host           string `json:"host"`
	Operation      string `json:"operation"`
	PermissionType string `json:"permission_type"`
}

// ID generates a unique identifier for the ACL based on its fields.
// Note that the ID changes any time a field is changed, which results in Terraform planning to recreate the resource-
// this is acceptable for our use case since ACLs are immutable and any change requires deletion and recreation.
func (a *ACLResponse) ID() string {
	rawID := a.ResourceType + "|" +
		a.ResourceName + "|" +
		a.PatternType + "|" +
		a.Principal + "|" +
		a.Host + "|" +
		a.Operation + "|" +
		a.PermissionType

	hash := sha256.Sum256([]byte(rawID))
	return hex.EncodeToString(hash[:])
}

type ACLCreateRequest struct {
	VirtualClusterID string     `json:"virtual_cluster_id"`
	ACL              ACLRequest `json:"acl"`
}

type ACLDescribeResponse struct {
	ACL ACLResponse `json:"acl"`
}

type ACLListRequest struct {
	VirtualClusterID string `json:"virtual_cluster_id"`
}

type ACLListResponse struct {
	ACLs []ACLResponse `json:"acls"`
}

type ACLDeleteRequest struct {
	VirtualClusterID string       `json:"virtual_cluster_id"`
	ACLs             []ACLRequest `json:"acls"`
}

type ACLDeleteResponse struct {
	ACLs []ACLResponse `json:"acls"`
}

// CreateACL creates a new ACL in the specified virtual cluster.
func (c *Client) CreateACL(vcID string, acl ACLRequest) (*ACLResponse, error) {
	payload, err := json.Marshal(ACLCreateRequest{VirtualClusterID: vcID, ACL: acl})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/virtual_clusters/acls/create", c.HostURL), bytes.NewReader(payload))
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

	c.aclsCache.invalidate(vcID)

	return &aclCreateResp.ACL, nil
}

// GetACL retrieves a specific ACL by its ID within the specified virtual cluster.
func (c *Client) GetACL(vcID string, targetACL ACLRequest) (*ACLResponse, error) {
	acls, err := c.ListACLs(vcID)
	if err != nil {
		return nil, fmt.Errorf("failed to list ACLs: %w", err)
	}

	for _, acl := range acls {
		if aclsEqual(targetACL, acl) {
			return &acl, nil
		}
	}

	return nil, ErrNotFound
}

// ListACLs retrieves all ACLs for a given virtual cluster.
func (c *Client) ListACLs(vcID string) ([]ACLResponse, error) {
	if acls, ok := c.aclsCache.get(vcID); ok {
		return acls, nil
	}

	payload, err := json.Marshal(ACLListRequest{VirtualClusterID: vcID})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/virtual_clusters/acls/list", c.HostURL), bytes.NewReader(payload))
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

	c.aclsCache.set(vcID, res.ACLs)

	return cloneACLResponses(res.ACLs), nil
}

// DeleteACL deletes an ACL by its ID within the specified virtual cluster.
func (c *Client) DeleteACL(vcID string, acl ACLRequest) error {
	payload, err := json.Marshal(ACLDeleteRequest{VirtualClusterID: vcID, ACLs: []ACLRequest{acl}})
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/virtual_clusters/acls/delete", c.HostURL), bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	body, err := c.doRequest(req, nil)
	if err != nil {
		return err
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

	c.aclsCache.invalidate(vcID)

	return nil
}

// aclsEqual returns true if all identifying fields of two ACLs are equal.
func aclsEqual(a ACLRequest, b ACLResponse) bool {
	return a.ResourceType == b.ResourceType &&
		a.ResourceName == b.ResourceName &&
		a.PatternType == b.PatternType &&
		a.Principal == b.Principal &&
		a.Host == b.Host &&
		a.Operation == b.Operation &&
		a.PermissionType == b.PermissionType
}

// Some users have thousands or even tens of thousands of ACLs and when they
// run a terraform plan/apply it would take forever because each ACL lookup
// requires fetching all of the ACLs from the cluster and then filtering them
// client side. We could solve that problem by adding a "get one ACL" API to
// the backend, but that would not reduce the amount of time it takes to do a
// large plan/apply as each ACL lookup would still take ~100ms at minimum.
//
// As a result, we have this "smart cache" that is seeded by a single API call
// to list all the ACLs in the cluster and then we keep it up to date as
// mutations occur
type aclsCache struct {
	mu       sync.Mutex
	aclsByVC map[string][]ACLResponse
}

func (c *aclsCache) get(vcID string) ([]ACLResponse, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.aclsByVC == nil {
		return nil, false
	}

	acls, ok := c.aclsByVC[vcID]
	if !ok {
		return nil, false
	}

	return cloneACLResponses(acls), true
}

func (c *aclsCache) set(vcID string, acls []ACLResponse) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.aclsByVC == nil {
		c.aclsByVC = make(map[string][]ACLResponse)
	}

	c.aclsByVC[vcID] = cloneACLResponses(acls)
}

func (c *aclsCache) invalidate(vcID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.aclsByVC == nil {
		return
	}

	delete(c.aclsByVC, vcID)
}

func cloneACLResponses(acls []ACLResponse) []ACLResponse {
	return append([]ACLResponse(nil), acls...)
}
