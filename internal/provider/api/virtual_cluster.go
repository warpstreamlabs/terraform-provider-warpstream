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
	VirtualClusterTypeBYOC           = "byoc"
	VirtualClusterTypeSchemaRegistry = "byoc_schema_registry"
	VirtualClusterTypeTableFlow      = "byoc_data_lake"

	// legacy is only available for certain tenants, this is controlled on the Warpstream side.
	VirtualClusterTierLegacy       = "legacy"
	VirtualClusterTierDev          = "dev"
	VirtualClusterTierFundamentals = "fundamentals"
	VirtualClusterTierPro          = "pro"
)

type VirtualCluster struct {
	ID            string        `json:"id"`
	Name          string        `json:"name"`
	Type          string        `json:"type"`
	AgentKeys     *[]APIKey     `json:"agent_keys"`
	AgentPoolID   string        `json:"agent_pool_id"`
	AgentPoolName string        `json:"agent_pool_name"`
	CreatedAt     string        `json:"created_at"`
	CloudProvider string        `json:"cloud_provider"`
	ClusterRegion ClusterRegion `json:"cluster_region"`
	BootstrapURL  *string       `json:"bootstrap_url"`
	WorkspaceID   string        `json:"workspace_id"`
}

type ClusterRegion struct {
	IsMultiRegion bool         `json:"is_multi_region"`
	RegionGroup   *RegionGroup `json:"region_group"`
	Region        *Region      `json:"region"`
}

type Region struct {
	Name          string `json:"name"`
	CloudProvider string `json:"cloud_provider"`
}

type RegionGroup struct {
	Name    string   `json:"name"`
	Regions []Region `json:"regions"`
}

type VirtualClusterDescribeResponse struct {
	VirtualCluster VirtualCluster `json:"virtual_cluster"`
}

type VirtualClusterListResponse struct {
	VirtualClusters []VirtualCluster `json:"virtual_clusters"`
}

type VirtualClusterCreateResponse struct {
	VirtualClusterID string  `json:"virtual_cluster_id"`
	AgentPoolID      string  `json:"agent_pool_id"`
	AgentPoolName    string  `json:"agent_pool_name"`
	Name             string  `json:"virtual_cluster_name"`
	BootstrapURL     *string `json:"bootstrap_url"`
	AgentKey         APIKey  `json:"agent_key"`
	WorkspaceID      string  `json:"workspace_id"`
}

type VirtualClusterDescribeRequest struct {
	ID string `json:"virtual_cluster_id"`
}

type VirtualClusterCreateRequest struct {
	Name                 string            `json:"virtual_cluster_name"`
	Type                 string            `json:"virtual_cluster_type,omitempty"`
	Tier                 string            `json:"virtual_cluster_tier,omitempty"`
	Region               *string           `json:"virtual_cluster_region,omitempty"`
	RegionGroup          *string           `json:"virtual_cluster_region_group,omitempty"`
	CloudProvider        string            `json:"virtual_cluster_cloud_provider,omitempty"`
	Tags                 map[string]string `json:"virtual_cluster_tags,omitempty"`
	SkipAgentKeyCreation bool              `json:"skip_agent_key_creation,omitempty"`
}

type VirtualClusterRenameRequest struct {
	ID      string `json:"virtual_cluster_id"`
	NewName string `json:"new_virtual_cluster_name"`
}

type VirtualClusterDeleteRequest struct {
	ID   string `json:"virtual_cluster_id"`
	Name string `json:"virtual_cluster_name"`
}

// GetVirtualCluster - Returns description of virtual cluster.
func (c *Client) GetVirtualCluster(id string) (*VirtualCluster, error) {
	payload, err := json.Marshal(VirtualClusterDescribeRequest{ID: id})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("GET", fmt.Sprintf("%s/describe_virtual_cluster", c.HostURL), bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, nil)
	if err != nil {
		return nil, err
	}

	res := VirtualClusterDescribeResponse{}
	err = json.Unmarshal(body, &res)
	if err != nil {
		return nil, err
	}

	return &res.VirtualCluster, nil
}

