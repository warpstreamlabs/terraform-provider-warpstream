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

const (
	VirtualClusterTypeBYOC           = "byoc"
	VirtualClusterTypeSchemaRegistry = "byoc_schema_registry"
	VirtualClusterTypeTableFlow      = "byoc_data_lake"

	// legacy is only available for certain tenants, this is controlled on the Warpstream side.
	VirtualClusterTierLegacy       = "legacy"
	VirtualClusterTierDev          = "dev"
	VirtualClusterTierFundamentals = "fundamentals"
	VirtualClusterTierPro          = "pro"
	VirtualClusterTierEnterprise   = "enterprise"
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
	Tier          string        `json:"tier"`
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

	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/describe_virtual_cluster", c.HostURL), bytes.NewReader(payload))
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
	Type        string
	Tier        string
	Region      *string
	RegionGroup *string
	Cloud       string
	Tags        map[string]string
	// HashedAgentKey creates the cluster with a hashed agent key through create_virtual_cluster_v2, returned
	// in AgentKeys. Otherwise no agent key is created.
	HashedAgentKey bool
}

// CreateVirtualCluster - Create new virtual cluster.
func (c *Client) CreateVirtualCluster(name string, opts ClusterParameters) (*VirtualCluster, error) {
	var trimmed string
	switch opts.Type {
	case VirtualClusterTypeSchemaRegistry:
		trimmed = strings.TrimPrefix(name, "vcn_sr_")
	case VirtualClusterTypeTableFlow:
		trimmed = strings.TrimPrefix(name, "vcn_dl_")
	default:
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
		SkipAgentKeyCreation: !opts.HashedAgentKey,
	})
	if err != nil {
		return nil, err
	}

	var res *VirtualClusterCreateResponse
	if opts.HashedAgentKey {
		res, err = c.createVirtualClusterV2(payload, name, opts.Type)
	} else {
		res, err = c.createVirtualClusterV1(payload)
	}
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

func (c *Client) createVirtualClusterV1(payload []byte) (*VirtualClusterCreateResponse, error) {
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/create_virtual_cluster", c.HostURL), bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, nil)
	if err != nil {
		return nil, err
	}

	res := VirtualClusterCreateResponse{}
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

// createVirtualClusterV2 calls create_virtual_cluster_v2, whose agent key secret is only in the create
// response. After an ambiguous failure, a cluster with this name and type created since the attempt
// started is ours; deleting it also revokes its agent key.
func (c *Client) createVirtualClusterV2(payload []byte, name, clusterType string) (*VirtualClusterCreateResponse, error) {
	return createWithRecovery(
		"virtual cluster "+name,
		func() (*VirtualClusterCreateResponse, error) {
			req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/create_virtual_cluster_v2", c.HostURL), bytes.NewReader(payload))
			if err != nil {
				return nil, err
			}
			body, err := c.doRequestOnce(req)
			if err != nil {
				return nil, err
			}
			res := VirtualClusterCreateResponse{}
			if err := json.Unmarshal(body, &res); err != nil {
				return nil, fmt.Errorf("%w: invalid create response: %w", ErrAmbiguous, err)
			}
			return &res, nil
		},
		func(startedAt time.Time) error { return c.deleteOrphanedVirtualCluster(name, clusterType, startedAt) },
		fmt.Sprintf("delete any virtual cluster named %s with list_virtual_clusters and delete_virtual_cluster", name),
	)
}

func (c *Client) deleteOrphanedVirtualCluster(name, clusterType string, startedAt time.Time) error {
	clusters, err := c.GetVirtualClusters()
	if err != nil {
		return err
	}

	for _, cluster := range clusters {
		if cluster.Name != name {
			continue
		}
		if clusterType != "" && cluster.Type != clusterType {
			return fmt.Errorf("virtual cluster %s (%s) exists with type %s", name, cluster.ID, cluster.Type)
		}
		if err := requireCreatedSince("virtual cluster", name, cluster.ID, cluster.CreatedAt, startedAt); err != nil {
			return err
		}

		log.Printf("deleting virtual cluster %s (%s) left behind by the failed create", name, cluster.ID)
		if err := c.DeleteVirtualCluster(cluster.ID, cluster.Name); err != nil && !errors.Is(err, ErrNotFound) {
			return err
		}
		return nil
	}
	return nil
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

	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/rename_virtual_cluster", c.HostURL), bytes.NewReader(payload))
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

	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/delete_virtual_cluster", c.HostURL), bytes.NewReader(payload))
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
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/list_virtual_clusters", c.HostURL), nil)
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

type VirtualClusterUpdateTierRequest struct {
	VirtualClusterID string `json:"virtual_cluster_id"`
	Tier             string `json:"tier"`
}

func (c *Client) UpdateVirtualClusterTier(id string, tier string) error {
	payload, err := json.Marshal(VirtualClusterUpdateTierRequest{
		VirtualClusterID: id,
		Tier:             tier,
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/update_virtual_cluster_tier", c.HostURL), bytes.NewReader(payload))
	if err != nil {
		return err
	}

	_, err = c.doRequest(req, nil)
	if err != nil {
		return err
	}

	return nil
}
