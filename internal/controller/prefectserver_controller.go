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

package controller

import (
	"context"
	"fmt"
	"time"

	"dario.cat/mergo"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	prefectiov1 "github.com/PrefectHQ/prefect-operator/api/v1"
	"github.com/PrefectHQ/prefect-operator/internal/conditions"
	"github.com/PrefectHQ/prefect-operator/internal/constants"
	"github.com/PrefectHQ/prefect-operator/internal/utils"
	"github.com/go-logr/logr"
)

// PrefectServerReconciler reconciles a PrefectServer object
type PrefectServerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.17.3/pkg/reconcile
func (r *PrefectServerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)

	log.Info("Reconciling PrefectServer")

	server := &prefectiov1.PrefectServer{}
	err := r.Get(ctx, req.NamespacedName, server)
	if errors.IsNotFound(err) {
		return ctrl.Result{}, nil
	} else if err != nil {
		return ctrl.Result{}, err
	}

	// Defer a final status update at the end of the reconciliation loop, so that any of the
	// individual reconciliation functions can update the status as they see fit.
	defer func() {
		if statusErr := r.Status().Update(ctx, server); statusErr != nil {
			log.Error(statusErr, "Failed to update PrefectServer status")
		}
	}()

	desiredDeployment, desiredPVC, desiredMigrationJob := r.prefectServerDeployment(server)
	desiredService := r.prefectServerService(server)

	var result *ctrl.Result

	// Reconcile the PVC, if one is required
	result, err = r.reconcilePVC(ctx, server, desiredPVC, log)
	if result != nil {
		return *result, err
	}

	// Reconcile the migration job, if one is required.
	// NOTE: if an active migration is still running,
	// this reconciliation will requeue and exit early here.
	result, err = r.reconcileMigrationJob(ctx, server, desiredMigrationJob, log)
	if result != nil {
		return *result, err
	}

	// Reconcile the Deployment
	result, err = r.reconcileDeployment(ctx, server, desiredDeployment, log)
	if result != nil {
		return *result, err
	}

	// Reconcile the Service
	result, err = r.reconcileService(ctx, server, desiredService, log)
	if result != nil {
		return *result, err
	}

	return ctrl.Result{}, nil
}

func (r *PrefectServerReconciler) reconcilePVC(ctx context.Context, server *prefectiov1.PrefectServer, desiredPVC *corev1.PersistentVolumeClaim, log logr.Logger) (*ctrl.Result, error) {
	objName := constants.PVC

	if desiredPVC == nil {
		meta.SetStatusCondition(&server.Status.Conditions, conditions.NotRequired(objName))
		return nil, nil
	}

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      desiredPVC.Name,
			Namespace: server.Namespace,
		},
	}

	result, err := controllerutil.CreateOrUpdate(ctx, r.Client, pvc, func() error {
		if err := ctrl.SetControllerReference(server, pvc, r.Scheme); err != nil {
			return err
		}

		return mergo.Merge(pvc, desiredPVC, mergo.WithOverride)
	})

	log.Info("CreateOrUpdate", "object", objName, "name", server.Name, "result", result)

	meta.SetStatusCondition(
		&server.Status.Conditions,
		conditions.GetStatusConditionForOperationResult(result, objName, err),
	)

	if err != nil {
		return &ctrl.Result{}, err
	}

	return nil, nil
}

func (r *PrefectServerReconciler) reconcileMigrationJob(ctx context.Context, server *prefectiov1.PrefectServer, desiredMigrationJob *batchv1.Job, log logr.Logger) (*ctrl.Result, error) {
	objName := constants.MigrationJob

	if desiredMigrationJob == nil {
		meta.SetStatusCondition(&server.Status.Conditions, conditions.NotRequired(objName))
		return nil, nil
	}

	foundMigrationJob := &batchv1.Job{}
	err := r.Get(ctx, types.NamespacedName{Namespace: server.Namespace, Name: desiredMigrationJob.Name}, foundMigrationJob)

	switch {
	case errors.IsNotFound(err):
		log.Info("Creating migration Job", "name", desiredMigrationJob.Name)
		if err = r.Create(ctx, desiredMigrationJob); err != nil {
			meta.SetStatusCondition(&server.Status.Conditions, conditions.NotCreated(objName, err))
			return &ctrl.Result{}, err
		}
		meta.SetStatusCondition(&server.Status.Conditions, conditions.Created(objName))

	case err != nil:
		meta.SetStatusCondition(&server.Status.Conditions, conditions.UnknownError(objName, err))
		return &ctrl.Result{}, err

	case !isMigrationJobFinished(foundMigrationJob):
		log.Info("Waiting on active migration Job to complete", "name", foundMigrationJob.Name)
		meta.SetStatusCondition(
			&server.Status.Conditions,
			conditions.AlreadyExists(
				objName,
				fmt.Sprintf("migration Job %s is still active", foundMigrationJob.Name),
			),
		)

		// We'll requeue after 20 seconds to check on the migration Job's status
		return &ctrl.Result{Requeue: true, RequeueAfter: 20 * time.Second}, nil

	default:
		if !meta.IsStatusConditionTrue(server.Status.Conditions, "MigrationJobReconciled") {
			meta.SetStatusCondition(&server.Status.Conditions, conditions.Updated(objName))
		}
	}

	return nil, err
}

