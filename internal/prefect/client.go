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
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/go-logr/logr"
)

// PrefectClient defines the interface for interacting with the Prefect API
type PrefectClient interface {
	// CreateOrUpdateDeployment creates a new deployment or updates an existing one
	CreateOrUpdateDeployment(ctx context.Context, deployment *DeploymentSpec) (*Deployment, error)
	// GetDeployment retrieves a deployment by ID
	GetDeployment(ctx context.Context, deploymentID string) (*Deployment, error)
	// GetDeploymentByName retrieves a deployment by name and flow ID
	GetDeploymentByName(ctx context.Context, name, flowID string) (*Deployment, error)
	// UpdateDeployment updates an existing deployment
	UpdateDeployment(ctx context.Context, deploymentID string, updates *DeploymentSpec) error
	// DeleteDeployment deletes a deployment
	DeleteDeployment(ctx context.Context, deploymentID string) error
	// CreateOrGetFlow creates a new flow or returns an existing one with the same name
	CreateOrGetFlow(ctx context.Context, flow *FlowSpec) (*Flow, error)
	// GetFlowByName retrieves a flow by name
	GetFlowByName(ctx context.Context, name string) (*Flow, error)
}

// HTTPClient represents an HTTP client interface for testing
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// Client implements the PrefectClient interface
type Client struct {
	BaseURL    string
	APIKey     string
	HTTPClient HTTPClient
	log        logr.Logger
}

// NewClient creates a new Prefect API client
func NewClient(baseURL, apiKey string, log logr.Logger) *Client {
	return &Client{
		BaseURL: baseURL,
		APIKey:  apiKey,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		log: log,
	}
}

// DeploymentSpec represents the request payload for creating/updating deployments
type DeploymentSpec struct {
	Name                    string                   `json:"name"`
	FlowID                  string                   `json:"flow_id"`
	Description             *string                  `json:"description,omitempty"`
	Version                 *string                  `json:"version,omitempty"`
	Tags                    []string                 `json:"tags,omitempty"`
	Parameters              map[string]interface{}   `json:"parameters,omitempty"`
	JobVariables            map[string]interface{}   `json:"job_variables,omitempty"`
	WorkQueueName           *string                  `json:"work_queue_name,omitempty"`
	WorkPoolName            *string                  `json:"work_pool_name,omitempty"`
	Paused                  *bool                    `json:"paused,omitempty"`
	Schedules               []Schedule               `json:"schedules,omitempty"`
	ConcurrencyLimit        *int                     `json:"concurrency_limit,omitempty"`
	GlobalConcurrencyLimits []string                 `json:"global_concurrency_limits,omitempty"`
	Entrypoint              *string                  `json:"entrypoint,omitempty"`
	Path                    *string                  `json:"path,omitempty"`
	PullSteps               []map[string]interface{} `json:"pull_steps,omitempty"`
	ParameterOpenAPISchema  map[string]interface{}   `json:"parameter_openapi_schema,omitempty"`
	EnforceParameterSchema  *bool                    `json:"enforce_parameter_schema,omitempty"`
}

// Deployment represents a Prefect deployment
type Deployment struct {
	ID                      string                   `json:"id"`
	Created                 time.Time                `json:"created"`
	Updated                 time.Time                `json:"updated"`
	Name                    string                   `json:"name"`
	Version                 *string                  `json:"version"`
	Description             *string                  `json:"description"`
	FlowID                  string                   `json:"flow_id"`
	Paused                  bool                     `json:"paused"`
	Tags                    []string                 `json:"tags"`
	Parameters              map[string]interface{}   `json:"parameters"`
	JobVariables            map[string]interface{}   `json:"job_variables"`
	WorkQueueName           *string                  `json:"work_queue_name"`
	WorkPoolName            *string                  `json:"work_pool_name"`
	Status                  string                   `json:"status"`
	Schedules               []Schedule               `json:"schedules"`
	ConcurrencyLimit        *int                     `json:"concurrency_limit"`
	GlobalConcurrencyLimits []string                 `json:"global_concurrency_limits"`
	Entrypoint              *string                  `json:"entrypoint"`
	Path                    *string                  `json:"path"`
	PullSteps               []map[string]interface{} `json:"pull_steps"`
	ParameterOpenAPISchema  map[string]interface{}   `json:"parameter_openapi_schema"`
	EnforceParameterSchema  bool                     `json:"enforce_parameter_schema"`
}

