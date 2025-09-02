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

	prefectiov1 "github.com/PrefectHQ/prefect-operator/api/v1"
	"github.com/PrefectHQ/prefect-operator/internal/portforward"
	"github.com/go-logr/logr"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// PrefectClient defines the interface for interacting with the Prefect API
type PrefectClient interface {
	// CreateOrUpdateDeployment creates a new deployment or updates an existing one
	CreateOrUpdateDeployment(ctx context.Context, deployment *DeploymentSpec) (*Deployment, error)
	// GetDeployment retrieves a deployment by ID
	GetDeployment(ctx context.Context, id string) (*Deployment, error)
	// GetDeploymentByName retrieves a deployment by name and flow ID
	GetDeploymentByName(ctx context.Context, name, flowID string) (*Deployment, error)
	// UpdateDeployment updates an existing deployment
	UpdateDeployment(ctx context.Context, id string, deployment *DeploymentSpec) (*Deployment, error)
	// DeleteDeployment deletes a deployment
	DeleteDeployment(ctx context.Context, id string) error
	// CreateOrGetFlow creates a new flow or returns an existing one with the same name
	CreateOrGetFlow(ctx context.Context, flow *FlowSpec) (*Flow, error)
	// GetFlowByName retrieves a flow by name
	GetFlowByName(ctx context.Context, name string) (*Flow, error)
	// DeleteWorkPool deletes a work pool
	DeleteWorkPool(ctx context.Context, id string) error
}

// HTTPClient represents an HTTP client interface for testing
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// Client implements the PrefectClient interface
type Client struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
	log        logr.Logger
	// PortForwardClient is used to port-forward to the Prefect server when running outside the cluster
	PortForwardClient portforward.PortForwarder
}

// NewClient creates a new Prefect API client
func NewClient(baseURL, apiKey string, log logr.Logger) *Client {
	return &Client{
		BaseURL:    baseURL,
		APIKey:     apiKey,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
		log:        log,
	}
}

// NewClientFromServerReference creates a new PrefectClient from a PrefectServerReference
func NewClientFromServerReference(serverRef *prefectiov1.PrefectServerReference, apiKey string, log logr.Logger) (*Client, error) {
	// Create a base client first to check if we're running in cluster
	baseClient := NewClient("", apiKey, log)

	// Determine if we need port-forwarding
	needsPortForwarding := !baseClient.isRunningInCluster() && serverRef.IsInCluster()

	// Set the base URL based on whether we need port-forwarding
	var baseURL string
	if needsPortForwarding {
		// When port-forwarding, use localhost with port 14200
		baseURL = "http://localhost:14200/api"
		log.V(1).Info("Using localhost for port-forwarding", "url", baseURL)
	} else {
		// Use the server's namespace as fallback if not specified
		fallbackNamespace := serverRef.Namespace
		if fallbackNamespace == "" {
			fallbackNamespace = "default" // Default to "default" namespace if not specified
		}
		baseURL = serverRef.GetAPIURL(fallbackNamespace)
		log.V(1).Info("Using in-cluster URL", "url", baseURL)
	}

	client := NewClient(baseURL, apiKey, log)

	if needsPortForwarding {
		// Initialize port-forwarding client with local port 14200 and remote port 4200
		portForwardClient := portforward.NewKubectlPortForwarder(serverRef.Namespace, serverRef.Name, 14200, 4200)
		client.PortForwardClient = portForwardClient

		// Set up port-forwarding
		stopCh := make(chan struct{}, 1)
		readyCh := make(chan struct{}, 1)
		errCh := make(chan error, 1)

		go func() {
			errCh <- client.PortForwardClient.ForwardPorts(stopCh, readyCh)
		}()

		select {
		case err := <-errCh:
			return nil, err
		case <-readyCh:
			log.V(1).Info("Port-forwarding is ready")
		}
	}

	return client, nil
}