func (r *PrefectServerReconciler) reconcileDeployment(ctx context.Context, server *prefectiov1.PrefectServer, desiredDeployment appsv1.Deployment, log logr.Logger) (*ctrl.Result, error) {
	objName := constants.Deployment

	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      server.Name,
			Namespace: server.Namespace,
		},
	}

	result, err := controllerutil.CreateOrUpdate(ctx, r.Client, deploy, func() error {
		if err := ctrl.SetControllerReference(server, deploy, r.Scheme); err != nil {
			return err
		}

		return mergo.Merge(deploy, desiredDeployment, mergo.WithOverride)
	})

	log.Info("CreateOrUpdate", "object", objName, "name", server.Name, "result", result)

	meta.SetStatusCondition(
		&server.Status.Conditions,
		conditions.GetStatusConditionForOperationResult(result, objName, err),
	)

	if err != nil {
		return &ctrl.Result{}, err
	}

	serverContainer := desiredDeployment.Spec.Template.Spec.Containers[0]

	switch result {
	case controllerutil.OperationResultCreated:
		server.Status.Version = prefectiov1.VersionFromImage(serverContainer.Image)
	case controllerutil.OperationResultUpdated:
		ready := deploy.Status.ReadyReplicas > 0
		version := prefectiov1.VersionFromImage(serverContainer.Image)
		if server.Status.Ready != ready || server.Status.Version != version {
			server.Status.Ready = ready
			server.Status.Version = version
		}
	}

	return nil, nil
}

func (r *PrefectServerReconciler) reconcileService(ctx context.Context, server *prefectiov1.PrefectServer, desiredService corev1.Service, log logr.Logger) (*ctrl.Result, error) {
	objName := constants.Service

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      server.Name,
			Namespace: server.Namespace,
		},
	}

	result, err := controllerutil.CreateOrUpdate(ctx, r.Client, service, func() error {
		if err := ctrl.SetControllerReference(server, service, r.Scheme); err != nil {
			return err
		}

		return mergo.Merge(service, desiredService, mergo.WithOverride)
	})

	log.Info("CreateOrUpdate", "object", objName, "name", server.Name, "result", result)

	meta.SetStatusCondition(
		&server.Status.Conditions,
		conditions.GetStatusConditionForOperationResult(result, objName, err),
	)

	if err != nil {
		return &ctrl.Result{}, err
	}

	return nil, nil
}

func isMigrationJobFinished(foundMigrationJob *batchv1.Job) bool {
	switch {
	case foundMigrationJob.Status.Succeeded > 0:
		return true
	case foundMigrationJob.Status.Failed > 0:
		return true
	default:
		return false
	}
}

func (r *PrefectServerReconciler) prefectServerDeployment(server *prefectiov1.PrefectServer) (appsv1.Deployment, *corev1.PersistentVolumeClaim, *batchv1.Job) {
	var pvc *corev1.PersistentVolumeClaim
	var migrationJob *batchv1.Job
	var deploymentSpec appsv1.DeploymentSpec

	if server.Spec.SQLite != nil {
		pvc = r.sqlitePersistentVolumeClaim(server)
		deploymentSpec = r.sqliteDeploymentSpec(server, pvc)
	} else if server.Spec.Postgres != nil {
		migrationJob = r.postgresMigrationJob(server)
		deploymentSpec = r.postgresDeploymentSpec(server)
	} else {
		if server.Spec.Ephemeral == nil {
			server.Spec.Ephemeral = &prefectiov1.EphemeralConfiguration{}
		}
		deploymentSpec = r.ephemeralDeploymentSpec(server)
	}

	// Append any extra containers into the Deployment if configured.
	if server.Spec.ExtraContainers != nil {
		deploymentSpec.Template.Spec.Containers = append(deploymentSpec.Template.Spec.Containers, server.Spec.ExtraContainers...)
	}

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      server.Name,
			Namespace: server.Namespace,
		},
		Spec: deploymentSpec,
	}

	// Set PrefectServer instance as the owner and controller
	// TODO: handle errors from SetControllerReference.
	_ = ctrl.SetControllerReference(server, dep, r.Scheme)
	if pvc != nil {
		_ = ctrl.SetControllerReference(server, pvc, r.Scheme)
	}
	if migrationJob != nil {
		_ = ctrl.SetControllerReference(server, migrationJob, r.Scheme)
	}
	return *dep, pvc, migrationJob
}

