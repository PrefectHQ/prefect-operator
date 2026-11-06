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
	"context"
	"encoding/json"
	"fmt"
	"maps"
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
	// UpdateDeployment updates an existing deployment; clearFields resets
	// previously-declared fields (e.g. the concurrency limit) instead of
	// leaving them untouched
	UpdateDeployment(ctx context.Context, id string, deployment *DeploymentSpec, clearFields []string) (*Deployment, error)
	// DeleteDeployment deletes a deployment
	DeleteDeployment(ctx context.Context, id string) error
	CreateDeploymentSchedules(ctx context.Context, deploymentID string, schedules []DeploymentSchedule) error
	UpdateDeploymentSchedule(ctx context.Context, deploymentID, scheduleID string, update DeploymentScheduleUpdate) error
	DeleteDeploymentSchedule(ctx context.Context, deploymentID, scheduleID string) error
	// CreateOrGetFlow creates a new flow or returns an existing one with the same name
	CreateOrGetFlow(ctx context.Context, flow *FlowSpec) (*Flow, error)
	// GetFlowByName retrieves a flow by name
	GetFlowByName(ctx context.Context, name string) (*Flow, error)
	// CreateWorkPool creates a new work pool
	CreateWorkPool(ctx context.Context, workPool *WorkPoolSpec) (*WorkPool, error)
	// GetWorkPool retrieves a work pool by name
	GetWorkPool(ctx context.Context, name string) (*WorkPool, error)
	// UpdateWorkPool updates an existing work pool
	UpdateWorkPool(ctx context.Context, name string, workPool *WorkPoolSpec) error
	// DeleteWorkPool deletes a work pool
	DeleteWorkPool(ctx context.Context, id string) error
	// GetWorkQueue retrieves a work queue within a pool by name; nil when absent
	GetWorkQueue(ctx context.Context, workPoolName, name string) (*WorkQueue, error)
	// CreateWorkQueue creates a work queue within a pool
	CreateWorkQueue(ctx context.Context, workPoolName string, queue *WorkQueueSpec) (*WorkQueue, error)
	// UpdateWorkQueue updates a work queue in place; clearFields resets
	// previously-declared fields to their create-time defaults
	UpdateWorkQueue(ctx context.Context, workPoolName, name string, queue *WorkQueueSpec, clearFields []string) error
	// DeleteWorkQueue deletes a work queue within a pool by name
	DeleteWorkQueue(ctx context.Context, workPoolName, name string) error
	// GetWorkerMetadata retrieves aggregate metadata for all worker types
	GetWorkerMetadata(ctx context.Context) (map[string]WorkerMetadata, error)
	// CreateAutomation creates a new automation
	CreateAutomation(ctx context.Context, automation *AutomationSpec) (*Automation, error)
	// GetAutomation retrieves an automation by ID
	GetAutomation(ctx context.Context, id string) (*Automation, error)
	// UpdateAutomation updates an existing automation by ID
	UpdateAutomation(ctx context.Context, id string, automation *AutomationSpec) (*Automation, error)
	// DeleteAutomation deletes an automation by ID
	DeleteAutomation(ctx context.Context, id string) error
	// FindDeploymentByName resolves a deployment by its name (tenant-wide) to its ID
	FindDeploymentByName(ctx context.Context, name string) (*Deployment, error)
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
func NewClientFromServerReference(serverRef *prefectiov1.PrefectServerReference, apiKey string, fallbackNamespace string, log logr.Logger) (*Client, error) {
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
		// Use the server's namespace, or fallback to the provided namespace
		namespace := serverRef.Namespace
		if namespace == "" {
			namespace = fallbackNamespace // Use provided fallback instead of hardcoded "default"
		}
		baseURL = serverRef.GetAPIURL(namespace)
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

	// Create client using the existing factory function, passing the namespace as fallback
	return NewClientFromServerReference(serverRef, apiKey, namespace, log)
}

