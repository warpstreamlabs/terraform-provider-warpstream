package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

// VirtualClusterConfiguration is what describe reports. Every field here is always populated by
// the API, so reads never have to invent a value; write requests use ConfigurationUpdate, which
// carries only the fields the provider actually sends.
type VirtualClusterConfiguration struct {
	AclsEnabled              bool   `json:"are_acls_enabled"`
	ACLShadowingEnabled      bool   `json:"acl_shadowing_enabled"`
	AutoCreateTopic          bool   `json:"is_auto_create_topic_enabled"`
	DefaultNumPartitions     int64  `json:"default_num_partitions"`
	DefaultRetentionMillis   int64  `json:"default_retention_millis"`
	DefaultTopicType         string `json:"default_topic_type"`
	EnableDeletionProtection bool   `json:"enable_deletion_protection"`
	EnableSoftTopicDeletion  bool   `json:"soft_delete_topics_enabled"`
	Tier                     string `json:"tier,omitempty"`

	// The api returns the raw time.Duration value so we have to parse it accordingly.
	// Unlike default_retention_millis which is returned from the api in milliseconds.
	SoftTopicDeletionTTL *time.Duration `json:"inactive_topics_ttl,omitempty"`

	// BrokerConfigs is the generic representation of cluster-level broker configs, keyed by
	// Kafka-style name (e.g. "message.max.bytes"); all values are strings. It is absent until
	// a config has been written through it.
	BrokerConfigs map[string]*string `json:"broker_configs,omitempty"`
}

// ConfigurationUpdate is the update request body. The settings that have a Kafka-style config
// name are all written through BrokerConfigs; only the ones with no equivalent there are still
// sent as their own field.
type ConfigurationUpdate struct {
	AclsEnabled              bool              `json:"are_acls_enabled"`
	ACLShadowingEnabled      bool              `json:"acl_shadowing_enabled"`
	EnableDeletionProtection bool              `json:"enable_deletion_protection"`
	Tier                     string            `json:"tier,omitempty"`
	BrokerConfigs            map[string]string `json:"broker_configs,omitempty"`
}

type ConfigurationDescribeRequest struct {
	VirtualClusterID string `json:"virtual_cluster_id"`
}

type ConfigurationDescribeResponse struct {
	Configuration VirtualClusterConfiguration `json:"virtual_cluster_configuration"`
}

type ConfigurationUpdateRequest struct {
	VirtualClusterID string              `json:"virtual_cluster_id"`
	Configuration    ConfigurationUpdate `json:"virtual_cluster_configuration"`
}

// GetConfiguration - Describe virtual cluster configuration.
func (c *Client) GetConfiguration(vc VirtualCluster) (*VirtualClusterConfiguration, error) {
	payload, err := json.Marshal(ConfigurationDescribeRequest{VirtualClusterID: vc.ID})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/describe_virtual_cluster_configuration", c.HostURL), bytes.NewReader(payload))
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
func (c *Client) UpdateConfiguration(cfg ConfigurationUpdate, vc VirtualCluster) error {
	payload, err := json.Marshal(ConfigurationUpdateRequest{VirtualClusterID: vc.ID, Configuration: cfg})
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/update_virtual_cluster_configuration", c.HostURL), bytes.NewReader(payload))
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
