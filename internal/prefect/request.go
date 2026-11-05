package prefect

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// ErrNotFound matches the error for a Prefect API response with status 404.
//
// Client methods answer a 404 in one of three ways, by operation:
//   - Get methods return (nil, nil).
//   - Delete methods return nil, because the object is already gone.
//   - Every other method returns an error that matches ErrNotFound with
//     errors.Is. For an update, the object does not exist. For a create, the
//     object it belongs to does not exist, such as the pool of a work queue.
var ErrNotFound = errors.New("not found")

// apiError is the error for a Prefect API response with a non-2xx status.
type apiError struct {
	statusCode int
	body       string
}

func (e *apiError) Error() string {
	return fmt.Sprintf("API request failed with status %d: %s", e.statusCode, e.body)
}

// Is reports whether a 404 response is matched against ErrNotFound.
func (e *apiError) Is(target error) bool {
	return target == ErrNotFound && e.statusCode == http.StatusNotFound
}

// getObject sends a GET to reqURL and decodes the response into a new T. It
// returns (nil, nil) when Prefect answers 404.
func getObject[T any](ctx context.Context, c *Client, reqURL string) (*T, error) {
	var result T
	err := c.do(ctx, http.MethodGet, reqURL, nil, &result)
	if errors.Is(err, ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// deleteObject sends a DELETE to reqURL. A 404 means the object is already
// gone, so deleteObject returns nil for it.
func (c *Client) deleteObject(ctx context.Context, reqURL string) error {
	err := c.do(ctx, http.MethodDelete, reqURL, nil, nil)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	return err
}

// isSuccessStatusCode returns true if the HTTP status code indicates success (2xx range)
func isSuccessStatusCode(statusCode int) bool {
	return 200 <= statusCode && statusCode < 300
}

// endpoint returns the URL for an API path. It path-escapes each argument
// before substituting it into format: Prefect names may contain spaces, '#',
// and '?' (types/names.py bans only / % & > <), and unescaped they would start
// a fragment or a query string.
func (c *Client) endpoint(format string, args ...string) string {
	escaped := make([]any, len(args))
	for i, arg := range args {
		escaped[i] = url.PathEscape(arg)
	}
	return c.BaseURL + fmt.Sprintf(format, escaped...)
}

// do sends a request to the Prefect API. When in is non-nil, do sends it as
// the JSON body. When out is non-nil and the response has a body, do decodes
// the body into out. A response with a non-2xx status returns an *apiError.
func (c *Client) do(ctx context.Context, method, reqURL string, in, out any) error {
	var body io.Reader
	if in != nil {
		data, err := json.Marshal(in)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		body = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, reqURL, body)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}
	c.log.V(1).Info("Prefect API request", "method", method, "url", reqURL, "status", resp.StatusCode)

	if !isSuccessStatusCode(resp.StatusCode) {
		return &apiError{statusCode: resp.StatusCode, body: string(respBody)}
	}
	if out != nil && len(bytes.TrimSpace(respBody)) > 0 {
		if err := json.Unmarshal(respBody, out); err != nil {
			return fmt.Errorf("failed to unmarshal response: %w", err)
		}
	}
	return nil
}