// DeploymentSpec represents the request payload for creating/updating deployments
type DeploymentSpec struct {
	Name                    string               `json:"name"`
	FlowID                  string               `json:"flow_id"`
	Description             *string              `json:"description,omitempty"`
	Version                 *string              `json:"version,omitempty"`
	Tags                    []string             `json:"tags,omitempty"`
	Parameters              map[string]any       `json:"parameters,omitempty"`
	JobVariables            map[string]any       `json:"job_variables,omitempty"`
	WorkQueueName           *string              `json:"work_queue_name,omitempty"`
	WorkPoolName            *string              `json:"work_pool_name,omitempty"`
	Paused                  *bool                `json:"paused,omitempty"`
	Schedules               []DeploymentSchedule `json:"schedules,omitempty"`
	ConcurrencyLimit        *int                 `json:"concurrency_limit,omitempty"`
	GlobalConcurrencyLimits []string             `json:"global_concurrency_limits,omitempty"`
	Entrypoint              *string              `json:"entrypoint,omitempty"`
	Path                    *string              `json:"path,omitempty"`
	PullSteps               []map[string]any     `json:"pull_steps,omitempty"`
	ParameterOpenAPISchema  map[string]any       `json:"parameter_openapi_schema,omitempty"`
	EnforceParameterSchema  *bool                `json:"enforce_parameter_schema,omitempty"`
}

// Deployment represents a Prefect deployment
type Deployment struct {
	ID                      string                  `json:"id"`
	Created                 time.Time               `json:"created"`
	Updated                 time.Time               `json:"updated"`
	Name                    string                  `json:"name"`
	Version                 *string                 `json:"version"`
	Description             *string                 `json:"description"`
	FlowID                  string                  `json:"flow_id"`
	Paused                  bool                    `json:"paused"`
	Tags                    []string                `json:"tags"`
	Parameters              map[string]any          `json:"parameters"`
	JobVariables            map[string]any          `json:"job_variables"`
	WorkQueueName           *string                 `json:"work_queue_name"`
	WorkPoolName            *string                 `json:"work_pool_name"`
	Status                  string                  `json:"status"`
	Schedules               []DeploymentSchedule    `json:"schedules"`
	ConcurrencyLimit        *int                    `json:"concurrency_limit"`
	GlobalConcurrencyLimit  *GlobalConcurrencyLimit `json:"global_concurrency_limit,omitempty"`
	GlobalConcurrencyLimits []string                `json:"global_concurrency_limits"`
	Entrypoint              *string                 `json:"entrypoint"`
	Path                    *string                 `json:"path"`
	PullSteps               []map[string]any        `json:"pull_steps"`
	ParameterOpenAPISchema  map[string]any          `json:"parameter_openapi_schema"`
	EnforceParameterSchema  bool                    `json:"enforce_parameter_schema"`
}

// GlobalConcurrencyLimit carries the deployment's live concurrency limit;
// the deprecated top-level concurrency_limit response field is always null.
type GlobalConcurrencyLimit struct {
	Limit int `json:"limit"`
}

// Schedule represents a Prefect deployment schedule.
// Supports interval, cron, and rrule schedule types.
// Exactly one of Interval, Cron, or RRule should be specified.
type Schedule struct {
	ID string `json:"id,omitempty"`

	// === INTERVAL SCHEDULE FIELDS ===
	// Maps to: IntervalSchedule schema in Prefect API
	Interval   *float64   `json:"interval,omitempty"`    // seconds (required for interval)
	AnchorDate *time.Time `json:"anchor_date,omitempty"` // anchor date for interval schedules

	// === CRON SCHEDULE FIELDS ===
	// Maps to: CronSchedule schema in Prefect API
	Cron  *string `json:"cron,omitempty"`   // cron expression (required for cron)
	DayOr *bool   `json:"day_or,omitempty"` // day/day_of_week connection logic

	// === RRULE SCHEDULE FIELDS ===
	// Maps to: RRuleSchedule schema in Prefect API
	RRule *string `json:"rrule,omitempty"` // RFC 5545 RRULE string (required for rrule)

	// === COMMON FIELDS ===
	// Shared across all schedule types
	Timezone         *string `json:"timezone,omitempty"`
	Active           *bool   `json:"active,omitempty"`
	MaxScheduledRuns *int    `json:"max_scheduled_runs,omitempty"`
}

