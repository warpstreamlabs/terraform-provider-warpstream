package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

type VirtualClusterConfiguration struct {
	AclsEnabled              bool   `json:"are_acls_enabled"`
	AutoCreateTopic          bool   `json:"is_auto_create_topic_enabled"`
	DefaultNumPartitions     int64  `json:"default_num_partitions"`
	DefaultRetentionMillis   int64  `json:"default_retention_millis"`
	EnableDeletionProtection bool   `json:"enable_deletion_protection"`
	SoftDeleteTopicEnable    bool   `json:"warpstream.soft.delete.topic.enable"`
	SoftDeleteTopicTTLHours  int64  `json:"warpstream.soft.delete.topic.ttl.hours"`
	Tier                     string `json:"tier,omitempty"`
}

type ConfigurationDescribeRequest struct {
	VirtualClusterID string `json:"virtual_cluster_id"`
}

type ConfigurationDescribeResponse struct {
	Configuration VirtualClusterConfiguration `json:"virtual_cluster_configuration"`
}

type ConfigurationUpdateRequest struct {
	VirtualClusterID string                      `json:"virtual_cluster_id"`
	Configuration    VirtualClusterConfiguration `json:"virtual_cluster_configuration"`
}

// GetConfiguration - Describe virtual cluster configuration.
func (c *Client) GetConfiguration(vc VirtualCluster) (*VirtualClusterConfiguration, error) {
	payload, err := json.Marshal(ConfigurationDescribeRequest{VirtualClusterID: vc.ID})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("GET", fmt.Sprintf("%s/describe_virtual_cluster_configuration", c.HostURL), bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, nil)
	if err != nil {
		return nil, err
	}

	res := ConfigurationDescribeResponse{}
	err = json.Unmarshal(body, &res)
	if err != nil {
		return nil, err
	}

	return &res.Configuration, nil
}

// UpdateConfiguration - Update virtual cluster configuration.
func (c *Client) UpdateConfiguration(cfg VirtualClusterConfiguration, vc VirtualCluster) error {
	payload, err := json.Marshal(ConfigurationUpdateRequest{VirtualClusterID: vc.ID, Configuration: cfg})
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/update_virtual_cluster_configuration", c.HostURL), bytes.NewReader(payload))
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
