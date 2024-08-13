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
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const DEFAULT_PREFECT_IMAGE = "prefecthq/prefect:3.0.0rc15-python3.12"

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// PrefectServerSpec defines the desired state of a PrefectServer
type PrefectServerSpec struct {
	// Version defines the version of the Prefect Server to deploy
	Version *string `json:"version,omitempty"`

	// Image defines the exact image to deploy for the Prefect Server, overriding Version
	Image *string `json:"image,omitempty"`

	// Ephemeral defines whether the server will be deployed with an ephemeral storage backend
	Ephemeral *EphemeralConfiguration `json:"ephemeral,omitempty"`

	// SQLite defines whether the server will be deployed with a SQLite backend with persistent volume storage
	SQLite *SQLiteConfiguration `json:"sqlite,omitempty"`

	// Postgres defines whether the server will be deployed with a PostgreSQL backend connecting to the
	// database with the provided connection information
	Postgres *PostgresConfiguration `json:"postgres,omitempty"`

	// A list of environment variables to set on the Prefect Server
	Settings []corev1.EnvVar `json:"settings,omitempty"`
}

type EphemeralConfiguration struct {
}

func (s *EphemeralConfiguration) ToEnvVars() []corev1.EnvVar {
	return []corev1.EnvVar{
		{
			Name:  "PREFECT_API_DATABASE_DRIVER",
			Value: "sqlite+aiosqlite",
		},
		{
			Name:  "PREFECT_API_DATABASE_NAME",
			Value: "/var/lib/prefect/prefect.db",
		},
		{
			Name:  "PREFECT_API_DATABASE_MIGRATE_ON_START",
			Value: "True",
		},
	}
}

type SQLiteConfiguration struct {
	// StorageClassName is the name of the StorageClass of the PersistentVolumeClaim storing the SQLite database
	StorageClassName string `json:"storageClassName,omitempty"`

	// Size is the requested size of the PersistentVolumeClaim storing the `prefect.db`
	Size resource.Quantity `json:"size,omitempty"`
}

func (s *SQLiteConfiguration) ToEnvVars() []corev1.EnvVar {
	return []corev1.EnvVar{
		{
			Name:  "PREFECT_API_DATABASE_DRIVER",
			Value: "sqlite+aiosqlite",
		},
		{
			Name:  "PREFECT_API_DATABASE_NAME",
			Value: "/var/lib/prefect/prefect.db",
		},
		{
			Name:  "PREFECT_API_DATABASE_MIGRATE_ON_START",
			Value: "True",
		},
	}
}

type PostgresConfiguration struct {
	Host         *string              `json:"host,omitempty"`
	HostFrom     *corev1.EnvVarSource `json:"hostFrom,omitempty"`
	Port         *int                 `json:"port,omitempty"`
	PortFrom     *corev1.EnvVarSource `json:"portFrom,omitempty"`
	User         *string              `json:"user,omitempty"`
	UserFrom     *corev1.EnvVarSource `json:"userFrom,omitempty"`
	Password     *string              `json:"password,omitempty"`
	PasswordFrom *corev1.EnvVarSource `json:"passwordFrom,omitempty"`
	Database     *string              `json:"database,omitempty"`
	DatabaseFrom *corev1.EnvVarSource `json:"databaseFrom,omitempty"`
}

func (p *PostgresConfiguration) ToEnvVars() []corev1.EnvVar {
	return []corev1.EnvVar{
		{
			Name:  "PREFECT_API_DATABASE_DRIVER",
			Value: "postgresql+asyncpg",
		},
		p.HostEnvVar(),
		p.PortEnvVar(),
		p.UserEnvVar(),
		p.PasswordEnvVar(),
		p.DatabaseEnvVar(),
		{
			Name:  "PREFECT_API_DATABASE_MIGRATE_ON_START",
			Value: "False",
		},
	}
}

func (p *PostgresConfiguration) HostEnvVar() corev1.EnvVar {
	if p.Host != nil && *p.Host != "" {
		return corev1.EnvVar{
			Name:  "PREFECT_API_DATABASE_HOST",
			Value: *p.Host,
		}
	}
	return corev1.EnvVar{
		Name:      "PREFECT_API_DATABASE_HOST",
		ValueFrom: p.HostFrom,
	}
}

