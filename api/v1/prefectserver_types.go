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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// PrefectServerSpec defines the desired state of a PrefectServer
type PrefectServerSpec struct {
	// Version defines the version of the Prefect Server to deploy
	Version *string `json:"version,omitempty"`

	// Image defines the exact image to deploy for the Prefect Server, overriding Version
	Image *string `json:"image,omitempty"`

	// Resources defines the CPU and memory resources for each replica of the Prefect Server
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// ExtraContainers defines additional containers to add to the Prefect Server Deployment
	ExtraContainers []corev1.Container `json:"extraContainers,omitempty"`

	// ExtraServicePorts defines additional ports to expose on the Prefect Server Service
	ExtraServicePorts []corev1.ServicePort `json:"extraServicePorts,omitempty"`

	// ExtraArgs defines additional arguments to pass to the Prefect Server Deployment
	ExtraArgs []string `json:"extraArgs,omitempty"`

	// Ephemeral defines whether the Prefect Server will be deployed with an ephemeral storage backend
	Ephemeral *EphemeralConfiguration `json:"ephemeral,omitempty"`

	// SQLite defines whether the server will be deployed with a SQLite backend with persistent volume storage
	SQLite *SQLiteConfiguration `json:"sqlite,omitempty"`

	// Postgres defines whether the server will be deployed with a PostgreSQL backend connecting to the
	// database with the provided connection information
	Postgres *PostgresConfiguration `json:"postgres,omitempty"`

	// Redis defines whether the server will be deployed with a Redis message broker
	Redis *RedisConfiguration `json:"redis,omitempty"`

	// A list of environment variables to set on the Prefect Server
	Settings []corev1.EnvVar `json:"settings,omitempty"`

	// DeploymentLabels defines additional labels to add to the server Deployment
	DeploymentLabels map[string]string `json:"deploymentLabels,omitempty"`

	// ServiceLabels defines additional labels to add to the server Service
	ServiceLabels map[string]string `json:"serviceLabels,omitempty"`

	// MigrationJobLabels defines additional labels to add to the migration Job
	MigrationJobLabels map[string]string `json:"migrationJobLabels,omitempty"`

	// NodeSelector defines the node selector for the Prefect Server Deployment and migration Job
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
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

type RedisConfiguration struct {
	Host         *string              `json:"host,omitempty"`
	HostFrom     *corev1.EnvVarSource `json:"hostFrom,omitempty"`
	Port         *int                 `json:"port,omitempty"`
	PortFrom     *corev1.EnvVarSource `json:"portFrom,omitempty"`
	Database     *int                 `json:"database,omitempty"`
	DatabaseFrom *corev1.EnvVarSource `json:"databaseFrom,omitempty"`
	Username     *string              `json:"username,omitempty"`
	UsernameFrom *corev1.EnvVarSource `json:"usernameFrom,omitempty"`
	Password     *string              `json:"password,omitempty"`
	PasswordFrom *corev1.EnvVarSource `json:"passwordFrom,omitempty"`
}

func (r *RedisConfiguration) ToEnvVars() []corev1.EnvVar {
	var envVars = []corev1.EnvVar{
		{
			Name:  "PREFECT_MESSAGING_BROKER",
			Value: "prefect_redis.messaging",
		},
		{
			Name:  "PREFECT_MESSAGING_CACHE",
			Value: "prefect_redis.messaging",
		},
	}

	if r.Host != nil || r.HostFrom != nil {
		envVars = append(envVars, r.HostEnvVar())
	}
	if r.Port != nil || r.PortFrom != nil {
		envVars = append(envVars, r.PortEnvVar())
	}
	if r.Database != nil || r.DatabaseFrom != nil {
		envVars = append(envVars, r.DatabaseEnvVar())
	}
	if r.Username != nil || r.UsernameFrom != nil {
		envVars = append(envVars, r.UsernameEnvVar())
	}
	if r.Password != nil || r.PasswordFrom != nil {
		envVars = append(envVars, r.PasswordEnvVar())
	}

	return envVars
}

func (r *RedisConfiguration) HostEnvVar() corev1.EnvVar {
	if r.Host != nil && *r.Host != "" {
		return corev1.EnvVar{
			Name:  "PREFECT_REDIS_MESSAGING_HOST",
			Value: *r.Host,
		}
	}
	return corev1.EnvVar{
		Name:      "PREFECT_REDIS_MESSAGING_HOST",
		ValueFrom: r.HostFrom,
	}
}

func (r *RedisConfiguration) PortEnvVar() corev1.EnvVar {
	if r.Port != nil && *r.Port != 0 {
		return corev1.EnvVar{
			Name:  "PREFECT_REDIS_MESSAGING_PORT",
			Value: strconv.Itoa(*r.Port),
		}
	}
	return corev1.EnvVar{
		Name:      "PREFECT_REDIS_MESSAGING_PORT",
		ValueFrom: r.PortFrom,
	}
}

func (r *RedisConfiguration) DatabaseEnvVar() corev1.EnvVar {
	if r.Database != nil {
		return corev1.EnvVar{
			Name:  "PREFECT_REDIS_MESSAGING_DB",
			Value: strconv.Itoa(*r.Database),
		}
	}
	return corev1.EnvVar{
		Name:      "PREFECT_REDIS_MESSAGING_DB",
		ValueFrom: r.DatabaseFrom,
	}
}

func (r *RedisConfiguration) UsernameEnvVar() corev1.EnvVar {
	if r.Username != nil && *r.Username != "" {
		return corev1.EnvVar{
			Name:  "PREFECT_REDIS_MESSAGING_USERNAME",
			Value: *r.Username,
		}
	}
	return corev1.EnvVar{
		Name:      "PREFECT_REDIS_MESSAGING_USERNAME",
		ValueFrom: r.UsernameFrom,
	}
}

func (r *RedisConfiguration) PasswordEnvVar() corev1.EnvVar {
	if r.Password != nil && *r.Password != "" {
		return corev1.EnvVar{
			Name:  "PREFECT_REDIS_MESSAGING_PASSWORD",
			Value: *r.Password,
		}
	}
	return corev1.EnvVar{
		Name:      "PREFECT_REDIS_MESSAGING_PASSWORD",
		ValueFrom: r.PasswordFrom,
	}
}

// PrefectServerStatus defines the observed state of PrefectServer
type PrefectServerStatus struct {
	// Version is the version of the PrefectServer that is currently running
	Version string `json:"version"`

	// Ready indicates that the PrefectServer is ready to serve requests
	Ready bool `json:"ready"`

	// Conditions store the status conditions of the PrefectServer instances
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path="prefectservers",singular="prefectserver",shortName="ps",scope="Namespaced"
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Version",type="string",JSONPath=".status.version",description="The version of this Prefect server"
// +kubebuilder:printcolumn:name="Ready",type="boolean",JSONPath=".status.ready",description="Whether this Prefect server is ready to receive requests"
// PrefectServer is the Schema for the prefectservers API
type PrefectServer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PrefectServerSpec   `json:"spec,omitempty"`
	Status PrefectServerStatus `json:"status,omitempty"`
}

func (s *PrefectServer) ServerLabels() map[string]string {
	labels := map[string]string{
		"prefect.io/server": s.Name,
		"app":               "prefect-server",
	}
	for k, v := range s.Spec.DeploymentLabels {
		labels[k] = v
	}
	return labels
}

func (s *PrefectServer) ServiceLabels() map[string]string {
	labels := map[string]string{
		"prefect.io/server": s.Name,
	}
	for k, v := range s.Spec.ServiceLabels {
		labels[k] = v
	}
	return labels
}

func (s *PrefectServer) MigrationJobLabels() map[string]string {
	labels := map[string]string{
		"prefect.io/server": s.Name,
	}
	for k, v := range s.Spec.MigrationJobLabels {
		labels[k] = v
	}
	return labels
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

func (s *PrefectServer) EntrypointArugments() []string {
	command := []string{"prefect", "server", "start", "--host", "0.0.0.0"}
	command = append(command, s.Spec.ExtraArgs...)

	return command
}

func (s *PrefectServer) ToEnvVars() []corev1.EnvVar {
	envVars := []corev1.EnvVar{
		{
			Name:  "PREFECT_HOME",
			Value: "/var/lib/prefect/",
		},
	}

	if s.Spec.Postgres != nil {
		envVars = append(envVars, s.Spec.Postgres.ToEnvVars()...)
	} else if s.Spec.SQLite != nil {
		envVars = append(envVars, s.Spec.SQLite.ToEnvVars()...)
	} else if s.Spec.Ephemeral != nil {
		envVars = append(envVars, s.Spec.Ephemeral.ToEnvVars()...)
	}

	if s.Spec.Redis != nil {
		envVars = append(envVars, s.Spec.Redis.ToEnvVars()...)
	}

	envVars = append(envVars, s.Spec.Settings...)

	return envVars
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