func (r *PrefectServerReconciler) ephemeralDeploymentSpec(server *prefectiov1.PrefectServer) appsv1.DeploymentSpec {
	return appsv1.DeploymentSpec{
		Replicas: ptr.To(int32(1)),
		Strategy: appsv1.DeploymentStrategy{
			Type: appsv1.RollingUpdateDeploymentStrategyType,
		},
		Selector: &metav1.LabelSelector{
			MatchLabels: server.ServerLabels(),
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: server.ServerLabels(),
			},
			Spec: corev1.PodSpec{
				NodeSelector: server.Spec.NodeSelector,
				Volumes: []corev1.Volume{
					{
						Name: "prefect-data",
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						},
					},
				},
				Containers: []corev1.Container{
					{
						Name: "prefect-server",

						Image:           server.Image(),
						ImagePullPolicy: corev1.PullIfNotPresent,

						Command: server.Command(),
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "prefect-data",
								MountPath: "/var/lib/prefect/",
							},
						},
						Env: append(append(server.ToEnvVars(), server.Spec.Ephemeral.ToEnvVars()...), server.Spec.Settings...),
						Ports: []corev1.ContainerPort{
							{
								Name:          "api",
								ContainerPort: 4200,
								Protocol:      corev1.ProtocolTCP,
							},
						},

						Resources: server.Spec.Resources,

						StartupProbe:   server.StartupProbe(),
						ReadinessProbe: server.ReadinessProbe(),
						LivenessProbe:  server.LivenessProbe(),

						TerminationMessagePath:   "/dev/termination-log",
						TerminationMessagePolicy: corev1.TerminationMessageReadFile,
					},
				},
			},
		},
	}
}

func (r *PrefectServerReconciler) sqlitePersistentVolumeClaim(server *prefectiov1.PrefectServer) *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: server.Namespace,
			Name:      server.Name + "-data",
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			StorageClassName: &server.Spec.SQLite.StorageClassName,
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: server.Spec.SQLite.Size,
				},
			},
		},
	}
}

func (r *PrefectServerReconciler) sqliteDeploymentSpec(server *prefectiov1.PrefectServer, pvc *corev1.PersistentVolumeClaim) appsv1.DeploymentSpec {
	return appsv1.DeploymentSpec{
		Replicas: ptr.To(int32(1)),
		Strategy: appsv1.DeploymentStrategy{
			Type: appsv1.RecreateDeploymentStrategyType,
		},
		Selector: &metav1.LabelSelector{
			MatchLabels: server.ServerLabels(),
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: server.ServerLabels(),
			},
			Spec: corev1.PodSpec{
				NodeSelector: server.Spec.NodeSelector,
				Volumes: []corev1.Volume{
					{
						Name: "prefect-data",
						VolumeSource: corev1.VolumeSource{
							PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
								ClaimName: pvc.Name,
							},
						},
					},
				},
				Containers: []corev1.Container{
					{
						Name: "prefect-server",

						Image:           server.Image(),
						ImagePullPolicy: corev1.PullIfNotPresent,

						Command: server.Command(),
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "prefect-data",
								MountPath: "/var/lib/prefect/",
							},
						},
						Env: append(
							append(
								server.ToEnvVars(),
								server.Spec.SQLite.ToEnvVars()...),
							server.Spec.Settings...,
						),
						Ports: []corev1.ContainerPort{
							{
								Name:          "api",
								ContainerPort: 4200,
								Protocol:      corev1.ProtocolTCP,
							},
						},

						Resources: server.Spec.Resources,

						StartupProbe:   server.StartupProbe(),
						ReadinessProbe: server.ReadinessProbe(),
						LivenessProbe:  server.LivenessProbe(),

						TerminationMessagePath:   "/dev/termination-log",
						TerminationMessagePolicy: corev1.TerminationMessageReadFile,
					},
				},
			},
		},
	}
}

