package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"
)

type workspaceCreateV2Response struct {
	ID             string  `json:"workspace_id"`
	ApplicationKey *APIKey `json:"application_key"`
}

// CreateWorkspaceV2 creates a workspace together with a v2 application key through
// create_workspace_v2. The key's secret is only in this response. After an ambiguous failure, a
// workspace with this name created since the attempt started is ours; deleting it also revokes its
// application key.
func (c *Client) CreateWorkspaceV2(name string) (string, *APIKey, error) {
	payload, err := json.Marshal(WorkspaceCreateRequest{Name: name})
	if err != nil {
		return "", nil, err
	}

	res, err := createWithRecovery(
		"workspace "+name,
		func() (*workspaceCreateV2Response, error) {
			req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/create_workspace_v2", c.HostURL), bytes.NewReader(payload))
			if err != nil {
				return nil, err
			}
			body, err := c.doRequestOnce(req)
			if err != nil {
				return nil, err
			}
			res := workspaceCreateV2Response{}
			if err := json.Unmarshal(body, &res); err != nil {
				return nil, fmt.Errorf("%w: invalid create response: %w", ErrAmbiguous, err)
			}
			return &res, nil
		},
		func(startedAt time.Time) error { return c.deleteOrphanedWorkspace(name, startedAt) },
		fmt.Sprintf("delete any workspace named %s with list_workspaces and delete_workspace", name),
	)
	if err != nil {
		return "", nil, err
	}
	return res.ID, res.ApplicationKey, nil
}

func (c *Client) deleteOrphanedWorkspace(name string, startedAt time.Time) error {
	workspaces, err := c.GetWorkspaces()
	if err != nil {
		return err
	}

	for _, workspace := range workspaces {
		if workspace.Name != name {
			continue
		}
		if err := requireCreatedSince("workspace", name, workspace.ID, workspace.CreatedAt, startedAt); err != nil {
			return err
		}

		log.Printf("deleting workspace %s (%s) left behind by the failed create", name, workspace.ID)
		if err := c.DeleteWorkspace(workspace.ID); err != nil && !errors.Is(err, ErrNotFound) {
			return err
		}
		return nil
	}
	return nil
}
