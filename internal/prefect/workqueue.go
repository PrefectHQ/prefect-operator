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
)

// ErrWorkQueueNotFound is returned when a work queue no longer exists in
// Prefect (e.g. it was deleted out-of-band). Callers can use errors.Is to
// distinguish this from other API errors and recreate rather than loop.
var ErrWorkQueueNotFound = errors.New("work queue not found")

// WorkQueueSpec is the create/update payload for a work queue.
type WorkQueueSpec struct {
	Name             string  `json:"name"`
	Description      *string `json:"description,omitempty"`
	IsPaused         *bool   `json:"is_paused,omitempty"`
	ConcurrencyLimit *int32  `json:"concurrency_limit,omitempty"`
	Priority         *int32  `json:"priority,omitempty"`
}

// WorkQueue is a work queue as returned by the Prefect API.
type WorkQueue struct {
	ID               string  `json:"id"`
	Name             string  `json:"name"`
	Description      *string `json:"description,omitempty"`
	IsPaused         *bool   `json:"is_paused,omitempty"`
	ConcurrencyLimit *int32  `json:"concurrency_limit,omitempty"`
	Priority         *int32  `json:"priority,omitempty"`
	WorkPoolName     string  `json:"work_pool_name,omitempty"`
}

// GetWorkQueue retrieves a work queue within a pool by name via
// GET /work_pools/{pool}/queues/{name}. Returns (nil, nil) when the queue does
// not exist, matching GetWorkPool's contract.
func (c *Client) GetWorkQueue(ctx context.Context, workPoolName, name string) (*WorkQueue, error) {
	url := fmt.Sprintf("%s/work_pools/%s/queues/%s", c.BaseURL, workPoolName, name)
	c.log.V(1).Info("Getting work queue", "url", url, "workPool", workPoolName, "name", name)
	return c.getWorkQueue(ctx, url)
}

// GetWorkQueueByID retrieves a work queue via GET /work_queues/{id}.
// Returns (nil, nil) when the queue does not exist.
func (c *Client) GetWorkQueueByID(ctx context.Context, id string) (*WorkQueue, error) {
	url := fmt.Sprintf("%s/work_queues/%s", c.BaseURL, id)
	c.log.V(1).Info("Getting work queue", "url", url, "workQueueId", id)
	return c.getWorkQueue(ctx, url)
}

func (c *Client) getWorkQueue(ctx context.Context, url string) (*WorkQueue, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	c.setHeaders(req, false)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if !isSuccessStatusCode(resp.StatusCode) {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result WorkQueue
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	return &result, nil
}

// CreateWorkQueue creates a work queue via POST /work_pools/{pool}/queues.
func (c *Client) CreateWorkQueue(ctx context.Context, workPoolName string, queue *WorkQueueSpec) (*WorkQueue, error) {
	url := fmt.Sprintf("%s/work_pools/%s/queues", c.BaseURL, workPoolName)
	c.log.V(1).Info("Creating work queue", "url", url, "workPool", workPoolName, "name", queue.Name)

	jsonData, err := json.Marshal(queue)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal work queue: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	c.setHeaders(req, true)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	if !isSuccessStatusCode(resp.StatusCode) {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result WorkQueue
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	c.log.V(1).Info("Work queue created successfully", "workQueueId", result.ID)
	return &result, nil
}

// UpdateWorkQueue updates a work queue via PATCH /work_queues/{id}. Fields left
// unset in the spec are omitted from the payload, so PATCH semantics leave them
// unchanged in Prefect.
func (c *Client) UpdateWorkQueue(ctx context.Context, id string, queue *WorkQueueSpec) error {
	url := fmt.Sprintf("%s/work_queues/%s", c.BaseURL, id)
	c.log.V(1).Info("Updating work queue", "url", url, "workQueueId", id)

	jsonData, err := json.Marshal(queue)
	if err != nil {
		return fmt.Errorf("failed to marshal work queue updates: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "PATCH", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	c.setHeaders(req, true)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}
	// A 404 means the queue was deleted out-of-band; signal recreate.
	if resp.StatusCode == http.StatusNotFound {
		return ErrWorkQueueNotFound
	}
	if !isSuccessStatusCode(resp.StatusCode) {
		return fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// DeleteWorkQueue deletes a work queue via DELETE /work_queues/{id}.
func (c *Client) DeleteWorkQueue(ctx context.Context, id string) error {
	url := fmt.Sprintf("%s/work_queues/%s", c.BaseURL, id)
	c.log.V(1).Info("Deleting work queue", "url", url, "workQueueId", id)

	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	c.setHeaders(req, false)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}
	// Treat 404 as already-deleted (idempotent cleanup).
	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	if !isSuccessStatusCode(resp.StatusCode) {
		return fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}
