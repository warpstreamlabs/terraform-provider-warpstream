package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

type UserRole struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	AccessGrants []AccessGrant `json:"access_grants"`
	CreatedAt    string        `json:"created_at"`
}

type UserRoleListResponse struct {
	Roles []UserRole `json:"user_roles"`
}

type UserRoleCreateRequest struct {
	Name         string        `json:"user_role_name"`
	AccessGrants []AccessGrant `json:"access_grants"`
}

type UserRoleDeleteRequest struct {
	ID string `json:"user_role_id"`
}

// CreateUserRole - Create new User Role.
func (c *Client) CreateUserRole(name string, grants []AccessGrant) (string, error) {
	payload, err := json.Marshal(UserRoleCreateRequest{Name: name, AccessGrants: grants})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/create_user_role", c.HostURL), bytes.NewReader(payload))
	if err != nil {
		return "", err
	}

	body, err := c.doRequest(req, nil)
	if err != nil {
		return "", err
	}

	var res struct {
		ID string `json:"user_role_id"`
	}
	err = json.Unmarshal(body, &res)
	if err != nil {
		return "", err
	}

	return res.ID, nil
}

// DeleteUserRole - Delete a User Role.
func (c *Client) DeleteUserRole(id string) error {
	payload, err := json.Marshal(UserRoleDeleteRequest{ID: id})
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/delete_user_role", c.HostURL), bytes.NewReader(payload))
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

// getUserRoles - Returns list of User Roles.
func (c *Client) getUserRoles() ([]UserRole, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/list_user_roles", c.HostURL), nil)
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, nil)
	if err != nil {
		return nil, err
	}

	res := UserRoleListResponse{}
	err = json.Unmarshal(body, &res)
	if err != nil {
		return nil, err
	}

	return res.Roles, nil
}

// GetUserRole - Return one Role.
func (c *Client) GetUserRole(roleID string) (*UserRole, error) {
	roles, err := c.getUserRoles()

	if err != nil {
		return nil, fmt.Errorf("failed to get user roles list: %w", err)
	}

	for _, role := range roles {
		if role.ID == roleID {
			return &role, nil
		}
	}

	return nil, ErrNotFound
}

func (c *Client) FindUserRole(name string) (*UserRole, error) {
	roles, err := c.getUserRoles()
	if err != nil {
		return nil, fmt.Errorf("failed to get user roles list: %w", err)
	}

	for _, role := range roles {
		if role.Name == name {
			return &role, nil
		}
	}

	return nil, ErrNotFound
}
