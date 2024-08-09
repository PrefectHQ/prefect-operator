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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// PrefectWorkPoolSpec defines the desired state of PrefectWorkPool
type PrefectWorkPoolSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Version defines the version of Prefect the Work Pool will run
	Version string `json:"version,omitempty"`

	// Server defines which Prefect Server to connect to
	Server PrefectServerReference `json:"server,omitempty"`

	// Workers defines the number of workers to run in the Work Pool
	Workers int32 `json:"workers,omitempty"`
}

type PrefectServerReference struct {
	// Namespace is the namespace where the Prefect Server is running
	Namespace string `json:"namespace,omitempty"`

	// Name is the name of the Prefect Server in the given namespace
	Name string `json:"name,omitempty"`
}

// PrefectWorkPoolStatus defines the observed state of PrefectWorkPool
type PrefectWorkPoolStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// PrefectWorkPool is the Schema for the prefectworkpools API
type PrefectWorkPool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PrefectWorkPoolSpec   `json:"spec,omitempty"`
	Status PrefectWorkPoolStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// PrefectWorkPoolList contains a list of PrefectWorkPool
type PrefectWorkPoolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PrefectWorkPool `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PrefectWorkPool{}, &PrefectWorkPoolList{})
}
