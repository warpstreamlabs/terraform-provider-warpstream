package api

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"os"
	"strings"
	"time"

	"github.com/hashicorp/go-retryablehttp"
)

// HostURL - Default Warpstream URL.
const HostURL string = "https://api.prod.us-east-1.warpstream.com/api/v1"

// Client.
type Client struct {
	HostURL    string
	HTTPClient *retryablehttp.Client
	Token      string
}

// NewClient.
func NewClient(host string, token *string) (*Client, error) {
	retryClient := retryablehttp.NewClient()
	retryClient.RetryMax = 5
	retryClient.StandardClient().Timeout = 10 * time.Second
	c := Client{
		HTTPClient: retryClient,
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
	d, err := httputil.DumpRequest(req, true)
	if err != nil {
		return nil, fmt.Errorf("internal client error: %s", err)
	}
	log.Printf("%q\n", d)

	token := c.Token

	if authToken != nil {
		token = *authToken
	}

	req.Header.Set("warpstream-api-key", token)
	req.Header.Set("Content-Type", "application/json")

	retryReq, err := retryablehttp.FromRequest(req)
	if err != nil {
		return nil, err
	}

	res, err := c.HTTPClient.Do(retryReq)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	log.Printf("%q\n", body)

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status: %d, body: %s", res.StatusCode, body)
	}

	if strings.Contains(string(body), "internal server error") {
		return nil, fmt.Errorf("status: 500, body: internal server error")
	}

	return body, err
}

func NewClientDefault() (*Client, error) {
	token := os.Getenv("WARPSTREAM_API_KEY")
	host := os.Getenv("WARPSTREAM_API_URL")

	return NewClient(host, &token)
}