type ClusterParameters struct {
	Type           string
	Tier           string
	Region         *string
	RegionGroup    *string
	Cloud          string
	Tags           map[string]string
	CreateAgentKey bool
}

// CreateVirtualCluster - Create new virtual cluster.
func (c *Client) CreateVirtualCluster(name string, opts ClusterParameters) (*VirtualCluster, error) {
	var trimmed string
	if opts.Type == VirtualClusterTypeSchemaRegistry {
		trimmed = strings.TrimPrefix(name, "vcn_sr_")
	} else if opts.Type == VirtualClusterTypeTableFlow {
		trimmed = strings.TrimPrefix(name, "vcn_tf_")
	} else {
		trimmed = strings.TrimPrefix(name, "vcn_")
	}
	payload, err := json.Marshal(VirtualClusterCreateRequest{
		Name:                 trimmed,
		Type:                 opts.Type,
		Tier:                 opts.Tier,
		Region:               opts.Region,
		RegionGroup:          opts.RegionGroup,
		CloudProvider:        opts.Cloud,
		Tags:                 opts.Tags,
		SkipAgentKeyCreation: !opts.CreateAgentKey,
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/create_virtual_cluster", c.HostURL), bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, nil)
	if err != nil {
		return nil, err
	}

	res := VirtualClusterCreateResponse{}
	err = json.Unmarshal(body, &res)
	if err != nil {
		return nil, err
	}

	vc := VirtualCluster{
		ID:            res.VirtualClusterID,
		AgentPoolID:   res.AgentPoolID,
		AgentPoolName: res.AgentPoolName,
		Name:          res.Name,
		BootstrapURL:  res.BootstrapURL,
		AgentKeys:     &[]APIKey{res.AgentKey},
		WorkspaceID:   res.WorkspaceID,
	}
	return &vc, nil
}

func (c *Client) RenameVirtualCluster(id string, newName string) error {
	newNameParts := strings.Split(newName, "vcn_")
	if len(newNameParts) < 2 {
		// Should never happen because of schema-level validation.
		return fmt.Errorf("virtual cluster's new name must start with 'vcn_'")
	}

	payload, err := json.Marshal(VirtualClusterRenameRequest{ID: id, NewName: newNameParts[1]})
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/rename_virtual_cluster", c.HostURL), bytes.NewReader(payload))
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

// DeleteVirtualCluster - Delete a virtual cluster.
func (c *Client) DeleteVirtualCluster(id string, name string) error {
	payload, err := json.Marshal(VirtualClusterDeleteRequest{ID: id, Name: name})
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/delete_virtual_cluster", c.HostURL), bytes.NewReader(payload))
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

// GetVirtualClusters - Returns list of virtual clusters.
func (c *Client) GetVirtualClusters() ([]VirtualCluster, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/list_virtual_clusters", c.HostURL), nil)
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, nil)
	if err != nil {
		return nil, err
	}

	res := VirtualClusterListResponse{}
	err = json.Unmarshal(body, &res)
	if err != nil {
		return nil, err
	}

	return res.VirtualClusters, nil
}

// FindVirtualCluster - Returns virtual cluster with given name.
func (c *Client) FindVirtualCluster(name string) (*VirtualCluster, error) {
	vcs, err := c.GetVirtualClusters()
	if err != nil {
		return nil, err
	}

	for _, vc := range vcs {
		if vc.Name == name {
			return &vc, nil
		}
	}

	return nil, fmt.Errorf("could not find virtual cluster with name %s: %w", name, ErrNotFound)
}

// GetDefaultCluster - Return the default virtual cluster.
func (c *Client) GetDefaultCluster() (*VirtualCluster, error) {
	return c.FindVirtualCluster("vcn_default")
}
