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

package v1

import (
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// PrefectDeploymentSpec defines the desired state of a PrefectDeployment
type PrefectDeploymentSpec struct {
	// Server configuration for connecting to Prefect API
	Server PrefectServerReference `json:"server"`

	// WorkPool configuration specifying where the deployment should run
	WorkPool PrefectWorkPoolReference `json:"workPool"`

	// Deployment configuration defining the Prefect deployment
	Deployment PrefectDeploymentConfiguration `json:"deployment"`
}

// PrefectWorkPoolReference defines the work pool for the deployment
type PrefectWorkPoolReference struct {
	// Namespace is the namespace containing the work pool
	// +optional
	Namespace *string `json:"namespace,omitempty"`

	// Name is the name of the work pool
	Name string `json:"name"`

	// WorkQueue is the specific work queue within the work pool
	// +optional
	WorkQueue *string `json:"workQueue,omitempty"`
}

// PrefectDeploymentConfiguration defines the deployment specification
type PrefectDeploymentConfiguration struct {
	// Description is a human-readable description of the deployment
	// +optional
	Description *string `json:"description,omitempty"`

	// Tags are labels for organizing and filtering deployments
	// +optional
	Tags []string `json:"tags,omitempty"`

	// Labels are key-value pairs for additional metadata
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// VersionInfo describes the deployment version
	// +optional
	VersionInfo *PrefectVersionInfo `json:"versionInfo,omitempty"`

	// Entrypoint is the entrypoint for the flow (e.g., "my_code.py:my_function")
	Entrypoint string `json:"entrypoint"`

	// Path is the path to the flow code
	// +optional
	Path *string `json:"path,omitempty"`

	// PullSteps defines steps to retrieve the flow code
	// +optional
	PullSteps []runtime.RawExtension `json:"pullSteps,omitempty"`

	// ParameterOpenApiSchema defines the OpenAPI schema for flow parameters
	// +optional
	ParameterOpenApiSchema *runtime.RawExtension `json:"parameterOpenApiSchema,omitempty"`

	// EnforceParameterSchema determines if parameter schema should be enforced
	// +optional
	EnforceParameterSchema *bool `json:"enforceParameterSchema,omitempty"`

	// Parameters are default parameters for flow runs
	// +optional
	Parameters *runtime.RawExtension `json:"parameters,omitempty"`

	// JobVariables are variables passed to the infrastructure
	// +optional
	JobVariables *runtime.RawExtension `json:"jobVariables,omitempty"`

	// Paused indicates if the deployment is paused
	// +optional
	Paused *bool `json:"paused,omitempty"`

	// Schedules defines when the deployment should run
	// +optional
	Schedules []PrefectSchedule `json:"schedules,omitempty"`

	// ConcurrencyLimit limits concurrent runs of this deployment
	// +optional
	ConcurrencyLimit *int `json:"concurrencyLimit,omitempty"`

	// GlobalConcurrencyLimit references a global concurrency limit
	// +optional
	GlobalConcurrencyLimit *PrefectGlobalConcurrencyLimit `json:"globalConcurrencyLimit,omitempty"`
}

// PrefectVersionInfo describes deployment version information
type PrefectVersionInfo struct {
	// Type is the version type (e.g., "git")
	// +optional
	Type *string `json:"type,omitempty"`

	// Version is the version string
	// +optional
	Version *string `json:"version,omitempty"`
}

// PrefectSchedule defines a schedule for the deployment.
// This structure exactly matches Prefect's prefect.yaml and API format.
// Exactly one of Interval, Cron, or RRule must be specified.
type PrefectSchedule struct {
	// Slug is a unique identifier for the schedule
	// Maps to: DeploymentScheduleCreate.slug (string)
	Slug string `json:"slug"`

	// === INTERVAL SCHEDULE FIELDS ===
	// Maps to: IntervalSchedule schema in Prefect API

	// Interval is the schedule interval in seconds (required for interval schedules)
	// Maps to: IntervalSchedule.interval (number, required)
	// +optional
	Interval *int `json:"interval,omitempty"`

	// AnchorDate is the anchor date for interval schedules in RFC3339 format
	// Maps to: IntervalSchedule.anchor_date (string, format: date-time)
	// Example: "2024-01-01T00:00:00Z"
	// +optional
	AnchorDate *string `json:"anchor_date,omitempty"`

	// === CRON SCHEDULE FIELDS ===
	// Maps to: CronSchedule schema in Prefect API

	// Cron is a valid cron expression (required for cron schedules)
	// Maps to: CronSchedule.cron (string, required)
	// Examples: "0 9 * * *" (daily at 9am), "*/5 * * * *" (every 5 minutes)
	// +optional
	Cron *string `json:"cron,omitempty"`

	// DayOr controls how croniter handles day and day_of_week entries
	// Maps to: CronSchedule.day_or (boolean, default: true)
	// true = OR logic (standard cron), false = AND logic (like fcron)
	// +optional
	DayOr *bool `json:"day_or,omitempty"`

	// === RRULE SCHEDULE FIELDS ===
	// Maps to: RRuleSchedule schema in Prefect API

	// RRule is a valid RFC 5545 RRULE string (required for rrule schedules)
	// Maps to: RRuleSchedule.rrule (string, required)
	// Examples: "RRULE:FREQ=WEEKLY;BYDAY=MO", "RRULE:FREQ=MONTHLY;BYDAY=1FR"
	// +optional
	RRule *string `json:"rrule,omitempty"`

	// === COMMON FIELDS (shared across all schedule types) ===

	// Timezone for the schedule (IANA timezone string)
	// Maps to: IntervalSchedule.timezone, CronSchedule.timezone, RRuleSchedule.timezone
	// Examples: "America/New_York", "UTC", "Europe/London"
	// +optional
	Timezone *string `json:"timezone,omitempty"`

	// Active indicates if the schedule is active
	// Maps to: DeploymentScheduleCreate.active (boolean, default: true)
	// +optional
	Active *bool `json:"active,omitempty"`

	// MaxScheduledRuns limits the number of scheduled runs
	// Maps to: DeploymentScheduleCreate.max_scheduled_runs (integer > 0)
	// +optional
	MaxScheduledRuns *int `json:"max_scheduled_runs,omitempty"`
}

// PrefectGlobalConcurrencyLimit defines global concurrency limit configuration
type PrefectGlobalConcurrencyLimit struct {
	// Active indicates if the limit is active
	// +optional
	Active *bool `json:"active,omitempty"`

	// Name is the name of the global concurrency limit
	Name string `json:"name"`

	// Limit is the concurrency limit value
	// +optional
	Limit *int `json:"limit,omitempty"`

	// SlotDecayPerSecond defines how quickly slots are released
	// +optional
	SlotDecayPerSecond *string `json:"slotDecayPerSecond,omitempty"`

	// CollisionStrategy defines behavior when limit is exceeded
	// +optional
	CollisionStrategy *string `json:"collisionStrategy,omitempty"`
}

// PrefectDeploymentStatus defines the observed state of PrefectDeployment
type PrefectDeploymentStatus struct {
	// Id is the deployment ID from Prefect
	// +optional
	Id *string `json:"id,omitempty"`

	// FlowId is the flow ID from Prefect
	// +optional
	FlowId *string `json:"flowId,omitempty"`

	// Ready indicates that the deployment exists and is configured correctly
	Ready bool `json:"ready"`

	// SpecHash tracks changes to the spec to minimize API calls
	SpecHash string `json:"specHash,omitempty"`

	// LastSyncTime is the last time the deployment was synced with Prefect
	// +optional
	LastSyncTime *metav1.Time `json:"lastSyncTime,omitempty"`

	// ObservedGeneration tracks the last processed generation
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions store the status conditions of the PrefectDeployment instances
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path="prefectdeployments",singular="prefectdeployment",shortName="pd",scope="Namespaced"
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type="boolean",JSONPath=".status.ready",description="Whether this Prefect deployment is ready"
// +kubebuilder:printcolumn:name="ID",type="string",JSONPath=".status.id",description="The Prefect deployment ID"
// +kubebuilder:printcolumn:name="WorkPool",type="string",JSONPath=".spec.workPool.name",description="The work pool for this deployment"
// PrefectDeployment is the Schema for the prefectdeployments API
type PrefectDeployment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PrefectDeploymentSpec   `json:"spec,omitempty"`
	Status PrefectDeploymentStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// PrefectDeploymentList contains a list of PrefectDeployment
type PrefectDeploymentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PrefectDeployment `json:"items"`
}

// Here for future validation hooks.
func (deployment *PrefectDeployment) Validate() error {
	entryPoint := deployment.Spec.Deployment.Entrypoint

	found := strings.Contains(entryPoint, ":")
	if !found {
		return fmt.Errorf("invalid entrypoint format (missing ':'): %s", entryPoint)
	}

	return nil
}

func init() {
	SchemeBuilder.Register(&PrefectDeployment{}, &PrefectDeploymentList{})
}
