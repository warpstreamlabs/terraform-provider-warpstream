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

// CreateVirtualClusterV2 creates a virtual cluster together with a hashed agent key through
// create_virtual_cluster_v2. The key's secret is only in this response. After an ambiguous failure, a
// cluster with this name and type created since the attempt started is ours; deleting it also revokes
// its agent key.
func (c *Client) CreateVirtualClusterV2(name string, opts ClusterParameters) (*VirtualCluster, *APIKey, error) {
	payload, err := json.Marshal(VirtualClusterCreateRequest{
		Name:          strings.TrimPrefix(name, "vcn_"),
		Type:          opts.Type,
		Tier:          opts.Tier,
		Region:        opts.Region,
		RegionGroup:   opts.RegionGroup,
		CloudProvider: opts.Cloud,
		Tags:          opts.Tags,
	})
	if err != nil {
		return nil, nil, err
	}

	res, err := createWithRecovery(
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
		func(startedAt time.Time) error { return c.deleteOrphanedVirtualCluster(name, opts.Type, startedAt) },
		fmt.Sprintf("delete any virtual cluster named %s with list_virtual_clusters and delete_virtual_cluster", name),
	)
	if err != nil {
		return nil, nil, err
	}

	vc := VirtualCluster{
		ID:            res.VirtualClusterID,
		AgentPoolID:   res.AgentPoolID,
		AgentPoolName: res.AgentPoolName,
		Name:          res.Name,
		BootstrapURL:  res.BootstrapURL,
		WorkspaceID:   res.WorkspaceID,
	}
	if res.AgentKey.ID == "" {
		return &vc, nil, nil
	}
	return &vc, &res.AgentKey, nil
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
