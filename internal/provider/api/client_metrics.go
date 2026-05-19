package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

// ClientMetricsSubscription is a named KIP-714 client metrics subscription,
// matching HTTPClientMetricsSubscription on the backend. It is returned by
// the list and describe endpoints and carried (alongside virtual_cluster_id
// at the top level) by the batch update endpoint. All three content fields
// are pointers so that "unset" vs "zero value" round-trips correctly through
// the API's omitempty semantics.
type ClientMetricsSubscription struct {
	Name       string  `json:"name"`
	IntervalMs *int32  `json:"interval_ms,omitempty"`
	Metrics    *string `json:"metrics,omitempty"`
	Match      *string `json:"match,omitempty"`
}

type ClientMetricsSubscriptionListRequest struct {
	VirtualClusterID string `json:"virtual_cluster_id"`
}

type ClientMetricsSubscriptionListResponse struct {
	Subscriptions []ClientMetricsSubscription `json:"subscriptions"`
}

type ClientMetricsSubscriptionDescribeRequest struct {
	VirtualClusterID string `json:"virtual_cluster_id"`
	Name             string `json:"name"`
}

type ClientMetricsSubscriptionDescribeResponse struct {
	Subscription ClientMetricsSubscription `json:"client_metrics_subscription"`
}

// ClientMetricsSubscriptionsUpdateRequest matches
// HTTPUpdateClientMetricsSubscriptionsRequest: a batch upsert keyed by name.
// The server rejects duplicate names in the list and validates every entry
// before persisting anything.
type ClientMetricsSubscriptionsUpdateRequest struct {
	VirtualClusterID           string                      `json:"virtual_cluster_id"`
	ClientMetricsSubscriptions []ClientMetricsSubscription `json:"client_metrics_subscriptions"`
}

// ClientMetricsSubscriptionsDeleteRequest matches
// HTTPDeleteClientMetricsSubscriptionsRequest: a batch delete by name.
// The call is all-or-nothing: if any requested name is absent on the
// cluster, the server returns 404 and no names are removed.
type ClientMetricsSubscriptionsDeleteRequest struct {
	VirtualClusterID               string   `json:"virtual_cluster_id"`
	ClientMetricsSubscriptionNames []string `json:"client_metrics_subscription_names"`
}

// ListClientMetricsSubscriptions lists all KIP-714 client metrics
// subscriptions for the given virtual cluster.
func (c *Client) ListClientMetricsSubscriptions(virtualClusterID string) ([]ClientMetricsSubscription, error) {
	payload, err := json.Marshal(ClientMetricsSubscriptionListRequest{
		VirtualClusterID: virtualClusterID,
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/list_client_metrics_subscriptions", c.HostURL), bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, nil)
	if err != nil {
		return nil, fmt.Errorf("error listing client metrics subscriptions: %w", err)
	}

	var res ClientMetricsSubscriptionListResponse
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, err
	}
	return res.Subscriptions, nil
}

// DescribeClientMetricsSubscription returns the named subscription, or
// ErrNotFound if it does not exist.
func (c *Client) DescribeClientMetricsSubscription(virtualClusterID string, name string) (*ClientMetricsSubscription, error) {
	payload, err := json.Marshal(ClientMetricsSubscriptionDescribeRequest{
		VirtualClusterID: virtualClusterID,
		Name:             name,
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/describe_client_metrics_subscription", c.HostURL), bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, nil)
	if err != nil {
		return nil, fmt.Errorf("error describing client metrics subscription %q: %w", name, err)
	}

	var res ClientMetricsSubscriptionDescribeResponse
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, err
	}
	return &res.Subscription, nil
}

// UpdateClientMetricsSubscriptions upserts a batch of named subscriptions
// atomically. Every entry replaces any existing subscription with the same
// name (whole-subscription replace: nil fields become unset). The server
// rejects duplicate names in the batch and validates every entry before
// persisting anything; if any entry fails validation, nothing is written.
// An empty batch is a no-op.
func (c *Client) UpdateClientMetricsSubscriptions(virtualClusterID string, subscriptions []ClientMetricsSubscription) error {
	payload, err := json.Marshal(ClientMetricsSubscriptionsUpdateRequest{
		VirtualClusterID:           virtualClusterID,
		ClientMetricsSubscriptions: subscriptions,
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/update_client_metrics_subscriptions", c.HostURL), bytes.NewReader(payload))
	if err != nil {
		return err
	}

	if _, err := c.doRequest(req, nil); err != nil {
		return fmt.Errorf("error updating %d client metrics subscription(s): %w", len(subscriptions), err)
	}
	return nil
}

// DeleteClientMetricsSubscriptions deletes a batch of subscriptions by name.
// The call is all-or-nothing: if any name is absent on the cluster, the
// server returns ErrNotFound and no names are removed. Duplicate names in
// the list are tolerated by the server. An empty batch is a no-op.
func (c *Client) DeleteClientMetricsSubscriptions(virtualClusterID string, names []string) error {
	payload, err := json.Marshal(ClientMetricsSubscriptionsDeleteRequest{
		VirtualClusterID:               virtualClusterID,
		ClientMetricsSubscriptionNames: names,
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/delete_client_metrics_subscriptions", c.HostURL), bytes.NewReader(payload))
	if err != nil {
		return err
	}

	if _, err := c.doRequest(req, nil); err != nil {
		return fmt.Errorf("error deleting %d client metrics subscription(s): %w", len(names), err)
	}
	return nil
}
