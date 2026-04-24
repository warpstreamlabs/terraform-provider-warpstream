package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

// EventTypeConfig represents per-event-type configuration.
type EventTypeConfig struct {
	Enabled              *bool   `json:"enabled,omitempty"`
	RetentionPeriodNanos *uint64 `json:"retention_period_nanos,omitempty"`
}

// EventsState represents the events state for a virtual cluster.
type EventsState struct {
	Enabled    bool                       `json:"enabled"`
	EventTypes map[string]EventTypeConfig `json:"event_types,omitempty"`
}

// EventsStateDescribeRequest for GetEventsState.
type EventsStateDescribeRequest struct {
	VirtualClusterID string `json:"virtual_cluster_id"`
}

// EventsStateDescribeResponse for GetEventsState.
type EventsStateDescribeResponse struct {
	Enabled    bool                       `json:"enabled"`
	EventTypes map[string]EventTypeConfig `json:"event_types"`
}

// EventsStateUpdateRequest for UpdateEventsState.
type EventsStateUpdateRequest struct {
	VirtualClusterID string                     `json:"virtual_cluster_id"`
	Enabled          *bool                      `json:"enabled,omitempty"`
	EventTypes       map[string]EventTypeConfig `json:"event_types,omitempty"`
}

// GetEventsState - Describe virtual cluster events state.
func (c *Client) GetEventsState(vc VirtualCluster) (*EventsState, error) {
	payload, err := json.Marshal(EventsStateDescribeRequest{VirtualClusterID: vc.ID})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/get_events_state", c.HostURL), bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, nil)
	if err != nil {
		return nil, err
	}

	res := EventsStateDescribeResponse{}
	err = json.Unmarshal(body, &res)
	if err != nil {
		return nil, err
	}

	return &EventsState{
		Enabled:    res.Enabled,
		EventTypes: res.EventTypes,
	}, nil
}

// UpdateEventsState - Update virtual cluster events state.
func (c *Client) UpdateEventsState(enabled *bool, eventTypes map[string]EventTypeConfig, vc VirtualCluster) error {
	payload, err := json.Marshal(EventsStateUpdateRequest{
		VirtualClusterID: vc.ID,
		Enabled:          enabled,
		EventTypes:       eventTypes,
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/update_events_state", c.HostURL), bytes.NewReader(payload))
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