// DeploymentSchedule represents a deployment schedule in the Prefect API.
// This matches the DeploymentScheduleCreate schema which wraps the schedule object.
type DeploymentSchedule struct {
	ID   string  `json:"id,omitempty"`
	Slug *string `json:"slug,omitempty"`

	// Schedule contains the actual schedule configuration (interval, cron, or rrule)
	Schedule Schedule `json:"schedule"`

	// Active indicates if the schedule is active
	Active *bool `json:"active,omitempty"`

	// MaxScheduledRuns limits the number of scheduled runs
	MaxScheduledRuns *int `json:"max_scheduled_runs,omitempty"`

	// MaxActiveRuns limits the number of active runs
	MaxActiveRuns *int `json:"max_active_runs,omitempty"`

	// Catchup indicates if the worker should catch up on late runs
	Catchup *bool `json:"catchup,omitempty"`

	// Parameters are schedule-specific parameters
	Parameters map[string]any `json:"parameters,omitempty"`
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

type WorkPoolSpec struct {
	Name             string         `json:"name,omitempty"`
	Description      *string        `json:"description,omitempty"`
	Type             string         `json:"type,omitempty"`
	BaseJobTemplate  map[string]any `json:"base_job_template,omitempty"`
	IsPaused         *bool          `json:"is_paused,omitempty"`
	ConcurrencyLimit *int           `json:"concurrency_limit,omitempty"`
	// StorageConfiguration map[string]interface{} `json:"storage_configuration,omitempty"`
}

type WorkPool struct {
	ID               string         `json:"id"`
	Created          time.Time      `json:"created"`
	Updated          time.Time      `json:"updated"`
	Name             string         `json:"name"`
	Type             string         `json:"type"`
	Description      *string        `json:"description"`
	BaseJobTemplate  map[string]any `json:"base_job_template"`
	IsPaused         bool           `json:"is_paused"`
	ConcurrencyLimit *int           `json:"concurrency_limit"`
	Status           string         `json:"status"`
	DefaultQueueID   *string        `json:"default_queue_id"`
	// StorageConfiguration map[string]interface{} `json:"storage_configuration,omitempty"`
}

// CreateOrUpdateDeployment creates or updates a deployment using the Prefect API
func (c *Client) CreateOrUpdateDeployment(ctx context.Context, deployment *DeploymentSpec) (*Deployment, error) {
	var result Deployment
	if err := c.do(ctx, http.MethodPost, c.endpoint("/deployments/"), deployment, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetDeployment retrieves a deployment by ID. It returns (nil, nil) when the
// deployment does not exist.
func (c *Client) GetDeployment(ctx context.Context, id string) (*Deployment, error) {
	return getObject[Deployment](ctx, c, c.endpoint("/deployments/%s", id))
}

// GetDeploymentByName retrieves a deployment by name and flow ID
// This is a simplified implementation - in reality, you might need to use the deployments filter API
func (c *Client) GetDeploymentByName(ctx context.Context, name, flowID string) (*Deployment, error) {
	// TODO: Implement proper filtering API call
	// For now, this is a placeholder that would need to be implemented based on Prefect's filter API
	return nil, fmt.Errorf("GetDeploymentByName not yet implemented - use GetDeployment with ID")
}

// DeploymentFieldConcurrencyLimit is the CRD JSON name of the concurrency
// limit, used in clearFields and status.appliedFields.
const DeploymentFieldConcurrencyLimit = "concurrencyLimit"

// UpdateDeployment updates an existing deployment. Updates apply with
// exclude_unset semantics, so clearing a field needs its explicit null —
// clearFields names the fields to reset.
func (c *Client) UpdateDeployment(ctx context.Context, id string, deployment *DeploymentSpec, clearFields []string) (*Deployment, error) {
	raw, err := json.Marshal(deployment)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal deployment updates: %w", err)
	}
	var updateBody map[string]any
	if err := json.Unmarshal(raw, &updateBody); err != nil {
		return nil, fmt.Errorf("failed to prepare deployment update body: %w", err)
	}
	delete(updateBody, "name")
	delete(updateBody, "flow_id")
	for _, f := range clearFields {
		if f == DeploymentFieldConcurrencyLimit {
			updateBody["concurrency_limit"] = nil
		}
	}

	reqURL := c.endpoint("/deployments/%s", id)
	var result Deployment
	if err := c.do(ctx, http.MethodPatch, reqURL, updateBody, &result); err != nil {
		return nil, err
	}
	// Prefect answers the PATCH with 204 and no body, so read the deployment back.
	if result.ID == "" {
		if err := c.do(ctx, http.MethodGet, reqURL, nil, &result); err != nil {
			return nil, err
		}
	}
	return &result, nil
}

// DeleteDeployment deletes a deployment by ID
func (c *Client) DeleteDeployment(ctx context.Context, id string) error {
	return c.deleteObject(ctx, c.endpoint("/deployments/%s", id))
}

type DeploymentScheduleUpdate struct {
	Schedule         *Schedule      `json:"schedule,omitempty"`
	Active           *bool          `json:"active,omitempty"`
	MaxScheduledRuns *int           `json:"max_scheduled_runs,omitempty"`
	Parameters       map[string]any `json:"parameters,omitempty"`
}

// CreateDeploymentSchedules adds schedules to a deployment
func (c *Client) CreateDeploymentSchedules(ctx context.Context, deploymentID string, schedules []DeploymentSchedule) error {
	return c.do(ctx, http.MethodPost, c.endpoint("/deployments/%s/schedules", deploymentID), schedules, nil)
}

// UpdateDeploymentSchedule updates one of a deployment's schedules
func (c *Client) UpdateDeploymentSchedule(ctx context.Context, deploymentID, scheduleID string, update DeploymentScheduleUpdate) error {
	return c.do(ctx, http.MethodPatch, c.endpoint("/deployments/%s/schedules/%s", deploymentID, scheduleID), update, nil)
}

// DeleteDeploymentSchedule deletes one of a deployment's schedules
func (c *Client) DeleteDeploymentSchedule(ctx context.Context, deploymentID, scheduleID string) error {
	return c.deleteObject(ctx, c.endpoint("/deployments/%s/schedules/%s", deploymentID, scheduleID))
}

// CreateOrGetFlow creates a new flow or returns an existing one with the same name
func (c *Client) CreateOrGetFlow(ctx context.Context, flow *FlowSpec) (*Flow, error) {
	existingFlow, err := c.GetFlowByName(ctx, flow.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to check for existing flow: %w", err)
	}
	if existingFlow != nil {
		return existingFlow, nil
	}

	var result Flow
	if err := c.do(ctx, http.MethodPost, c.endpoint("/flows/"), flow, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetFlowByName retrieves a flow by name. It returns (nil, nil) when the flow
// does not exist.
func (c *Client) GetFlowByName(ctx context.Context, name string) (*Flow, error) {
	return getObject[Flow](ctx, c, c.endpoint("/flows/name/%s", name))
}

// GetWorkPool retrieves a work pool by name. It returns (nil, nil) when the
// pool does not exist.
func (c *Client) GetWorkPool(ctx context.Context, name string) (*WorkPool, error) {
	return getObject[WorkPool](ctx, c, c.endpoint("/work_pools/%s", name))
}

// CreateWorkPool creates a work pool
func (c *Client) CreateWorkPool(ctx context.Context, workPool *WorkPoolSpec) (*WorkPool, error) {
	var result WorkPool
	if err := c.do(ctx, http.MethodPost, c.endpoint("/work_pools/"), workPool, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// UpdateWorkPool updates an existing work pool
func (c *Client) UpdateWorkPool(ctx context.Context, name string, workPool *WorkPoolSpec) error {
	return c.do(ctx, http.MethodPatch, c.endpoint("/work_pools/%s", name), workPool, nil)
}

// DeleteWorkPool deletes a work pool by name
func (c *Client) DeleteWorkPool(ctx context.Context, name string) error {
	return c.deleteObject(ctx, c.endpoint("/work_pools/%s", name))
}

// isRunningInCluster checks if the operator is running in-cluster
func (c *Client) isRunningInCluster() bool {
	_, err := rest.InClusterConfig()
	return err == nil
}

type WorkerMetadata struct {
	Type                   string         `json:"type"`
	Description            string         `json:"description"`
	DisplayName            string         `json:"display_name"`
	DocumentationURL       string         `json:"documentation_url"`
	InstallCommand         string         `json:"install_command"`
	IsBeta                 bool           `json:"is_beta"`
	LogoURL                string         `json:"logo_url"`
	DefaultBaseJobTemplate map[string]any `json:"default_base_job_configuration"`
}

// GetWorkerMetadata retrieves aggregate metadata for all worker types. It
// returns (nil, nil) when the server has no worker metadata route.
func (c *Client) GetWorkerMetadata(ctx context.Context) (map[string]WorkerMetadata, error) {
	// The response is keyed by collection, then by worker type.
	result, err := getObject[map[string]map[string]WorkerMetadata](ctx, c,
		c.endpoint("/collections/views/aggregate-worker-metadata"))
	if result == nil || err != nil {
		return nil, err
	}

	metadata := map[string]WorkerMetadata{}
	for _, integration := range *result {
		maps.Copy(metadata, integration)
	}
	return metadata, nil
}
