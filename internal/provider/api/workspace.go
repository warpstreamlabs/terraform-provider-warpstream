package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

type Workspace struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
}

type WorkspaceListResponse struct {
	Workspaces []Workspace `json:"workspaces"`
}

type WorkspaceCreateRequest struct {
	Name                       string `json:"workspace_name"`
	SkipApplicationKeyCreation bool   `json:"skip_application_key_creation"`
}

type WorkspaceDeleteRequest struct {
	ID string `json:"workspace_id"`
}

type WorkspaceRenameRequest struct {
	ID   string `json:"workspace_id"`
	Name string `json:"new_workspace_name"`
}

// CreateWorkspace - Create new Workspace.
func (c *Client) CreateWorkspace(name string) (string, error) {
	payload, err := json.Marshal(WorkspaceCreateRequest{Name: name, SkipApplicationKeyCreation: true})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/create_workspace", c.HostURL), bytes.NewReader(payload))
	if err != nil {
		return "", err
	}

	body, err := c.doRequest(req, nil)
	if err != nil {
		return "", err
	}

	var res struct {
		ID string `json:"workspace_id"`
	}
	err = json.Unmarshal(body, &res)
	if err != nil {
		return "", err
	}

	return res.ID, nil
}

// DeleteWorkspace - Delete a Workspace.
func (c *Client) DeleteWorkspace(id string) error {
	payload, err := json.Marshal(WorkspaceDeleteRequest{ID: id})
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/delete_workspace", c.HostURL), bytes.NewReader(payload))
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

// GetWorkspaces - Returns list of Workspaces.
func (c *Client) GetWorkspaces() ([]Workspace, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/list_workspaces", c.HostURL), nil)
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, nil)
	if err != nil {
		return nil, err
	}

	res := WorkspaceListResponse{}
	err = json.Unmarshal(body, &res)
	if err != nil {
		return nil, err
	}

	return res.Workspaces, nil
}

// GetWorkspace - Return one Workspace.
func (c *Client) GetWorkspace(workspaceID string) (*Workspace, error) {
	workspaces, err := c.GetWorkspaces()

	if err != nil {
		return nil, fmt.Errorf("failed to get workspaces list: %w", err)
	}

	for _, ws := range workspaces {
		if ws.ID == workspaceID {
			return &ws, nil
		}
	}

	return nil, ErrNotFound
}

// RenameWorkspace - Rename a Workspace.
func (c *Client) RenameWorkspace(workspaceID string, newName string) error {
	payload, err := json.Marshal(WorkspaceRenameRequest{ID: workspaceID, Name: newName})
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/rename_workspace", c.HostURL), bytes.NewReader(payload))
	if err != nil {
		return err
	}

	_, err = c.doRequest(req, nil)
	if err != nil {
		return err
	}

	return nil
}
