package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

type TagsDescribeRequest struct {
	VirtualClusterID string `json:"virtual_cluster_id"`
}

type TagsDescribeResponse struct {
	Tags map[string]string `json:"tags"`
}

type TagsUpdateRequest struct {
	VirtualClusterID string            `json:"virtual_cluster_id"`
	Tags             map[string]string `json:"tags"`
}

func (c *Client) GetTags(vc VirtualCluster) (map[string]string, error) {
	payload, err := json.Marshal(TagsDescribeRequest{VirtualClusterID: vc.ID})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("GET", fmt.Sprintf("%s/describe_virtual_cluster_tags", c.HostURL), bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, nil)
	if err != nil {
		return nil, err
	}

	res := TagsDescribeResponse{}
	err = json.Unmarshal(body, &res)
	if err != nil {
		return nil, err
	}
	if res.Tags == nil {
		return make(map[string]string), nil
	}

	return res.Tags, nil
}

func (c *Client) UpdateTags(tags map[string]string, vc VirtualCluster) error {
	payload, err := json.Marshal(TagsUpdateRequest{VirtualClusterID: vc.ID, Tags: tags})
	if err != nil {
		return err
	}

	req, err := http.NewRequest("GET", fmt.Sprintf("%s/update_virtual_cluster_tags", c.HostURL), bytes.NewReader(payload))
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
