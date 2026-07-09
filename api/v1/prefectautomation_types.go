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

	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// PrefectAutomationSpec defines the desired state of a PrefectAutomation.
// It mirrors the options of the Prefect Terraform provider's prefect_automation
// resource so automations can be managed declaratively via the operator.
type PrefectAutomationSpec struct {
	// Server configuration for connecting to the Prefect API
	Server PrefectServerReference `json:"server"`

	// Name of the automation
	Name string `json:"name"`

	// Description is human-readable documentation for the automation
	// +optional
	Description *string `json:"description,omitempty"`

	// Enabled activates or deactivates the automation
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// Trigger is the criteria for which events this automation covers.
	// Exactly one of event, metric, compound, or sequence must be set.
	Trigger PrefectAutomationTrigger `json:"trigger"`

	// Actions to perform when the automation is triggered
	// +optional
	Actions []PrefectAutomationAction `json:"actions,omitempty"`

	// ActionsOnTrigger are actions executed when the trigger fires
	// +optional
	ActionsOnTrigger []PrefectAutomationAction `json:"actionsOnTrigger,omitempty"`

	// ActionsOnResolve are actions executed when the trigger resolves
	// +optional
	ActionsOnResolve []PrefectAutomationAction `json:"actionsOnResolve,omitempty"`
}

// PrefectAutomationTrigger is a discriminated union of the supported trigger
// types. Exactly one field must be set.
type PrefectAutomationTrigger struct {
	// Event trigger reacts to (or waits for the absence of) events
	// +optional
	Event *PrefectEventTrigger `json:"event,omitempty"`

	// Metric trigger fires when a metric crosses a threshold
	// +optional
	Metric *PrefectMetricTrigger `json:"metric,omitempty"`

	// Compound trigger fires when a number of child triggers fire within a window
	// +optional
	Compound *PrefectCompoundTrigger `json:"compound,omitempty"`

	// Sequence trigger fires when child triggers fire in order within a window
	// +optional
	Sequence *PrefectSequenceTrigger `json:"sequence,omitempty"`
}

// PrefectEventTrigger matches Prefect's EventTrigger.
type PrefectEventTrigger struct {
	// Posture is the trigger posture.
	// +kubebuilder:validation:Enum=Reactive;Proactive
	Posture string `json:"posture"`

	// Expect is the set of event names the trigger expects to see
	// +optional
	Expect []string `json:"expect,omitempty"`

	// After is the set of event names that must precede the expected events
	// +optional
	After []string `json:"after,omitempty"`

	// ForEach groups events by the given resource fields
	// +optional
	ForEach []string `json:"forEach,omitempty"`

	// Match is a JSON object matching resources by label
	// +optional
	Match *runtime.RawExtension `json:"match,omitempty"`

	// MatchRelated is a JSON object matching related resources by label
	// +optional
	MatchRelated *runtime.RawExtension `json:"matchRelated,omitempty"`

	// Threshold is the number of events required to fire the trigger
	// +optional
	Threshold *int `json:"threshold,omitempty"`

	// Within is the time window in seconds
	// +optional
	Within *int `json:"within,omitempty"`
}

// PrefectMetricTrigger matches Prefect's MetricTrigger.
type PrefectMetricTrigger struct {
	// Metric is the metric query definition
	Metric PrefectMetricQuery `json:"metric"`

	// Match is a JSON object matching resources by label
	// +optional
	Match *runtime.RawExtension `json:"match,omitempty"`

	// MatchRelated is a JSON object matching related resources by label
	// +optional
	MatchRelated *runtime.RawExtension `json:"matchRelated,omitempty"`
}

// PrefectMetricQuery is the metric portion of a metric trigger.
type PrefectMetricQuery struct {
	// Name is the metric to query
	Name string `json:"name"`

	// Operator is the comparative operator used to evaluate the query result
	// +kubebuilder:validation:Enum="<";"<=";">";">="
	Operator string `json:"operator"`

	// Threshold is the value the metric is compared against (may be fractional)
	Threshold resource.Quantity `json:"threshold"`

	// Range is the lookback duration in seconds for the metric query
	Range int `json:"range"`

	// FiringFor is the duration in seconds the query must breach/resolve continuously
	FiringFor int `json:"firingFor"`
}

// PrefectChildTrigger is a trigger nested inside a compound or sequence trigger.
// Prefect only allows event or metric triggers as children (not further
// compound/sequence nesting), which also keeps the CRD schema non-recursive.
// Exactly one of event or metric must be set.
type PrefectChildTrigger struct {
	// Event trigger
	// +optional
	Event *PrefectEventTrigger `json:"event,omitempty"`

	// Metric trigger
	// +optional
	Metric *PrefectMetricTrigger `json:"metric,omitempty"`
}

// PrefectCompoundTrigger matches Prefect's CompoundTrigger.
type PrefectCompoundTrigger struct {
	// Require is how many child triggers must fire: "any", "all", or a number
	Require string `json:"require"`

	// Within is the time window in seconds
	// +optional
	Within *int `json:"within,omitempty"`

	// Triggers are the child triggers (event or metric)
	Triggers []PrefectChildTrigger `json:"triggers"`
}

