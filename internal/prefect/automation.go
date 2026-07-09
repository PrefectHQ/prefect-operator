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
	"time"
)

// ErrAutomationNotFound is returned when an automation no longer exists in
// Prefect (e.g. it was deleted out-of-band). Callers can use errors.Is to
// distinguish this from other API errors and recreate rather than loop.
var ErrAutomationNotFound = errors.New("automation not found")

// AutomationSpec is the request payload for creating/updating an automation.
// Trigger and actions are kept as generic maps so the client stays agnostic to
// the (large, evolving) Prefect trigger/action schema; the convert layer builds
// them from the typed PrefectAutomation.
type AutomationSpec struct {
	Name             string           `json:"name"`
	Description      string           `json:"description"`
	Enabled          *bool            `json:"enabled,omitempty"`
	Trigger          map[string]any   `json:"trigger"`
	Actions          []map[string]any `json:"actions"`
	ActionsOnTrigger []map[string]any `json:"actions_on_trigger,omitempty"`
	ActionsOnResolve []map[string]any `json:"actions_on_resolve,omitempty"`
}

// Automation is a Prefect automation as returned by the API.
type Automation struct {
	ID               string           `json:"id"`
	Created          time.Time        `json:"created"`
	Updated          time.Time        `json:"updated"`
	Name             string           `json:"name"`
	Description      string           `json:"description"`
	Enabled          bool             `json:"enabled"`
	Trigger          map[string]any   `json:"trigger"`
	Actions          []map[string]any `json:"actions"`
	ActionsOnTrigger []map[string]any `json:"actions_on_trigger"`
	ActionsOnResolve []map[string]any `json:"actions_on_resolve"`
}

func (c *Client) setHeaders(req *http.Request, withContentType bool) {
	if withContentType {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.APIKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.APIKey))
	}
}

// CreateAutomation creates a new automation via POST /automations/.
func (c *Client) CreateAutomation(ctx context.Context, automation *AutomationSpec) (*Automation, error) {
	url := fmt.Sprintf("%s/automations/", c.BaseURL)
	c.log.V(1).Info("Creating automation", "url", url, "automation", automation.Name)

	jsonData, err := json.Marshal(automation)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal automation: %w", err)
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

	var result Automation
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	c.log.V(1).Info("Automation created successfully", "automationId", result.ID)
	return &result, nil
}

// GetAutomation retrieves an automation by ID via GET /automations/{id}.
func (c *Client) GetAutomation(ctx context.Context, id string) (*Automation, error) {
	url := fmt.Sprintf("%s/automations/%s", c.BaseURL, id)
	c.log.V(1).Info("Getting automation", "url", url, "automationId", id)

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
	if !isSuccessStatusCode(resp.StatusCode) {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result Automation
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	return &result, nil
}

// UpdateAutomation updates an automation via PUT /automations/{id} (full replace,
// matching the declarative desired state). Returns the freshly-read automation.
func (c *Client) UpdateAutomation(ctx context.Context, id string, automation *AutomationSpec) (*Automation, error) {
	url := fmt.Sprintf("%s/automations/%s", c.BaseURL, id)
	c.log.V(1).Info("Updating automation", "url", url, "automationId", id)

	jsonData, err := json.Marshal(automation)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal automation updates: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewBuffer(jsonData))
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
	// A 404 means the automation was deleted out-of-band; signal recreate.
	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrAutomationNotFound
	}
	if !isSuccessStatusCode(resp.StatusCode) {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// PUT /automations/{id} returns 204 with no body; re-read to return current state.
	return c.GetAutomation(ctx, id)
}

// FindDeploymentByName resolves a deployment by name (tenant-wide) via
// POST /deployments/filter. Deployment names are only unique per flow (the
// canonical reference is "{flow}/{deployment}"), so a bare name can match more
// than one deployment. Errors if no deployment matches, and errors if the name
// is ambiguous (>1 match) rather than silently resolving to an arbitrary one.
func (c *Client) FindDeploymentByName(ctx context.Context, name string) (*Deployment, error) {
	url := fmt.Sprintf("%s/deployments/filter", c.BaseURL)
	c.log.V(1).Info("Finding deployment by name", "url", url, "name", name)

	filter := map[string]any{
		"deployments": map[string]any{
			keyName: map[string]any{"any_": []string{name}},
		},
		"limit": 2,
	}
	jsonData, err := json.Marshal(filter)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal deployment filter: %w", err)
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

	var results []Deployment
	if err := json.Unmarshal(body, &results); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("no deployment found with name %q", name)
	}
	if len(results) > 1 {
		return nil, fmt.Errorf("deployment name %q is ambiguous: it matches multiple deployments (names are only unique per flow); use a flow-qualified reference", name)
	}
	return &results[0], nil
}

// DeleteAutomation deletes an automation via DELETE /automations/{id}.
func (c *Client) DeleteAutomation(ctx context.Context, id string) error {
	url := fmt.Sprintf("%s/automations/%s", c.BaseURL, id)
	c.log.V(1).Info("Deleting automation", "url", url, "automationId", id)

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
