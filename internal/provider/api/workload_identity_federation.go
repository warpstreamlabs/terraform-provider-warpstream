package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

// ClaimMatch asserts that a claim in the workload's OIDC token (e.g. "sub") equals an expected value.
type ClaimMatch struct {
	ClaimPath     string `json:"claim_path"`
	ExpectedValue string `json:"expected_value"`
}

// WorkloadIdentityFederation is a per-virtual-cluster binding that lets a workload authenticate to the
// control plane with an OIDC token from an external issuer instead of a long-lived agent key.
type WorkloadIdentityFederation struct {
	ID                      string       `json:"id"`
	VirtualClusterID        string       `json:"virtual_cluster_id"`
	Name                    string       `json:"name"`
	IssuerURL               string       `json:"issuer_url"`
	Audience                string       `json:"audience"`
	ClaimMatchRules         []ClaimMatch `json:"claim_match_rules"`
	ReadOnly                bool         `json:"read_only"`
	MaxCredentialTTLSeconds int64        `json:"max_credential_ttl_seconds"`
	CreatedAt               string       `json:"created_at"`
}

type createWorkloadIdentityFederationRequest struct {
	VirtualClusterID        string       `json:"virtual_cluster_id"`
	Name                    string       `json:"name"`
	IssuerURL               string       `json:"issuer_url"`
	ClaimMatchRules         []ClaimMatch `json:"claim_match_rules"`
	ReadOnly                bool         `json:"read_only"`
	MaxCredentialTTLSeconds int64        `json:"max_credential_ttl_seconds"`
}

type listWorkloadIdentityFederationsRequest struct {
	VirtualClusterID string `json:"virtual_cluster_id"`
}

type listWorkloadIdentityFederationsResponse struct {
	WorkloadIdentityFederations []WorkloadIdentityFederation `json:"workload_identity_federations"`
}

type deleteWorkloadIdentityFederationRequest struct {
	VirtualClusterID string `json:"virtual_cluster_id"`
	ID               string `json:"id"`
}

// CreateWorkloadIdentityFederation creates a federation binding and returns the created resource,
// including the server-derived audience and creation timestamp.
func (c *Client) CreateWorkloadIdentityFederation(fed WorkloadIdentityFederation) (*WorkloadIdentityFederation, error) {
	payload, err := json.Marshal(createWorkloadIdentityFederationRequest{
		VirtualClusterID:        fed.VirtualClusterID,
		Name:                    fed.Name,
		IssuerURL:               fed.IssuerURL,
		ClaimMatchRules:         fed.ClaimMatchRules,
		ReadOnly:                fed.ReadOnly,
		MaxCredentialTTLSeconds: fed.MaxCredentialTTLSeconds,
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/create_workload_identity_federation", c.HostURL), bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, nil)
	if err != nil {
		return nil, err
	}

	res := WorkloadIdentityFederation{}
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

// ListWorkloadIdentityFederations returns all federation bindings for a virtual cluster.
func (c *Client) ListWorkloadIdentityFederations(virtualClusterID string) ([]WorkloadIdentityFederation, error) {
	payload, err := json.Marshal(listWorkloadIdentityFederationsRequest{VirtualClusterID: virtualClusterID})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/list_workload_identity_federations", c.HostURL), bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, nil)
	if err != nil {
		return nil, err
	}

	res := listWorkloadIdentityFederationsResponse{}
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, err
	}
	return res.WorkloadIdentityFederations, nil
}

// GetWorkloadIdentityFederation returns a single binding by ID, or ErrNotFound. There is no
// get-by-id endpoint, so it lists the cluster's bindings and filters (as GetAPIKey does).
func (c *Client) GetWorkloadIdentityFederation(virtualClusterID, id string) (*WorkloadIdentityFederation, error) {
	feds, err := c.ListWorkloadIdentityFederations(virtualClusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to list workload identity federations: %w", err)
	}
	for _, fed := range feds {
		if fed.ID == id {
			return &fed, nil
		}
	}
	return nil, ErrNotFound
}

// DeleteWorkloadIdentityFederation deletes a binding by ID within its virtual cluster.
func (c *Client) DeleteWorkloadIdentityFederation(virtualClusterID, id string) error {
	payload, err := json.Marshal(deleteWorkloadIdentityFederationRequest{VirtualClusterID: virtualClusterID, ID: id})
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/delete_workload_identity_federation", c.HostURL), bytes.NewReader(payload))
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
