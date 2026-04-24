package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

type SSOConfiguration struct {
	ID                   string `json:"id"`
	SSOIdentifier        string `json:"sso_identifier"`
	EntityID             string `json:"entity_id"`
	SAMLURL              string `json:"saml_url"`
	DefaultRoleID        string `json:"default_role_id"`
	EnableSSORoleMapping bool   `json:"enable_sso_role_mapping"`
	SigningCertificate   string `json:"signing_certificate"`
}

type SSOConfigurationCreateRequest struct {
	SSOIdentifier        string `json:"sso_identifier"`
	EntityID             string `json:"entity_id"`
	SAMLURL              string `json:"saml_url"`
	DefaultRoleID        string `json:"default_role_id"`
	EnableSSORoleMapping bool   `json:"enable_sso_role_mapping"`
	SigningCertificate   string `json:"signing_certificate"`
}
type SSOConfigurationUpdateRequest struct {
	SSOConnectionID      string `json:"sso_connection_id"`
	EntityID             string `json:"entity_id"`
	SAMLURL              string `json:"saml_url"`
	DefaultRoleID        string `json:"default_role_id"`
	EnableSSORoleMapping bool   `json:"enable_sso_role_mapping"`
	SigningCertificate   string `json:"signing_certificate"`
}

type SSOConfigurationDeleteRequest struct {
	SSOConnectionID string `json:"sso_connection_id"`
}

type SSOConfigurationGetResponseRequest struct {
	SSOConfiguration *SSOConfiguration `json:"sso_configuration"`
}

// CreateSSOConfiguration - Create new SSO configuration - there cannot be more than one per tenant.
func (c *Client) CreateSSOConfiguration(
	createRequest SSOConfigurationCreateRequest,
) (string, error) {
	payload, err := json.Marshal(createRequest)
	if err != nil {
		return "", fmt.Errorf("failed to marshal SSO configuration create request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/create_sso_configuration", c.HostURL), bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("failed to create SSO configuration create request: %w", err)
	}

	body, err := c.doRequest(req, nil)
	if err != nil {
		return "", fmt.Errorf("failed to POST SSO configuration create request: %w", err)
	}

	var res struct {
		SSOConnectionID string `json:"sso_connection_id"`
	}
	err = json.Unmarshal(body, &res)
	if err != nil {
		return "", fmt.Errorf("failed to unmarshal SSO configuration create response: %w", err)
	}

	return res.SSOConnectionID, nil
}

// DeleteSSOConfiguration - Delete tenant's sso configuration.
func (c *Client) DeleteSSOConfiguration(id string) error {
	payload, err := json.Marshal(SSOConfigurationDeleteRequest{SSOConnectionID: id})
	if err != nil {
		return fmt.Errorf("failed to marshal SSO configuration delete request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/delete_sso_configuration", c.HostURL), bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to create SSO configuration delete request: %w", err)
	}

	body, err := c.doRequest(req, nil)
	if err != nil {
		return fmt.Errorf("failed to delete SSO configuration delete request: %w", err)
	}

	if string(body) != "{}" {
		return errors.New(string(body))
	}

	return nil
}

// UpdateSSOConfiguration - Update SSO configuration.
func (c *Client) UpdateSSOConfiguration(updateRequest SSOConfigurationUpdateRequest) error {
	payload, err := json.Marshal(updateRequest)
	if err != nil {
		return fmt.Errorf("failed to marshal SSO configuration update request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/update_sso_configuration", c.HostURL), bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to create SSO configuration update request: %w", err)
	}

	body, err := c.doRequest(req, nil)
	if err != nil {
		return fmt.Errorf("failed to update SSO configuration update request: %w", err)
	}

	if string(body) != "{}" {
		return errors.New(string(body))
	}

	return nil
}

// GetSSOConfiguration - Return the current SSO configuration for the tenant if it matches the provided ID.
func (c *Client) GetSSOConfiguration(ssoConfigurationID string) (*SSOConfiguration, error) {
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/get_sso_configuration", c.HostURL), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create SSO configuration get request: %w", err)
	}

	body, err := c.doRequest(req, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get SSO configuration get request: %w", err)
	}

	res := SSOConfigurationGetResponseRequest{}
	err = json.Unmarshal(body, &res)
	if err != nil {
		return nil, fmt.Errorf("")
	}

	if res.SSOConfiguration == nil {
		return nil, fmt.Errorf("failed to parse SSO configuration get response: %w", err)
	}

	if res.SSOConfiguration.ID != ssoConfigurationID {
		return nil, ErrNotFound
	}

	return res.SSOConfiguration, nil
}

// GetSSOConfigurationWithoutID - Return the current SSO configuration for the tenant. Can return nil, nil if none exists.
func (c *Client) GetSSOConfigurationWithoutID() (*SSOConfiguration, error) {
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/get_sso_configuration", c.HostURL), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create SSO configuration get request: %w", err)
	}

	body, err := c.doRequest(req, nil)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get SSO configuration get request: %w", err)
	}

	res := SSOConfigurationGetResponseRequest{}
	err = json.Unmarshal(body, &res)
	if err != nil {
		return nil, fmt.Errorf("")
	}

	if res.SSOConfiguration == nil {
		return nil, fmt.Errorf("failed to parse SSO configuration get response: %w", err)
	}

	return res.SSOConfiguration, nil
}
