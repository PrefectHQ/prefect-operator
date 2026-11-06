/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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

// apiError is the error for a Prefect API response with a non-2xx status.
type apiError struct {
	statusCode int
	body       string
}

func (e *apiError) Error() string {
	return fmt.Sprintf("API request failed with status %d: %s", e.statusCode, e.body)
}

// isNotFound reports whether err is a Prefect API response with status 404.
func isNotFound(err error) bool {
	var apiErr *apiError
	return errors.As(err, &apiErr) && apiErr.statusCode == http.StatusNotFound
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
