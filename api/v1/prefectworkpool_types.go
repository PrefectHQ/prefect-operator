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
	"maps"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
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

	// Resources defines the CPU and memory resources for each worker in the Work Pool
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// ExtraContainers defines additional containers to add to each worker in the Work Pool
	ExtraContainers []corev1.Container `json:"extraContainers,omitempty"`

	// A list of environment variables to set on the Prefect Worker
	Settings []corev1.EnvVar `json:"settings,omitempty"`

	// DeploymentLabels defines additional labels to add to the Prefect Server Deployment
	DeploymentLabels map[string]string `json:"deploymentLabels,omitempty"`

	// ServiceAccountName defines the ServiceAccount to use for worker pods.
	// If not specified, the default ServiceAccount for the namespace will be used.
	// +optional
	ServiceAccountName *string `json:"serviceAccountName,omitempty"`

	// Base job template for flow runs in the Work Pool
	BaseJobTemplate *RawValueSource `json:"baseJobTemplate,omitempty"`
}

// PrefectWorkPoolStatus defines the observed state of PrefectWorkPool
type PrefectWorkPoolStatus struct {
	// Id is the workPool ID from Prefect
	// +optional
	Id *string `json:"id,omitempty"`

	// Version is the version of the Prefect Worker that is currently running
	Version string `json:"version"`

	// ReadyWorkers is the number of workers that are currently ready
	ReadyWorkers int32 `json:"readyWorkers"`

	// Ready is true if the work pool is ready to accept work
	Ready bool `json:"ready"`

	// SpecHash tracks changes to the spec to minimize API calls
	SpecHash string `json:"specHash,omitempty"`

	// LastSyncTime is the last time the workPool was synced with Prefect
	// +optional
	LastSyncTime *metav1.Time `json:"lastSyncTime,omitempty"`

	// ObservedGeneration tracks the last processed generation
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// BaseJobTemplateVersion tracks the version of BaseJobTemplate ConfigMap, if any is defined
	// +optional
	BaseJobTemplateVersion string `json:"baseJobTemplateVersion,omitempty"`

	// Conditions store the status conditions of the PrefectWorkPool instances
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path="prefectworkpools",singular="prefectworkpool",shortName="pwp",scope="Namespaced"
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Type",type="string",JSONPath=".spec.type",description="The type of this work pool"
// +kubebuilder:printcolumn:name="Version",type="string",JSONPath=".status.version",description="The version of this work pool"
// +kubebuilder:printcolumn:name="Ready",type="boolean",JSONPath=".status.ready",description="Whether the work pool is ready"
// +kubebuilder:printcolumn:name="Desired Workers",type="integer",JSONPath=".spec.workers",description="How many workers are desired"
// +kubebuilder:printcolumn:name="Ready Workers",type="integer",JSONPath=".status.readyWorkers",description="How many workers are ready"
// PrefectWorkPool is the Schema for the prefectworkpools API
type PrefectWorkPool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PrefectWorkPoolSpec   `json:"spec,omitempty"`
	Status PrefectWorkPoolStatus `json:"status,omitempty"`
}

func (s *PrefectWorkPool) WorkerLabels() map[string]string {
	labels := map[string]string{
		"prefect.io/worker": s.Name,
	}

	maps.Copy(labels, s.Spec.DeploymentLabels)

	return labels
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

// ServiceAccount returns the ServiceAccount name to use for worker pods.
// If not specified in the spec, returns empty string to use the default ServiceAccount.
func (s *PrefectWorkPool) ServiceAccount() string {
	if s.Spec.ServiceAccountName != nil && *s.Spec.ServiceAccountName != "" {
		return *s.Spec.ServiceAccountName
	}
	return ""
}

func (s *PrefectWorkPool) EntrypointArguments() []string {
	return []string{
		"prefect", "worker", "start",
		"--pool", s.Name, "--type", s.Spec.Type,
		"--with-healthcheck",
	}
}

// PrefectAPIURL returns the API URL for the Prefect Server, either from the RemoteAPIURL or
// from the in-cluster server
func (s *PrefectWorkPool) PrefectAPIURL() string {
	return s.Spec.Server.GetAPIURL(s.Namespace)
}

func (s *PrefectWorkPool) ToEnvVars() []corev1.EnvVar {
	envVars := []corev1.EnvVar{
		{
			Name:  "PREFECT_HOME",
			Value: "/var/lib/prefect/",
		},
		{
			Name:  "PREFECT_API_URL",
			Value: s.PrefectAPIURL(),
		},
		{
			Name:  "PREFECT_WORKER_WEBSERVER_PORT",
			Value: "8080",
		},
	}

	// If the API key is specified, add it to the environment variables.
	// If both are set, we favor ValueFrom > Value as it is more secure.
	if s.Spec.Server.APIKey != nil && s.Spec.Server.APIKey.ValueFrom != nil {
		envVars = append(envVars, corev1.EnvVar{
			Name:      "PREFECT_API_KEY",
			ValueFrom: s.Spec.Server.APIKey.ValueFrom,
		})
	} else if s.Spec.Server.APIKey != nil && s.Spec.Server.APIKey.Value != nil {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "PREFECT_API_KEY",
			Value: *s.Spec.Server.APIKey.Value,
		})
	}

	return envVars
}

func (s *PrefectWorkPool) HealthProbe() corev1.ProbeHandler {
	return corev1.ProbeHandler{
		HTTPGet: &corev1.HTTPGetAction{
			Path:   "/health",
			Port:   intstr.FromInt(8080),
			Scheme: corev1.URISchemeHTTP,
		},
	}
}

func (s *PrefectWorkPool) StartupProbe() *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler:        s.HealthProbe(),
		InitialDelaySeconds: 10,
		PeriodSeconds:       5,
		TimeoutSeconds:      5,
		SuccessThreshold:    1,
		FailureThreshold:    30,
	}
}
func (s *PrefectWorkPool) ReadinessProbe() *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler:        s.HealthProbe(),
		InitialDelaySeconds: 10,
		PeriodSeconds:       5,
		TimeoutSeconds:      5,
		SuccessThreshold:    1,
		FailureThreshold:    30,
	}
}
func (s *PrefectWorkPool) LivenessProbe() *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler:        s.HealthProbe(),
		InitialDelaySeconds: 120,
		PeriodSeconds:       10,
		TimeoutSeconds:      5,
		SuccessThreshold:    1,
		FailureThreshold:    2,
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
