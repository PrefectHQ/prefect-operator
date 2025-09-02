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
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// MockClient implements PrefectClient for testing
type MockClient struct {
	mu          sync.RWMutex
	deployments map[string]*Deployment
	flows       map[string]*Flow

	// Test configuration
	ShouldFailCreate     bool
	ShouldFailUpdate     bool
	ShouldFailGet        bool
	ShouldFailDelete     bool
	ShouldFailFlowCreate bool
	FailureMessage       string
}

// NewMockClient creates a new mock Prefect client
func NewMockClient() *MockClient {
	return &MockClient{
		deployments: make(map[string]*Deployment),
		flows:       make(map[string]*Flow),
	}
}

// CreateOrUpdateDeployment creates or updates a deployment in the mock store
func (m *MockClient) CreateOrUpdateDeployment(ctx context.Context, deployment *DeploymentSpec) (*Deployment, error) {
	if m.ShouldFailCreate {
		return nil, fmt.Errorf("mock error: %s", m.FailureMessage)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Find existing deployment by name and flow_id
	var existing *Deployment
	for _, d := range m.deployments {
		if d.Name == deployment.Name && d.FlowID == deployment.FlowID {
			existing = d
			break
		}
	}

	now := time.Now()

	if existing != nil {
		// Update existing deployment
		existing.Updated = now
		existing.Version = deployment.Version
		existing.Description = deployment.Description
		existing.Tags = deployment.Tags
		existing.Parameters = deployment.Parameters
		existing.JobVariables = deployment.JobVariables
		existing.WorkQueueName = deployment.WorkQueueName
		existing.WorkPoolName = deployment.WorkPoolName
		if deployment.Paused != nil {
			existing.Paused = *deployment.Paused
		}
		existing.Schedules = deployment.Schedules
		existing.ConcurrencyLimit = deployment.ConcurrencyLimit
		existing.GlobalConcurrencyLimits = deployment.GlobalConcurrencyLimits
		existing.Entrypoint = deployment.Entrypoint
		existing.Path = deployment.Path
		existing.PullSteps = deployment.PullSteps
		existing.ParameterOpenAPISchema = deployment.ParameterOpenAPISchema
		if deployment.EnforceParameterSchema != nil {
			existing.EnforceParameterSchema = *deployment.EnforceParameterSchema
		}

		return existing, nil
	}

	// Create new deployment
	newDeployment := &Deployment{
		ID:                      uuid.New().String(),
		Created:                 now,
		Updated:                 now,
		Name:                    deployment.Name,
		Version:                 deployment.Version,
		Description:             deployment.Description,
		FlowID:                  deployment.FlowID,
		Paused:                  deployment.Paused != nil && *deployment.Paused,
		Tags:                    deployment.Tags,
		Parameters:              deployment.Parameters,
		JobVariables:            deployment.JobVariables,
		WorkQueueName:           deployment.WorkQueueName,
		WorkPoolName:            deployment.WorkPoolName,
		Status:                  "READY", // Default status
		Schedules:               deployment.Schedules,
		ConcurrencyLimit:        deployment.ConcurrencyLimit,
		GlobalConcurrencyLimits: deployment.GlobalConcurrencyLimits,
		Entrypoint:              deployment.Entrypoint,
		Path:                    deployment.Path,
		PullSteps:               deployment.PullSteps,
		ParameterOpenAPISchema:  deployment.ParameterOpenAPISchema,
		EnforceParameterSchema:  deployment.EnforceParameterSchema != nil && *deployment.EnforceParameterSchema,
	}

	// Ensure slices are not nil
	if newDeployment.Tags == nil {
		newDeployment.Tags = []string{}
	}
	if newDeployment.Parameters == nil {
		newDeployment.Parameters = make(map[string]interface{})
	}
	if newDeployment.JobVariables == nil {
		newDeployment.JobVariables = make(map[string]interface{})
	}
	if newDeployment.Schedules == nil {
		newDeployment.Schedules = []Schedule{}
	}
	if newDeployment.GlobalConcurrencyLimits == nil {
		newDeployment.GlobalConcurrencyLimits = []string{}
	}
	if newDeployment.PullSteps == nil {
		newDeployment.PullSteps = []map[string]interface{}{}
	}
	if newDeployment.ParameterOpenAPISchema == nil {
		newDeployment.ParameterOpenAPISchema = make(map[string]interface{})
	}

	m.deployments[newDeployment.ID] = newDeployment
	return newDeployment, nil
}

// GetDeployment retrieves a deployment by ID from the mock store
func (m *MockClient) GetDeployment(ctx context.Context, deploymentID string) (*Deployment, error) {
	if m.ShouldFailGet {
		return nil, fmt.Errorf("mock error: %s", m.FailureMessage)
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	deployment, exists := m.deployments[deploymentID]
	if !exists {
		return nil, nil // Not found
	}

	// Return a copy to avoid race conditions
	return m.copyDeployment(deployment), nil
}

// GetDeploymentByName retrieves a deployment by name and flow ID from the mock store
func (m *MockClient) GetDeploymentByName(ctx context.Context, name, flowID string) (*Deployment, error) {
	if m.ShouldFailGet {
		return nil, fmt.Errorf("mock error: %s", m.FailureMessage)
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, deployment := range m.deployments {
		if deployment.Name == name && deployment.FlowID == flowID {
			return m.copyDeployment(deployment), nil
		}
	}

	return nil, nil // Not found
}

// UpdateDeployment updates an existing deployment in the mock store
func (m *MockClient) UpdateDeployment(ctx context.Context, id string, deployment *DeploymentSpec) (*Deployment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.deployments[id]
	if !ok {
		return nil, fmt.Errorf("deployment not found")
	}

	existing.Updated = time.Now()
	existing.Version = deployment.Version
	existing.Description = deployment.Description
	existing.Tags = deployment.Tags
	existing.Parameters = deployment.Parameters
	existing.JobVariables = deployment.JobVariables
	existing.WorkQueueName = deployment.WorkQueueName
	existing.WorkPoolName = deployment.WorkPoolName
	if deployment.Paused != nil {
		existing.Paused = *deployment.Paused
	}
	existing.Schedules = deployment.Schedules
	existing.ConcurrencyLimit = deployment.ConcurrencyLimit
	existing.GlobalConcurrencyLimits = deployment.GlobalConcurrencyLimits
	existing.Entrypoint = deployment.Entrypoint
	existing.Path = deployment.Path
	existing.PullSteps = deployment.PullSteps
	existing.ParameterOpenAPISchema = deployment.ParameterOpenAPISchema
	if deployment.EnforceParameterSchema != nil {
		existing.EnforceParameterSchema = *deployment.EnforceParameterSchema
	}

	return existing, nil
}

// DeleteDeployment removes a deployment from the mock store
func (m *MockClient) DeleteDeployment(ctx context.Context, deploymentID string) error {
	if m.ShouldFailDelete {
		return fmt.Errorf("mock error: %s", m.FailureMessage)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.deployments, deploymentID)
	return nil
}

// Helper methods for testing

// GetAllDeployments returns all deployments in the mock store (for testing)
func (m *MockClient) GetAllDeployments() map[string]*Deployment {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*Deployment)
	for id, deployment := range m.deployments {
		result[id] = m.copyDeployment(deployment)
	}
	return result
}

// Reset clears all deployments and resets error states (for testing)
func (m *MockClient) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.deployments = make(map[string]*Deployment)
	m.flows = make(map[string]*Flow)
	m.ShouldFailCreate = false
	m.ShouldFailUpdate = false
	m.ShouldFailGet = false
	m.ShouldFailDelete = false
	m.ShouldFailFlowCreate = false
	m.FailureMessage = ""
}

// SetError configures the mock to return errors for testing
func (m *MockClient) SetError(operation string, shouldFail bool, message string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.FailureMessage = message
	switch operation {
	case "create":
		m.ShouldFailCreate = shouldFail
	case "update":
		m.ShouldFailUpdate = shouldFail
	case "get":
		m.ShouldFailGet = shouldFail
	case "delete":
		m.ShouldFailDelete = shouldFail
	case "flow":
		m.ShouldFailFlowCreate = shouldFail
	}
}

// copyDeployment creates a deep copy of a deployment to avoid race conditions
func (m *MockClient) copyDeployment(d *Deployment) *Deployment {
	copy := *d

	// Deep copy slices and maps
	if d.Tags != nil {
		copy.Tags = make([]string, len(d.Tags))
		for i, tag := range d.Tags {
			copy.Tags[i] = tag
		}
	}

	if d.Parameters != nil {
		copy.Parameters = make(map[string]interface{})
		for k, v := range d.Parameters {
			copy.Parameters[k] = v
		}
	}

	if d.JobVariables != nil {
		copy.JobVariables = make(map[string]interface{})
		for k, v := range d.JobVariables {
			copy.JobVariables[k] = v
		}
	}

	if d.Schedules != nil {
		copy.Schedules = make([]Schedule, len(d.Schedules))
		for i, schedule := range d.Schedules {
			copy.Schedules[i] = schedule
		}
	}

	if d.GlobalConcurrencyLimits != nil {
		copy.GlobalConcurrencyLimits = make([]string, len(d.GlobalConcurrencyLimits))
		for i, limit := range d.GlobalConcurrencyLimits {
			copy.GlobalConcurrencyLimits[i] = limit
		}
	}

	if d.PullSteps != nil {
		copy.PullSteps = make([]map[string]interface{}, len(d.PullSteps))
		for i, step := range d.PullSteps {
			copy.PullSteps[i] = make(map[string]interface{})
			for k, v := range step {
				copy.PullSteps[i][k] = v
			}
		}
	}

	if d.ParameterOpenAPISchema != nil {
		copy.ParameterOpenAPISchema = make(map[string]interface{})
		for k, v := range d.ParameterOpenAPISchema {
			copy.ParameterOpenAPISchema[k] = v
		}
	}

	return &copy
}

// CreateOrGetFlow creates or gets a flow in the mock store
func (m *MockClient) CreateOrGetFlow(ctx context.Context, flow *FlowSpec) (*Flow, error) {
	if m.ShouldFailFlowCreate {
		return nil, fmt.Errorf("mock error: %s", m.FailureMessage)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Find existing flow by name
	var existing *Flow
	for _, f := range m.flows {
		if f.Name == flow.Name {
			existing = f
			break
		}
	}

	now := time.Now()

	if existing != nil {
		// Update existing flow
		existing.Updated = now
		existing.Tags = flow.Tags
		existing.Labels = flow.Labels
		return existing, nil
	}

	// Create new flow
	newFlow := &Flow{
		ID:      uuid.New().String(),
		Created: now,
		Updated: now,
		Name:    flow.Name,
		Tags:    flow.Tags,
		Labels:  flow.Labels,
	}

	// Ensure slices are not nil
	if newFlow.Tags == nil {
		newFlow.Tags = []string{}
	}
	if newFlow.Labels == nil {
		newFlow.Labels = make(map[string]string)
	}

	m.flows[newFlow.ID] = newFlow
	return newFlow, nil
}

// GetFlowByName retrieves a flow by name from the mock store
func (m *MockClient) GetFlowByName(ctx context.Context, name string) (*Flow, error) {
	if m.ShouldFailGet {
		return nil, fmt.Errorf("mock error: %s", m.FailureMessage)
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, flow := range m.flows {
		if flow.Name == name {
			return m.copyFlow(flow), nil
		}
	}

	return nil, nil // Not found
}

// copyFlow creates a deep copy of a flow to avoid race conditions
func (m *MockClient) copyFlow(f *Flow) *Flow {
	if f == nil {
		return nil
	}

	copy := *f

	// Deep copy slices and maps
	if f.Tags != nil {
		copy.Tags = make([]string, len(f.Tags))
		for i, tag := range f.Tags {
			copy.Tags[i] = tag
		}
	}
	if f.Labels != nil {
		copy.Labels = make(map[string]string)
		for k, v := range f.Labels {
			copy.Labels[k] = v
		}
	}

	return &copy
}

// DeleteWorkPool removes a work pool
func (m *MockClient) DeleteWorkPool(ctx context.Context, name string) error {
	if m.ShouldFailDelete {
		return fmt.Errorf("mock error: %s", m.FailureMessage)
	}
	return nil
}