// Schedule represents a deployment schedule
type Schedule struct {
	ID               string     `json:"id,omitempty"`
	Interval         *int       `json:"interval,omitempty"`
	AnchorDate       *time.Time `json:"anchor_date,omitempty"`
	Timezone         *string    `json:"timezone,omitempty"`
	Active           *bool      `json:"active,omitempty"`
	MaxScheduledRuns *int       `json:"max_scheduled_runs,omitempty"`
}

// FlowSpec represents the request payload for creating flows
type FlowSpec struct {
	Name   string            `json:"name"`
	Tags   []string          `json:"tags,omitempty"`
	Labels map[string]string `json:"labels,omitempty"`
}

// Flow represents a Prefect flow
type Flow struct {
	ID      string            `json:"id"`
	Created time.Time         `json:"created"`
	Updated time.Time         `json:"updated"`
	Name    string            `json:"name"`
	Tags    []string          `json:"tags"`
	Labels  map[string]string `json:"labels"`
}

// CreateOrUpdateDeployment creates or updates a deployment using the Prefect API
func (c *Client) CreateOrUpdateDeployment(ctx context.Context, deployment *DeploymentSpec) (*Deployment, error) {
	url := fmt.Sprintf("%s/api/deployments/", c.BaseURL)

	jsonData, err := json.Marshal(deployment)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal deployment spec: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.APIKey))
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			c.log.Error(err, "failed to close response body")
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result Deployment
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &result, nil
}

// GetDeployment retrieves a deployment by ID
func (c *Client) GetDeployment(ctx context.Context, deploymentID string) (*Deployment, error) {
	url := fmt.Sprintf("%s/api/deployments/%s", c.BaseURL, deploymentID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if c.APIKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.APIKey))
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			c.log.Error(err, "failed to close response body")
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil // Deployment doesn't exist
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result Deployment
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &result, nil
}

// GetDeploymentByName retrieves a deployment by name and flow ID
// This is a simplified implementation - in reality, you might need to use the deployments filter API
func (c *Client) GetDeploymentByName(ctx context.Context, name, flowID string) (*Deployment, error) {
	// TODO: Implement proper filtering API call
	// For now, this is a placeholder that would need to be implemented based on Prefect's filter API
	return nil, fmt.Errorf("GetDeploymentByName not yet implemented - use GetDeployment with ID")
}

// UpdateDeployment updates an existing deployment
func (c *Client) UpdateDeployment(ctx context.Context, deploymentID string, updates *DeploymentSpec) error {
	url := fmt.Sprintf("%s/api/deployments/%s", c.BaseURL, deploymentID)

	jsonData, err := json.Marshal(updates)
	if err != nil {
		return fmt.Errorf("failed to marshal deployment updates: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "PATCH", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.APIKey))
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			c.log.Error(err, "failed to close response body")
		}
	}()

	if resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// DeleteDeployment deletes a deployment
func (c *Client) DeleteDeployment(ctx context.Context, deploymentID string) error {
	url := fmt.Sprintf("%s/api/deployments/%s", c.BaseURL, deploymentID)

	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	if c.APIKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.APIKey))
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			c.log.Error(err, "failed to close response body")
		}
	}()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// CreateOrGetFlow creates a new flow or returns an existing one with the same name
func (c *Client) CreateOrGetFlow(ctx context.Context, flow *FlowSpec) (*Flow, error) {
	// First try to get the flow by name
	existingFlow, err := c.GetFlowByName(ctx, flow.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to check for existing flow: %w", err)
	}
	if existingFlow != nil {
		return existingFlow, nil
	}

	// If flow doesn't exist, create it
	url := fmt.Sprintf("%s/api/flows/", c.BaseURL)

	jsonData, err := json.Marshal(flow)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal flow spec: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.APIKey))
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			c.log.Error(err, "failed to close response body")
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result Flow
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &result, nil
}

// GetFlowByName retrieves a flow by name
func (c *Client) GetFlowByName(ctx context.Context, name string) (*Flow, error) {
	url := fmt.Sprintf("%s/api/flows/name/%s", c.BaseURL, name)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if c.APIKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.APIKey))
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			c.log.Error(err, "failed to close response body")
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil // Flow doesn't exist
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result Flow
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &result, nil
}
