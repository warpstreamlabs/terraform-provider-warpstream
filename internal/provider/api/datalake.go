package api

import (
	"context"
	"fmt"
)

type DLTable struct {
	TableName               string `json:"table_name"`
	TableUUID               string `json:"table_uuid"`
	SourceStreamName        string `json:"source_stream_name"`
	SourceClusterName       string `json:"source_cluster_name"`
	StatsEstimatedByteCount int64  `json:"stats_estimated_byte_count"`
	StatsEstimatedRowCount  int64  `json:"stats_estimated_row_count"`
	CreatedAtUnixNanos      uint64 `json:"created_at_unix_nanos"`
}

type DLListTablesRequest struct {
	VirtualClusterID string `json:"virtual_cluster_id"`
}

type DLListTablesResponse struct {
	Tables []*DLTable `json:"tables"`
}

type DLGetTableRequest struct {
	VirtualClusterID string `json:"virtual_cluster_id"`
	TableUUID        string `json:"table_uuid,omitempty"`
	TableName        string `json:"table_name,omitempty"`
}

type DLGetTableResponse struct {
	Table *DLTable `json:"table"`
}

type DLDeleteTableRequest struct {
	VirtualClusterID string `json:"virtual_cluster_id"`
	TableUUID        string `json:"table_uuid"`
}

type DLDeleteTableResponse struct{}

func (c *Client) DLListTables(
	ctx context.Context,
	req DLListTablesRequest,
) (DLListTablesResponse, error) {
	resp := &DLListTablesResponse{}
	if err := c.doJSONHTTP(ctx, req, "dl/list_tables", resp); err != nil {
		return DLListTablesResponse{}, fmt.Errorf("error listing datalake tables: %w", err)
	}
	return *resp, nil
}

func (c *Client) DLGetTable(
	ctx context.Context,
	req DLGetTableRequest,
) (DLGetTableResponse, error) {
	resp := &DLGetTableResponse{}
	if err := c.doJSONHTTP(ctx, req, "dl/get_table", resp); err != nil {
		return DLGetTableResponse{}, fmt.Errorf("error getting datalake table: %w", err)
	}
	return *resp, nil
}

func (c *Client) DLDeleteTable(
	ctx context.Context,
	req DLDeleteTableRequest,
) (DLDeleteTableResponse, error) {
	resp := &DLDeleteTableResponse{}
	if err := c.doJSONHTTP(ctx, req, "dl/delete_table", resp); err != nil {
		return DLDeleteTableResponse{}, fmt.Errorf("error deleting datalake table: %w", err)
	}
	return *resp, nil
}