// PrefectSequenceTrigger matches Prefect's SequenceTrigger.
type PrefectSequenceTrigger struct {
	// Within is the time window in seconds
	// +optional
	Within *int `json:"within,omitempty"`

	// Triggers are the child triggers, which must fire in order
	Triggers []PrefectChildTrigger `json:"triggers"`
}

// PrefectAutomationAction matches Prefect's action union. `type` selects the
// action; the remaining fields apply to the relevant action types.
type PrefectAutomationAction struct {
	// Type is the action to perform.
	// +kubebuilder:validation:Enum=do-nothing;run-deployment;pause-deployment;resume-deployment;cancel-flow-run;change-flow-run-state;suspend-flow-run;resume-flow-run;pause-work-queue;resume-work-queue;pause-work-pool;resume-work-pool;pause-automation;resume-automation;send-notification;call-webhook;declare-incident;pause-schedule-for-flow-run;resume-schedule-for-flow-run
	Type string `json:"type"`

	// Source selects whether the target is "selected" (explicit ID) or "inferred"
	// from the triggering event.
	// +kubebuilder:validation:Enum=selected;inferred
	// +optional
	Source *string `json:"source,omitempty"`

	// DeploymentID targets a deployment (run/pause/resume-deployment) by its
	// Prefect UUID.
	// +optional
	DeploymentID *string `json:"deploymentId,omitempty"`

	// DeploymentName targets a deployment (run/pause/resume-deployment) by its
	// Prefect deployment name; the operator resolves it to a deployment ID at
	// reconcile time (so automations can reference deployments declaratively,
	// without knowing the runtime-assigned UUID). Mutually exclusive with
	// DeploymentID. If the named deployment does not exist yet, reconciliation
	// requeues until it does.
	// +optional
	DeploymentName *string `json:"deploymentName,omitempty"`

	// AutomationID targets an automation (pause/resume-automation)
	// +optional
	AutomationID *string `json:"automationId,omitempty"`

	// WorkPoolID targets a work pool (pause/resume-work-pool)
	// +optional
	WorkPoolID *string `json:"workPoolId,omitempty"`

	// WorkQueueID targets a work queue (pause/resume-work-queue)
	// +optional
	WorkQueueID *string `json:"workQueueId,omitempty"`

	// ScheduleID targets a schedule (pause/resume-schedule-for-flow-run)
	// +optional
	ScheduleID *string `json:"scheduleId,omitempty"`

	// BlockDocumentID targets a block document (send-notification/call-webhook)
	// +optional
	BlockDocumentID *string `json:"blockDocumentId,omitempty"`

	// Parameters is a JSON object of parameters for run-deployment
	// +optional
	Parameters *runtime.RawExtension `json:"parameters,omitempty"`

	// JobVariables is a JSON object of job variables for run-deployment
	// +optional
	JobVariables *runtime.RawExtension `json:"jobVariables,omitempty"`

	// State is the target state for change-flow-run-state
	// +optional
	State *string `json:"state,omitempty"`

	// Name is the state name for change-flow-run-state
	// +optional
	Name *string `json:"name,omitempty"`

	// Message is the state message (change-flow-run-state) or notification body
	// +optional
	Message *string `json:"message,omitempty"`

	// Subject is the notification subject
	// +optional
	Subject *string `json:"subject,omitempty"`

	// Body is the notification body
	// +optional
	Body *string `json:"body,omitempty"`

	// Payload is the webhook payload (call-webhook)
	// +optional
	Payload *string `json:"payload,omitempty"`
}

// PrefectAutomationStatus defines the observed state of a PrefectAutomation.
type PrefectAutomationStatus struct {
	// Id is the automation ID from Prefect
	// +optional
	Id *string `json:"id,omitempty"`

	// Ready indicates that the automation exists and is configured correctly
	Ready bool `json:"ready"`

	// SpecHash tracks changes to the spec to minimize API calls
	// +optional
	SpecHash string `json:"specHash,omitempty"`

	// LastSyncTime is the last time the automation was synced with Prefect
	// +optional
	LastSyncTime *metav1.Time `json:"lastSyncTime,omitempty"`

	// ObservedGeneration tracks the last processed generation
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions store the status conditions of the PrefectAutomation instances
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path="prefectautomations",singular="prefectautomation",shortName="pa",scope="Namespaced"
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type="boolean",JSONPath=".status.ready",description="Whether this Prefect automation is ready"
// +kubebuilder:printcolumn:name="ID",type="string",JSONPath=".status.id",description="The Prefect automation ID"

// PrefectAutomation is the Schema for the prefectautomations API
type PrefectAutomation struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PrefectAutomationSpec   `json:"spec,omitempty"`
	Status PrefectAutomationStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// PrefectAutomationList contains a list of PrefectAutomation
type PrefectAutomationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PrefectAutomation `json:"items"`
}

// Validate performs basic structural validation not expressible via CRD markers.
func (a *PrefectAutomation) Validate() error {
	t := a.Spec.Trigger
	set := 0
	for _, present := range []bool{t.Event != nil, t.Metric != nil, t.Compound != nil, t.Sequence != nil} {
		if present {
			set++
		}
	}
	if set != 1 {
		return fmt.Errorf("exactly one of trigger.event, trigger.metric, trigger.compound, or trigger.sequence must be set (got %d)", set)
	}
	return nil
}

func init() {
	SchemeBuilder.Register(&PrefectAutomation{}, &PrefectAutomationList{})
}
