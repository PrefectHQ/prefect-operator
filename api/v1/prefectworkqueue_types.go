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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PrefectWorkQueueSpec defines the desired state of a PrefectWorkQueue.
// It mirrors the options of the Prefect Terraform provider's prefect_work_queue
// resource so work queues can be managed declaratively via the operator.
//
// A queue referenced by a PrefectDeployment's workQueue field is created
// implicitly by Prefect with no concurrency limit; declaring it here manages
// that limit (and priority) as config. Prefer a work-queue concurrency limit
// over a deployment-level one when run ORDER matters: workers pull from a
// queue sorted by next scheduled start time, whereas a deployment limit
// rejects the transition and re-schedules the run with a fresh timestamp,
// which loses the original ordering.
type PrefectWorkQueueSpec struct {
	// Server configuration for connecting to the Prefect API
	Server PrefectServerReference `json:"server"`

	// Interval is how often to re-check this work queue against the Prefect API
	// to correct out-of-band drift (edits or deletes made directly in Prefect).
	// Defaults to the operator's --default-resync-interval when unset. Values
	// below 10s are clamped.
	// +optional
	Interval *metav1.Duration `json:"interval,omitempty"`

	// Name of the work queue, as referenced by a deployment's workQueue field
	Name string `json:"name"`

	// WorkPoolName is the work pool this queue belongs to. A queue cannot move
	// between pools, so this field is immutable.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="workPoolName is immutable"
	WorkPoolName string `json:"workPoolName"`

	// ConcurrencyLimit caps how many flow runs this queue may have running at
	// once. Unset leaves the queue unlimited.
	// +kubebuilder:validation:Minimum=0
	// +optional
	ConcurrencyLimit *int32 `json:"concurrencyLimit,omitempty"`

	// Priority of this queue within the pool; lower numbers are served first.
	// +kubebuilder:validation:Minimum=1
	// +optional
	Priority *int32 `json:"priority,omitempty"`

	// Description of the queue.
	// +optional
	Description *string `json:"description,omitempty"`

	// IsPaused stops the queue from serving work when true.
	// +optional
	IsPaused *bool `json:"isPaused,omitempty"`
}

// PrefectWorkQueueStatus defines the observed state of a PrefectWorkQueue.
type PrefectWorkQueueStatus struct {
	// Id is the work queue ID from Prefect
	// +optional
	Id *string `json:"id,omitempty"`

	// Ready indicates that the work queue exists and is configured correctly
	Ready bool `json:"ready"`

	// SpecHash tracks changes to the spec to minimize API calls
	// +optional
	SpecHash string `json:"specHash,omitempty"`

	// LastSyncTime is the last time the work queue was synced with Prefect
	// +optional
	LastSyncTime *metav1.Time `json:"lastSyncTime,omitempty"`

	// ObservedGeneration tracks the last processed generation
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions store the status conditions of the PrefectWorkQueue instances
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path="prefectworkqueues",singular="prefectworkqueue",shortName="pwq",scope="Namespaced"
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Work Pool",type="string",JSONPath=".spec.workPoolName",description="The work pool this queue belongs to"
// +kubebuilder:printcolumn:name="Ready",type="boolean",JSONPath=".status.ready",description="Whether this Prefect work queue is ready"
// +kubebuilder:printcolumn:name="ID",type="string",JSONPath=".status.id",description="The Prefect work queue ID"

// PrefectWorkQueue is the Schema for the prefectworkqueues API
type PrefectWorkQueue struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PrefectWorkQueueSpec   `json:"spec,omitempty"`
	Status PrefectWorkQueueStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// PrefectWorkQueueList contains a list of PrefectWorkQueue
type PrefectWorkQueueList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PrefectWorkQueue `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PrefectWorkQueue{}, &PrefectWorkQueueList{})
}
