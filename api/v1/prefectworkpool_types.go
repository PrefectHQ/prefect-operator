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
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PrefectWorkPoolSpec defines the desired state of PrefectWorkPool
type PrefectWorkPoolSpec struct {
	// Version defines the version of the Prefect Server to deploy
	Version *string `json:"version,omitempty"`

	// Image defines the exact image to deploy for the Prefect Server, overriding Version
	Image *string `json:"image,omitempty"`

	// Server defines which Prefect Server to connect to
	Server PrefectServerReference `json:"server,omitempty"`

	// The type of the work pool, such as "kubernetes" or "process"
	Type string `json:"type,omitempty"`

	// Workers defines the number of workers to run in the Work Pool
	Workers int32 `json:"workers,omitempty"`

	// A list of environment variables to set on the Prefect Server
	Settings []corev1.EnvVar `json:"settings,omitempty"`
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

func (s *PrefectWorkPool) WorkerLabels() map[string]string {
	return map[string]string{
		"prefect.io/worker": s.Name,
	}
}

func (s *PrefectWorkPool) Image() string {
	suffix := ""
	if s.Spec.Type == "kubernetes" {
		suffix = "-kubernetes"
	}

	if s.Spec.Image != nil && *s.Spec.Image != "" {
		return *s.Spec.Image
	}
	if s.Spec.Version != nil && *s.Spec.Version != "" {
		return "prefecthq/prefect:" + *s.Spec.Version + "-python3.12" + suffix
	}
	return DEFAULT_PREFECT_IMAGE + suffix
}

func (s *PrefectWorkPool) Command() []string {
	workPoolName := s.Name
	if strings.HasPrefix(workPoolName, "prefect") {
		workPoolName = "pool-" + workPoolName
	}
	return []string{"prefect", "worker", "start", "--pool", workPoolName, "--type", s.Spec.Type}
}

func (s *PrefectWorkPool) PrefectAPIURL() string {
	return "http://" + s.Spec.Server.Name + "." + s.Spec.Server.Namespace + ".svc:4200/api"
}

func (s *PrefectWorkPool) ToEnvVars() []corev1.EnvVar {
	return []corev1.EnvVar{
		{
			Name:  "PREFECT_HOME",
			Value: "/var/lib/prefect/",
		},
		{
			Name:  "PREFECT_API_URL",
			Value: s.PrefectAPIURL(),
		},
	}
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
