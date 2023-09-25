package api

import (
	"fmt"
	"io"
	"net/http"
	"time"
)

// HostURL - Default Warpstream URL.
const HostURL string = "https://api.prod.us-east-1.warpstream.com/api/v1"

// Client.
type Client struct {
	HostURL    string
	HTTPClient *http.Client
	Token      string
}

// NewClient.
func NewClient(host string, token *string) (*Client, error) {
	c := Client{
		HTTPClient: &http.Client{Timeout: 10 * time.Second},
		// Default Warpstream URL
		HostURL: HostURL,
	}

	if host != "" {
		c.HostURL = host
	}

	// If token not provided, return empty client
	if token == nil {
		return &c, nil
	}

	c.Token = *token
	return &c, nil
}

func (c *Client) doRequest(req *http.Request, authToken *string) ([]byte, error) {
	token := c.Token

	if authToken != nil {
		token = *authToken
	}

	req.Header.Set("warpstream-api-key", token)
	req.Header.Set("Content-Type", "application/json")

	res, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status: %d, body: %s", res.StatusCode, body)
	}

	return body, err
}
