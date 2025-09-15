package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type HTTPListPipelinesRequest struct {
	VirtualClusterID string `json:"virtual_cluster_id"`
}

type HTTPListPipelinesResponse struct {
	Pipelines []HTTPPipelineOverview `json:"pipelines"`
}

type HTTPPipelineOverview struct {
	ID                      string `json:"id"`
	Name                    string `json:"name"`
	State                   string `json:"state"`
	Type                    string `json:"type"`
	DeployedConfigurationId string `json:"deployed_configuration_id"`
}

type HTTPCreatePipelineRequest struct {
	VirtualClusterID string `json:"virtual_cluster_id"`
	PipelineName     string `json:"pipeline_name"`
	Type             string `json:"pipeline_type"`
}

type HTTPCreatePipelineResponse struct {
	PipelineID                      string `json:"pipeline_id"`
	PipelineName                    string `json:"pipeline_name"`
	PipelineState                   string `json:"pipeline_state"`
	PipelineType                    string `json:"pipeline_type"`
	PipelineDeployedConfigurationId string `json:"pipeline_deployed_configuration_id"`
}

type HTTPCreatePipelineConfigurationRequest struct {
	VirtualClusterID  string `json:"virtual_cluster_id"`
	PipelineID        string `json:"pipeline_id"`
	ConfigurationYAML string `json:"configuration_yaml"`
}

type HTTPCreatePipelineConfigurationResponse struct {
	ConfigurationID string   `json:"configuration_id"`
	RemovedKeys     []string `json:"removed_keys"`
}

type HTTPChangePipelineStateRequest struct {
	VirtualClusterID        string  `json:"virtual_cluster_id"`
	PipelineID              string  `json:"pipeline_id"`
	DesiredState            *string `json:"desired_state"`
	DeployedConfigurationID *string `json:"deployed_configuration_id"`
}

type HTTPChangePipelineStateResponse struct {
}

type HTTPDeletePipelineRequest struct {
	VirtualClusterID string `json:"virtual_cluster_id"`
	PipelineID       string `json:"pipeline_id"`
}

type HTTPDeletePipelineResponse struct {
}

type HTTPDescribePipelineRequest struct {
	VirtualClusterID string `json:"virtual_cluster_id"`
	PipelineID       string `json:"pipeline_id"`
}

type HTTPDescribePipelineResponse struct {
	PipelineOverview HTTPPipelineOverview        `json:"pipeline_overview"`
	Configurations   []HTTPPipelineConfiguration `json:"pipeline_configurations"`
}

type HTTPPipelineConfiguration struct {
	ID                string `json:"id"`
	Version           int    `json:"version"`
	ConfigurationYAML string `json:"configuration_yaml"`
}

func (c *Client) CreatePipeline(
	ctx context.Context,
	req HTTPCreatePipelineRequest,
) (HTTPCreatePipelineResponse, error) {
	resp := &HTTPCreatePipelineResponse{}
	if err := c.doJSONHTTP(ctx, req, "create_pipeline", resp); err != nil {
		return HTTPCreatePipelineResponse{}, fmt.Errorf("error creating pipeline: %w", err)
	}
	return *resp, nil
}

func (c *Client) ListPipelines(
	ctx context.Context,
	req HTTPListPipelinesRequest,
) (HTTPListPipelinesResponse, error) {
	resp := &HTTPListPipelinesResponse{}
	if err := c.doJSONHTTP(ctx, req, "list_pipelines", resp); err != nil {
		return HTTPListPipelinesResponse{}, fmt.Errorf("error listing pipelines: %w", err)
	}
	return *resp, nil
}

func (c *Client) CreatePipelineConfiguration(
	ctx context.Context,
	req HTTPCreatePipelineConfigurationRequest,
) (HTTPCreatePipelineConfigurationResponse, error) {
	resp := &HTTPCreatePipelineConfigurationResponse{}
	if err := c.doJSONHTTP(ctx, req, "create_pipeline_configuration", resp); err != nil {
		return HTTPCreatePipelineConfigurationResponse{}, fmt.Errorf("error creating pipeline configuration: %w", err)
	}
	return *resp, nil
}

func (c *Client) ChangePipelineState(
	ctx context.Context,
	req HTTPChangePipelineStateRequest,
) (HTTPChangePipelineStateResponse, error) {
	resp := &HTTPChangePipelineStateResponse{}
	if err := c.doJSONHTTP(ctx, req, "change_pipeline_state", resp); err != nil {
		return HTTPChangePipelineStateResponse{}, fmt.Errorf("error changing pipeline state: %w", err)
	}
	return *resp, nil
}

func (c *Client) DescribePipeline(
	ctx context.Context,
	req HTTPDescribePipelineRequest,
) (HTTPDescribePipelineResponse, error) {
	resp := &HTTPDescribePipelineResponse{}
	if err := c.doJSONHTTP(ctx, req, "describe_pipeline", resp); err != nil {
		return HTTPDescribePipelineResponse{}, fmt.Errorf("error describing pipeline state: %w", err)
	}
	return *resp, nil
}

func (c *Client) DeletePipeline(
	ctx context.Context,
	req HTTPDeletePipelineRequest,
) (HTTPDeletePipelineResponse, error) {
	resp := &HTTPDeletePipelineResponse{}
	if err := c.doJSONHTTP(ctx, req, "delete_pipeline", resp); err != nil {
		return HTTPDeletePipelineResponse{}, fmt.Errorf("error deleting pipeline state: %w", err)
	}
	return *resp, nil
}

func (c *Client) doJSONHTTP(
	ctx context.Context,
	req any,
	path string,
	resp any,
) error {
	ctx, cc := context.WithTimeout(ctx, 15*time.Second)
	defer cc()

	marshaled, err := json.Marshal(&req)
	if err != nil {
		return fmt.Errorf("error marshaling request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(
		ctx, http.MethodPost,
		c.HostURL+"/"+path,
		bytes.NewReader(marshaled))
	if err != nil {
		return fmt.Errorf("error creating new HTTP request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	respB, err := c.doRequest(httpReq, nil)
	if err != nil {
		return fmt.Errorf("error writing/reading request: %w", err)
	}

	if err := json.Unmarshal(respB, resp); err != nil {
		return fmt.Errorf("error JSON unmarshaling response: %w\nPath: %s\nResponse: %s", err, c.HostURL+"/"+path, string(respB))
	}

	return nil
}
