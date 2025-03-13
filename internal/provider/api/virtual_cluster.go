package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/types"
)

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
}

type VirtualClusterDescribeRequest struct {
	ID string `json:"virtual_cluster_id"`
}

type VirtualClusterCreateRequest struct {
	Name          string  `json:"virtual_cluster_name"`
	Type          string  `json:"virtual_cluster_type,omitempty"`
	Region        *string `json:"virtual_cluster_region,omitempty"`
	RegionGroup   *string `json:"virtual_cluster_region_group,omitempty"`
	CloudProvider string  `json:"virtual_cluster_cloud_provider,omitempty"`
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

// CreateVirtualCluster - Create new virtual cluster.
func (c *Client) CreateVirtualCluster(name string, opts ClusterParameters) (*VirtualCluster, error) {
	var trimmed string
	if opts.Type == types.VirtualClusterTypeSchemaRegistry {
		trimmed = strings.TrimPrefix(name, "vcn_sr_")
	} else {
		trimmed = strings.TrimPrefix(name, "vcn_")
	}
	payload, err := json.Marshal(VirtualClusterCreateRequest{
		Name:          trimmed,
		Type:          opts.Type,
		Region:        opts.Region,
		RegionGroup:   opts.RegionGroup,
		CloudProvider: opts.Cloud,
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
	}
	return &vc, nil
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
