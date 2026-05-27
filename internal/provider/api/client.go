package api

import (
	"context"
	"errors"
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

var ErrNotFound = errors.New("Resource Not Found")

// HostURL - Default Warpstream URL.
const HostURL string = "https://api.prod.us-east-1.warpstream.com/api/v1"

// Client.
type Client struct {
	HostURL    string
	HTTPClient *retryablehttp.Client
	Token      string
	aclsCache  aclsCache
}

// NewClient.
func NewClient(host string, token *string) (*Client, error) {
	retryClient := retryablehttp.NewClient()
	retryClient.RetryMax = 5
	retryClient.ErrorHandler = func(resp *http.Response, err error, numTries int) (*http.Response, error) {
		if resp != nil && resp.Request != nil {
			if err == nil {
				err = fmt.Errorf("%s %s giving up after %d attempt(s)", resp.Request.Method, resp.Request.URL, numTries)
			} else {
				err = fmt.Errorf("%s %s giving up after %d attempt(s): %w", resp.Request.Method, resp.Request.URL, numTries, err)
			}
		} else if err == nil {
			err = fmt.Errorf("giving up after %d attempt(s)", numTries)
		} else {
			err = fmt.Errorf("giving up after %d attempt(s): %w", numTries, err)
		}
		return resp, err
	}
	retryClient.HTTPClient.Timeout = 30 * time.Second
	retryClient.CheckRetry = checkRetryPolicy
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

// checkRetryPolicy extends the default retry policy to also retry on HTTP 499
// (client-side cancellation). The WarpStream API returns 499 with
// "context_canceled" when the server detects the client connection was dropped
// before the response could be sent. This is transient and safe to retry.
func checkRetryPolicy(ctx context.Context, resp *http.Response, err error) (bool, error) {
	shouldRetry, checkErr := retryablehttp.DefaultRetryPolicy(ctx, resp, err)
	if shouldRetry {
		return true, checkErr
	}
	if ctx.Err() != nil {
		return false, ctx.Err()
	}
	if resp != nil && resp.StatusCode == 499 {
		return true, nil
	}
	return false, checkErr
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
		if res != nil {
			defer res.Body.Close()
			body, readErr := io.ReadAll(res.Body)
			if readErr == nil {
				return nil, fmt.Errorf("%w: status: %d, body: %s", err, res.StatusCode, body)
			}
		}
		return nil, err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	log.Printf("%q\n", body)

	if res.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}

	if res.StatusCode == http.StatusUnauthorized {
		errMsg := fmt.Sprintf("status: 401, body: %s", body)
		if strings.Contains(string(body), "invalid_api_key") {
			errMsg = fmt.Sprintf("%s\n\n Did you pass an authentication token to the provider?", errMsg)
		}
		return nil, errors.New(errMsg)
	}

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