func (r *PrefectServerReconciler) postgresDeploymentSpec(server *prefectiov1.PrefectServer) appsv1.DeploymentSpec {
	return appsv1.DeploymentSpec{
		Replicas: ptr.To(int32(1)),
		Strategy: appsv1.DeploymentStrategy{
			Type: appsv1.RollingUpdateDeploymentStrategyType,
		},
		Selector: &metav1.LabelSelector{
			MatchLabels: server.ServerLabels(),
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: server.ServerLabels(),
			},
			Spec: corev1.PodSpec{
				NodeSelector: server.Spec.NodeSelector,
				InitContainers: []corev1.Container{
					r.initContainerWaitForPostgres(server),
				},
				Containers: []corev1.Container{
					{
						Name: "prefect-server",

						Image:           server.Image(),
						ImagePullPolicy: corev1.PullIfNotPresent,

						Command: server.Command(),
						Env: append(
							append(
								server.ToEnvVars(),
								server.Spec.Postgres.ToEnvVars()...),
							server.Spec.Settings...,
						),
						Ports: []corev1.ContainerPort{
							{
								Name:          "api",
								ContainerPort: 4200,
								Protocol:      corev1.ProtocolTCP,
							},
						},

						Resources: server.Spec.Resources,

						StartupProbe:   server.StartupProbe(),
						ReadinessProbe: server.ReadinessProbe(),
						LivenessProbe:  server.LivenessProbe(),

						TerminationMessagePath:   "/dev/termination-log",
						TerminationMessagePolicy: corev1.TerminationMessageReadFile,
					},
				},
			},
		},
	}
}

func (r *PrefectServerReconciler) postgresMigrationJob(server *prefectiov1.PrefectServer) *batchv1.Job {
	jobSpec := batchv1.JobSpec{
		TTLSecondsAfterFinished: ptr.To(int32(60 * 60)), // 1 hour
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: server.MigrationJobLabels(),
			},
			Spec: corev1.PodSpec{
				NodeSelector: server.Spec.NodeSelector,
				InitContainers: []corev1.Container{
					r.initContainerWaitForPostgres(server),
				},
				Containers: []corev1.Container{
					{
						Name:    "prefect-server-migration",
						Image:   server.Image(),
						Command: []string{"prefect", "server", "database", "upgrade", "--yes"},
						Env: append(
							append(
								server.ToEnvVars(),
								server.Spec.Postgres.ToEnvVars()...,
							),
							server.Spec.Settings...,
						),
					},
				},
				RestartPolicy: corev1.RestartPolicyOnFailure,
			},
		},
	}

	// Generate hash based on the job spec
	hashSuffix, _ := utils.Hash(jobSpec, 8) // Use the first 8 characters of the hash
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: server.Namespace,
			Name:      fmt.Sprintf("%s-migration-%s", server.Name, hashSuffix),
		},
		Spec: jobSpec,
	}
}

func (r *PrefectServerReconciler) prefectServerService(server *prefectiov1.PrefectServer) corev1.Service {
	service := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: server.Namespace,
			Name:      server.Name,
		},
		Spec: corev1.ServiceSpec{
			Selector: server.ServiceLabels(),
			Ports: []corev1.ServicePort{
				{
					Name:       "api",
					Protocol:   corev1.ProtocolTCP,
					Port:       4200,
					TargetPort: intstr.FromString("api"),
				},
			},
		},
	}
	// Append any extra ports into the Service if configured.
	if server.Spec.ExtraServicePorts != nil {
		service.Spec.Ports = append(service.Spec.Ports, server.Spec.ExtraServicePorts...)
	}

	// TODO: handle errors from SetControllerReference.
	_ = ctrl.SetControllerReference(server, &service, r.Scheme)

	return service
}

func (r *PrefectServerReconciler) initContainerWaitForPostgres(server *prefectiov1.PrefectServer) corev1.Container {
	return corev1.Container{
		Name:            "wait-for-database",
		Image:           "postgres:16-alpine",
		ImagePullPolicy: corev1.PullIfNotPresent,
		Command: []string{
			"/bin/sh",
			"-c",
			"until pg_isready -h $PREFECT_API_DATABASE_HOST -U $PREFECT_API_DATABASE_USER; do echo 'Waiting for PostgreSQL...'; sleep 2; done;",
		},
		Env: []corev1.EnvVar{
			server.Spec.Postgres.HostEnvVar(),
			server.Spec.Postgres.UserEnvVar(),
		},
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *PrefectServerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&prefectiov1.PrefectServer{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Complete(r)
}
