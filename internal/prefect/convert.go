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
	"time"

	prefectiov1 "github.com/PrefectHQ/prefect-operator/api/v1"
)

// ConvertToDeploymentSpec converts a K8s PrefectDeployment to a Prefect API DeploymentSpec
func ConvertToDeploymentSpec(k8sDeployment *prefectiov1.PrefectDeployment, flowID string) (*DeploymentSpec, error) {
	spec := &DeploymentSpec{
		Name:   k8sDeployment.Name,
		FlowID: flowID,
	}

	deployment := k8sDeployment.Spec.Deployment

	// Basic fields
	spec.Description = deployment.Description
	spec.Tags = deployment.Tags
	spec.Paused = deployment.Paused
	spec.ConcurrencyLimit = deployment.ConcurrencyLimit
	spec.Entrypoint = &deployment.Entrypoint
	spec.Path = deployment.Path

	// Version info
	if deployment.VersionInfo != nil {
		spec.Version = deployment.VersionInfo.Version
	}

	// Work pool/queue configuration
	spec.WorkPoolName = &k8sDeployment.Spec.WorkPool.Name
	if k8sDeployment.Spec.WorkPool.WorkQueue != nil {
		spec.WorkQueueName = k8sDeployment.Spec.WorkPool.WorkQueue
	}

	// Parameters
	if deployment.Parameters != nil {
		var params map[string]interface{}
		if err := json.Unmarshal(deployment.Parameters.Raw, &params); err != nil {
			return nil, fmt.Errorf("failed to unmarshal parameters: %w", err)
		}
		spec.Parameters = params
	}

	// Job variables
	if deployment.JobVariables != nil {
		var jobVars map[string]interface{}
		if err := json.Unmarshal(deployment.JobVariables.Raw, &jobVars); err != nil {
			return nil, fmt.Errorf("failed to unmarshal job variables: %w", err)
		}
		spec.JobVariables = jobVars
	}

	// Parameter OpenAPI schema
	if deployment.ParameterOpenApiSchema != nil {
		var schema map[string]interface{}
		if err := json.Unmarshal(deployment.ParameterOpenApiSchema.Raw, &schema); err != nil {
			return nil, fmt.Errorf("failed to unmarshal parameter schema: %w", err)
		}
		spec.ParameterOpenAPISchema = schema
	}

	// Enforce parameter schema
	spec.EnforceParameterSchema = deployment.EnforceParameterSchema

	// Pull steps
	if deployment.PullSteps != nil {
		pullSteps := make([]map[string]interface{}, len(deployment.PullSteps))
		for i, step := range deployment.PullSteps {
			var stepMap map[string]interface{}
			if err := json.Unmarshal(step.Raw, &stepMap); err != nil {
				return nil, fmt.Errorf("failed to unmarshal pull step %d: %w", i, err)
			}
			pullSteps[i] = stepMap
		}
		spec.PullSteps = pullSteps
	}

	// Schedules
	if deployment.Schedules != nil {
		schedules := make([]Schedule, len(deployment.Schedules))
		for i, k8sSchedule := range deployment.Schedules {
			schedule := Schedule{
				Interval:         k8sSchedule.Schedule.Interval,
				Timezone:         k8sSchedule.Schedule.Timezone,
				Active:           k8sSchedule.Schedule.Active,
				MaxScheduledRuns: k8sSchedule.Schedule.MaxScheduledRuns,
			}

			// Parse anchor date if provided
			if k8sSchedule.Schedule.AnchorDate != nil {
				anchorDate, err := time.Parse(time.RFC3339, *k8sSchedule.Schedule.AnchorDate)
				if err != nil {
					return nil, fmt.Errorf("failed to parse anchor date for schedule %d: %w", i, err)
				}
				schedule.AnchorDate = &anchorDate
			}

			schedules[i] = schedule
		}
		spec.Schedules = schedules
	}

	// Global concurrency limits
	if deployment.GlobalConcurrencyLimit != nil {
		spec.GlobalConcurrencyLimits = []string{deployment.GlobalConcurrencyLimit.Name}
	}

	return spec, nil
}

// UpdateDeploymentStatus updates the K8s PrefectDeployment status from a Prefect API Deployment
func UpdateDeploymentStatus(k8sDeployment *prefectiov1.PrefectDeployment, prefectDeployment *Deployment) {
	k8sDeployment.Status.Id = &prefectDeployment.ID
	k8sDeployment.Status.FlowId = &prefectDeployment.FlowID
	k8sDeployment.Status.Ready = prefectDeployment.Status == "READY"
}

// GetFlowIDFromDeployment extracts or generates a flow ID for the deployment
func GetFlowIDFromDeployment(ctx context.Context, client PrefectClient, k8sDeployment *prefectiov1.PrefectDeployment) (string, error) {
	flowSpec := &FlowSpec{
		Name:   k8sDeployment.Name,
		Tags:   k8sDeployment.Spec.Deployment.Tags,
		Labels: k8sDeployment.Spec.Deployment.Labels,
	}
	flow, err := client.CreateOrGetFlow(ctx, flowSpec)
	if err != nil {
		return "", fmt.Errorf("failed to create or get flow: %w", err)
	}
	return flow.ID, nil
}