// NewClientFromK8s creates a new PrefectClient from a PrefectServerReference and Kubernetes client
// This combines API key retrieval with client creation for convenience
func NewClientFromK8s(ctx context.Context, serverRef *prefectiov1.PrefectServerReference, k8sClient client.Client, namespace string, log logr.Logger) (*Client, error) {
	// Get the API key from the server reference
	apiKey, err := serverRef.GetAPIKey(ctx, k8sClient, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to get API key: %w", err)
	}

	// Create client using the existing factory function
	return NewClientFromServerReference(serverRef, apiKey, log)
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
	url := fmt.Sprintf("%s/deployments/", c.BaseURL)
	c.log.V(1).Info("Creating or updating deployment", "url", url, "deployment", deployment.Name)

	jsonData, err := json.Marshal(deployment)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal deployment: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.APIKey))

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
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

	c.log.V(1).Info("Deployment created or updated successfully", "deploymentId", result.ID)
	return &result, nil
}

// GetDeployment retrieves a deployment by ID
func (c *Client) GetDeployment(ctx context.Context, id string) (*Deployment, error) {
	url := fmt.Sprintf("%s/deployments/%s", c.BaseURL, id)
	c.log.V(1).Info("Getting deployment", "url", url, "deploymentId", id)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.APIKey))

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
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

	c.log.V(1).Info("Deployment retrieved successfully", "deploymentId", result.ID)
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
func (c *Client) UpdateDeployment(ctx context.Context, id string, deployment *DeploymentSpec) (*Deployment, error) {
	url := fmt.Sprintf("%s/deployments/%s", c.BaseURL, id)
	c.log.V(1).Info("Updating deployment", "url", url, "deploymentId", id)

	jsonData, err := json.Marshal(deployment)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal deployment updates: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "PATCH", url, bytes.NewBuffer(jsonData))
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

	c.log.V(1).Info("Deployment updated successfully", "deploymentId", id)
	return &result, nil
}

// DeleteDeployment deletes a deployment
func (c *Client) DeleteDeployment(ctx context.Context, id string) error {
	url := fmt.Sprintf("%s/deployments/%s", c.BaseURL, id)
	c.log.V(1).Info("Deleting deployment", "url", url, "deploymentId", id)

	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.APIKey))

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	c.log.V(1).Info("Deployment deleted successfully", "deploymentId", id)
	return nil
}

// CreateOrGetFlow creates a new flow or returns an existing one with the same name
func (c *Client) CreateOrGetFlow(ctx context.Context, flow *FlowSpec) (*Flow, error) {
	// Check if flow already exists
	existingFlow, err := c.GetFlowByName(ctx, flow.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to check for existing flow: %w", err)
	}
	if existingFlow != nil {
		c.log.V(1).Info("Flow already exists, returning existing flow", "flowName", flow.Name, "flowId", existingFlow.ID)
		return existingFlow, nil
	}

	// If flow doesn't exist, create it
	url := fmt.Sprintf("%s/flows/", c.BaseURL)
	c.log.V(1).Info("Creating new flow", "url", url, "flowName", flow.Name)

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

	c.log.V(1).Info("Flow created successfully", "flowName", flow.Name, "flowId", result.ID)
	return &result, nil
}

// GetFlowByName retrieves a flow by name
func (c *Client) GetFlowByName(ctx context.Context, name string) (*Flow, error) {
	url := fmt.Sprintf("%s/flows/name/%s", c.BaseURL, name)
	c.log.V(1).Info("Getting flow by name", "url", url, "flowName", name)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
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

	c.log.V(1).Info("Flow retrieved successfully", "flowName", name, "flowId", result.ID)
	return &result, nil
}

// DeleteWorkPool deletes a work pool by ID
func (c *Client) DeleteWorkPool(ctx context.Context, name string) error {
	url := fmt.Sprintf("%s/work_pools/%s", c.BaseURL, name)
	c.log.V(1).Info("Deleting work pool", "url", url, "workPoolName", name)

	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.APIKey))

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	c.log.V(1).Info("Work pool deleted successfully", "workPoolName", name)
	return nil
}

// isRunningInCluster checks if the operator is running in-cluster
func (c *Client) isRunningInCluster() bool {
	_, err := rest.InClusterConfig()
	return err == nil
}
