package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

type TopicCreateRequest struct {
	VirtualClusterID string             `json:"virtual_cluster_id"`
	TopicName        string             `json:"topic_name"`
	PartitionCount   int                `json:"partition_count"`
	Configs          map[string]*string `json:"configs"`
}

type TopicDescribeRequest struct {
	VirtualClusterID string `json:"virtual_cluster_id"`
	TopicName        string `json:"topic_name"`
}

type TopicDescribeResponse struct {
	PartitionCount int                `json:"partition_count"`
	Configs        map[string]*string `json:"configs"`
}

type TopicUpdateRequest struct {
	VirtualClusterID string             `json:"virtual_cluster_id"`
	TopicName        string             `json:"topic_name"`
	PartitionCount   *int               `json:"partition_count,omitempty"`
	Configs          map[string]*string `json:"configs"`
}

type TopicDeleteRequest struct {
	VirtualClusterID string `json:"virtual_cluster_id"`
	TopicName        string `json:"topic_name"`
}

func (c *Client) CreateTopic(virtualClusterID string, topicName string, partitionCount int, configs map[string]*string) error {
	payload, err := json.Marshal(TopicCreateRequest{
		VirtualClusterID: virtualClusterID,
		TopicName:        topicName,
		PartitionCount:   partitionCount,
		Configs:          configs,
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/create_topic", c.HostURL), bytes.NewReader(payload))
	if err != nil {
		return err
	}

	_, err = c.doRequest(req, nil)
	if err != nil {
		return fmt.Errorf("error doing creating topic request: %w", err)
	}

	return nil
}

func (c *Client) DescribeTopic(virtualClusterID string, topicName string) (*Topic, error) {
	payload, err := json.Marshal(TopicDescribeRequest{
		VirtualClusterID: virtualClusterID,
		TopicName:        topicName,
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/describe_topic", c.HostURL), bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, nil)
	if err != nil {
		return nil, fmt.Errorf("error doing describe topic request: %w", err)
	}

	res := TopicDescribeResponse{}
	err = json.Unmarshal(body, &res)
	if err != nil {
		return nil, err
	}

	return &Topic{
		VirtualClusterID: virtualClusterID,
		TopicName:        topicName,
		PartitionCount:   res.PartitionCount,
		Configs:          res.Configs,
	}, nil
}

func (c *Client) UpdateTopic(virtualClusterID string, topicName string, partitionCount *int, configs map[string]*string) error {
	payload, err := json.Marshal(TopicUpdateRequest{
		VirtualClusterID: virtualClusterID,
		TopicName:        topicName,
		PartitionCount:   partitionCount,
		Configs:          configs,
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/update_topic", c.HostURL), bytes.NewReader(payload))
	if err != nil {
		return err
	}

	_, err = c.doRequest(req, nil)
	if err != nil {
		return fmt.Errorf("error doing update topic request: %w", err)
	}

	return nil
}

func (c *Client) DeleteTopic(virtualClusterID string, topicName string) error {
	payload, err := json.Marshal(TopicDeleteRequest{
		VirtualClusterID: virtualClusterID,
		TopicName:        topicName,
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/delete_topic", c.HostURL), bytes.NewReader(payload))
	if err != nil {
		return err
	}

	_, err = c.doRequest(req, nil)
	if err != nil {
		return fmt.Errorf("error doing delete topic request: %w", err)
	}

	return nil
}
