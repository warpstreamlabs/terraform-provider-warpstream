package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

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

	cfg := VirtualClusterConfiguration{
		AclsEnabled: res.Configuration.AclsEnabled,
	}
	return &cfg, nil
}

// UpdateConfiguration - Update virtual cluster configuration.
func (c *Client) UpdateConfiguration(cfg VirtualClusterConfiguration, vc VirtualCluster) error {
	payload, err := json.Marshal(ConfigurationUpdateRequest{VirtualClusterID: vc.ID, Configuration: cfg})
	if err != nil {
		return err
	}

	req, err := http.NewRequest("GET", fmt.Sprintf("%s/update_virtual_cluster_configuration", c.HostURL), bytes.NewReader(payload))
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
