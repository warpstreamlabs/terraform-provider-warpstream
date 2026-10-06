package api

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"strings"
)

// ErrAmbiguous marks a failed request that may still have been applied by the server: a transport
// error, a 499, or a 5xx.
var ErrAmbiguous = errors.New("the request may have been applied by the server")

// doRequestOnce is doRequest without retries, for v2 creates whose secret is only in the response and
// so must not be repeated blindly. A failure where the server may have acted wraps ErrAmbiguous.
func (c *Client) doRequestOnce(req *http.Request) ([]byte, error) {
	d, err := httputil.DumpRequest(req, true)
	if err != nil {
		return nil, fmt.Errorf("internal client error: %s", err)
	}
	log.Printf("%q\n", d)

	req.Header.Set("warpstream-api-key", c.Token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", c.UserAgent)

	res, err := c.HTTPClient.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrAmbiguous, err)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrAmbiguous, err)
	}
	log.Printf("%q\n", body)

	switch {
	case res.StatusCode >= http.StatusInternalServerError || res.StatusCode == 499:
		return nil, fmt.Errorf("%w: status: %d, body: %s", ErrAmbiguous, res.StatusCode, body)
	case res.StatusCode == http.StatusNotFound:
		return nil, ErrNotFound
	case res.StatusCode == http.StatusUnauthorized:
		errMsg := fmt.Sprintf("status: 401, body: %s", body)
		if strings.Contains(string(body), "invalid_api_key") {
			errMsg = fmt.Sprintf("%s\n\n Did you pass an authentication token to the provider?", errMsg)
		}
		return nil, errors.New(errMsg)
	case res.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("status: %d, body: %s", res.StatusCode, body)
	case strings.Contains(string(body), "internal server error"):
		return nil, fmt.Errorf("%w: status: 500, body: internal server error", ErrAmbiguous)
	}
	return body, nil
}