func (p *PostgresConfiguration) PortEnvVar() corev1.EnvVar {
	if p.Port != nil && *p.Port != 0 {
		return corev1.EnvVar{
			Name:  "PREFECT_API_DATABASE_PORT",
			Value: strconv.Itoa(*p.Port),
		}
	}
	return corev1.EnvVar{
		Name:      "PREFECT_API_DATABASE_PORT",
		ValueFrom: p.PortFrom,
	}
}

func (p *PostgresConfiguration) UserEnvVar() corev1.EnvVar {
	if p.User != nil && *p.User != "" {
		return corev1.EnvVar{
			Name:  "PREFECT_API_DATABASE_USER",
			Value: *p.User,
		}
	}
	return corev1.EnvVar{
		Name:      "PREFECT_API_DATABASE_USER",
		ValueFrom: p.UserFrom,
	}
}

func (p *PostgresConfiguration) PasswordEnvVar() corev1.EnvVar {
	if p.Password != nil && *p.Password != "" {
		return corev1.EnvVar{
			Name:  "PREFECT_API_DATABASE_PASSWORD",
			Value: *p.Password,
		}
	}
	return corev1.EnvVar{
		Name:      "PREFECT_API_DATABASE_PASSWORD",
		ValueFrom: p.PasswordFrom,
	}
}

func (p *PostgresConfiguration) DatabaseEnvVar() corev1.EnvVar {
	if p.Database != nil && *p.Database != "" {
		return corev1.EnvVar{
			Name:  "PREFECT_API_DATABASE_NAME",
			Value: *p.Database,
		}
	}
	return corev1.EnvVar{
		Name:      "PREFECT_API_DATABASE_NAME",
		ValueFrom: p.DatabaseFrom,
	}
}

// PrefectServerStatus defines the observed state of PrefectServer
type PrefectServerStatus struct {
	// Represents the observations of a PrefectServer's current state.
	// PrefectServer.status.conditions.type are: "Available", "Progressing", and "Degraded"
	// PrefectServer.status.conditions.status are one of True, False, Unknown.
	// PrefectServer.status.conditions.reason the value should be a CamelCase string and producers of specific
	// condition types may define expected values and meanings for this field, and whether the values
	// are considered a guaranteed API.
	// PrefectServer.status.conditions.Message is a human readable message indicating details about the transition.
	// For further information see: https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties

	// Conditions store the status conditions of the PrefectServer instances
	// +operator-sdk:csv:customresourcedefinitions:type=status
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// PrefectServer is the Schema for the prefectservers API
type PrefectServer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PrefectServerSpec   `json:"spec,omitempty"`
	Status PrefectServerStatus `json:"status,omitempty"`
}

func (s *PrefectServer) ServerLabels() map[string]string {
	return map[string]string{
		"app": s.Name,
	}
}

func (s *PrefectServer) Image() string {
	if s.Spec.Image != nil && *s.Spec.Image != "" {
		return *s.Spec.Image
	}
	if s.Spec.Version != nil && *s.Spec.Version != "" {
		return "prefecthq/prefect:" + *s.Spec.Version + "-python3.12"
	}
	return DEFAULT_PREFECT_IMAGE
}

func (s *PrefectServer) Command() []string {
	return []string{"prefect", "server", "start", "--host", "0.0.0.0"}
}

func (s *PrefectServer) HealthProbe() corev1.ProbeHandler {
	return corev1.ProbeHandler{
		HTTPGet: &corev1.HTTPGetAction{
			Path:   "/api/health",
			Port:   intstr.FromInt(4200),
			Scheme: corev1.URISchemeHTTP,
		},
	}
}

func (s *PrefectServer) StartupProbe() *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler:        s.HealthProbe(),
		InitialDelaySeconds: 10,
		PeriodSeconds:       5,
		TimeoutSeconds:      5,
		SuccessThreshold:    1,
		FailureThreshold:    30,
	}
}
func (s *PrefectServer) ReadinessProbe() *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler:        s.HealthProbe(),
		InitialDelaySeconds: 10,
		PeriodSeconds:       5,
		TimeoutSeconds:      5,
		SuccessThreshold:    1,
		FailureThreshold:    30,
	}
}
func (s *PrefectServer) LivenessProbe() *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler:        s.HealthProbe(),
		InitialDelaySeconds: 120,
		PeriodSeconds:       10,
		TimeoutSeconds:      5,
		SuccessThreshold:    1,
		FailureThreshold:    2,
	}
}

// +kubebuilder:object:root=true
// PrefectServerList contains a list of PrefectServer
type PrefectServerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PrefectServer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PrefectServer{}, &PrefectServerList{})
}