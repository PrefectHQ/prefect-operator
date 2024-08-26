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

	"dario.cat/mergo"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	prefectiov1 "github.com/PrefectHQ/prefect-operator/api/v1"
	"github.com/go-logr/logr"
)

// PrefectServerReconciler reconciles a PrefectServer object
type PrefectServerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=prefect.io,resources=prefectservers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=prefect.io,resources=prefectservers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=prefect.io,resources=prefectservers/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=events,verbs=create;patch
//+kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch

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

	desiredDeployment, desiredPVC, desiredMigrationJob := r.prefectServerDeployment(server)
	desiredService := r.prefectServerService(server)

	var result *ctrl.Result

	// Reconcile the PVC, if one is required
	result, err = r.reconcilePVC(ctx, server, desiredPVC, log)
	if result != nil {
		return *result, err
	}

	// Reconcile the migration job, if one is required
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
	if desiredPVC == nil {
		if statusErr := r.updateCondition(ctx, server, metav1.Condition{
			Type:    "PersistentVolumeClaimReconciled",
			Status:  metav1.ConditionTrue,
			Reason:  "PersistentVolumeClaimNotRequired",
			Message: "PersistentVolumeClaim is not required",
		}); statusErr != nil {
			return &ctrl.Result{}, statusErr
		}

		return nil, nil
	}

	foundPVC := &corev1.PersistentVolumeClaim{}
	err := r.Get(ctx, types.NamespacedName{Namespace: server.Namespace, Name: desiredPVC.Name}, foundPVC)
	if errors.IsNotFound(err) {
		log.Info("Creating PersistentVolumeClaim", "name", desiredPVC.Name)
		if err = r.Create(ctx, desiredPVC); err != nil {
			if statusErr := r.updateCondition(ctx, server, metav1.Condition{
				Type:    "PersistentVolumeClaimReconciled",
				Status:  metav1.ConditionFalse,
				Reason:  "PersistentVolumeClaimNotCreated",
				Message: "PersistentVolumeClaim was not created: " + err.Error(),
			}); statusErr != nil {
				return &ctrl.Result{}, statusErr
			}

			return &ctrl.Result{}, err
		}

		if statusErr := r.updateCondition(ctx, server, metav1.Condition{
			Type:    "PersistentVolumeClaimReconciled",
			Status:  metav1.ConditionTrue,
			Reason:  "PersistentVolumeClaimCreated",
			Message: "PersistentVolumeClaim was created",
		}); statusErr != nil {
			return &ctrl.Result{}, statusErr
		}
	} else if err != nil {
		if statusErr := r.updateCondition(ctx, server, metav1.Condition{
			Type:    "PersistentVolumeClaimReconciled",
			Status:  metav1.ConditionFalse,
			Reason:  "UnknownError",
			Message: "Unknown error: " + err.Error(),
		}); statusErr != nil {
			return &ctrl.Result{}, statusErr
		}

		return &ctrl.Result{}, err
	} else if !metav1.IsControlledBy(foundPVC, server) {
		errorMessage := fmt.Sprintf(
			"%s %s already exists and is not controlled by PrefectServer %s",
			"PersistentVolumeClaim", desiredPVC.Name, server.Name,
		)

		if statusErr := r.updateCondition(ctx, server, metav1.Condition{
			Type:    "PersistentVolumeClaimReconciled",
			Status:  metav1.ConditionFalse,
			Reason:  "PersistentVolumeClaimAlreadyExists",
			Message: errorMessage,
		}); statusErr != nil {
			return &ctrl.Result{}, statusErr
		}

		return &ctrl.Result{}, errors.NewBadRequest(errorMessage)
	} else if pvcNeedsUpdate(&foundPVC.Spec, &desiredPVC.Spec, log) {
		// TODO: handle patching the PVC if there are meaningful updates that we can make,
		// specifically the size request for a dynamically-provisioned PVC
	} else {
		if !meta.IsStatusConditionTrue(server.Status.Conditions, "PersistentVolumeClaimReconciled") {
			if statusErr := r.updateCondition(ctx, server, metav1.Condition{
				Type:    "PersistentVolumeClaimReconciled",
				Status:  metav1.ConditionTrue,
				Reason:  "PersistentVolumeClaimUpdated",
				Message: "PersistentVolumeClaim is in the correct state",
			}); statusErr != nil {
				return &ctrl.Result{}, statusErr
			}
		}
	}

	return nil, err
}

func (r *PrefectServerReconciler) reconcileMigrationJob(ctx context.Context, server *prefectiov1.PrefectServer, desiredMigrationJob *batchv1.Job, log logr.Logger) (*ctrl.Result, error) {
	if desiredMigrationJob == nil {
		if statusErr := r.updateCondition(ctx, server, metav1.Condition{
			Type:    "MigrationJobReconciled",
			Status:  metav1.ConditionTrue,
			Reason:  "MigrationJobNotRequired",
			Message: "MigrationJob is not required",
		}); statusErr != nil {
			return &ctrl.Result{}, statusErr
		}

		return nil, nil
	}

	foundMigrationJob := &batchv1.Job{}
	err := r.Get(ctx, types.NamespacedName{Namespace: server.Namespace, Name: desiredMigrationJob.Name}, foundMigrationJob)
	if errors.IsNotFound(err) {
		log.Info("Creating migration Job", "name", desiredMigrationJob.Name)
		if err = r.Create(ctx, desiredMigrationJob); err != nil {
			if statusErr := r.updateCondition(ctx, server, metav1.Condition{
				Type:    "MigrationJobReconciled",
				Status:  metav1.ConditionFalse,
				Reason:  "MigrationJobNotCreated",
				Message: "MigrationJob was not created: " + err.Error(),
			}); statusErr != nil {
				return &ctrl.Result{}, statusErr
			}

			return &ctrl.Result{}, err
		}
		if statusErr := r.updateCondition(ctx, server, metav1.Condition{
			Type:    "MigrationJobReconciled",
			Status:  metav1.ConditionTrue,
			Reason:  "MigrationJobCreated",
			Message: "MigrationJob was created",
		}); statusErr != nil {
			return &ctrl.Result{}, statusErr
		}
	} else if err != nil {
		if statusErr := r.updateCondition(ctx, server, metav1.Condition{
			Type:    "MigrationJobReconciled",
			Status:  metav1.ConditionFalse,
			Reason:  "UnknownError",
			Message: "Unknown error: " + err.Error(),
		}); statusErr != nil {
			return &ctrl.Result{}, statusErr
		}

		return &ctrl.Result{}, err
	} else if !metav1.IsControlledBy(foundMigrationJob, server) {
		errorMessage := fmt.Sprintf(
			"%s %s already exists and is not controlled by PrefectServer %s",
			"Job", desiredMigrationJob.Name, server.Name,
		)

		if statusErr := r.updateCondition(ctx, server, metav1.Condition{
			Type:    "MigrationJobReconciled",
			Status:  metav1.ConditionFalse,
			Reason:  "MigrationJobAlreadyExists",
			Message: errorMessage,
		}); statusErr != nil {
			return &ctrl.Result{}, statusErr
		}

		return &ctrl.Result{}, errors.NewBadRequest(errorMessage)
	} else if jobNeedsUpdate(&foundMigrationJob.Spec, &desiredMigrationJob.Spec, log) {
		// TODO: handle replacing the job if something has changed

	} else {
		if !meta.IsStatusConditionTrue(server.Status.Conditions, "MigrationJobReconciled") {
			if statusErr := r.updateCondition(ctx, server, metav1.Condition{
				Type:    "MigrationJobReconciled",
				Status:  metav1.ConditionTrue,
				Reason:  "MigrationJobUpdated",
				Message: "MigrationJob is in the correct state",
			}); statusErr != nil {
				return &ctrl.Result{}, statusErr
			}
		}
	}

	return nil, err
}

func (r *PrefectServerReconciler) reconcileDeployment(ctx context.Context, server *prefectiov1.PrefectServer, desiredDeployment appsv1.Deployment, log logr.Logger) (*ctrl.Result, error) {
	serverNamespacedName := types.NamespacedName{
		Namespace: server.Namespace,
		Name:      server.Name,
	}

	foundDeployment := &appsv1.Deployment{}
	err := r.Get(ctx, serverNamespacedName, foundDeployment)
	if errors.IsNotFound(err) {
		log.Info("Creating Deployment", "name", desiredDeployment.Name)
		if err = r.Create(ctx, &desiredDeployment); err != nil {
			if statusErr := r.updateCondition(ctx, server, metav1.Condition{
				Type:    "DeploymentReconciled",
				Status:  metav1.ConditionFalse,
				Reason:  "DeploymentNotCreated",
				Message: "Deployment was not created: " + err.Error(),
			}); statusErr != nil {
				return &ctrl.Result{}, statusErr
			}

			return &ctrl.Result{}, err
		}

		server.Status.Version = prefectiov1.VersionFromImage(desiredDeployment.Spec.Template.Spec.Containers[0].Image)

		if statusErr := r.updateCondition(ctx, server, metav1.Condition{
			Type:    "DeploymentReconciled",
			Status:  metav1.ConditionTrue,
			Reason:  "DeploymentCreated",
			Message: "Deployment was created",
		}); statusErr != nil {
			return &ctrl.Result{}, statusErr
		}
	} else if err != nil {
		if statusErr := r.updateCondition(ctx, server, metav1.Condition{
			Type:    "DeploymentReconciled",
			Status:  metav1.ConditionFalse,
			Reason:  "UnknownError",
			Message: "Unknown error: " + err.Error(),
		}); statusErr != nil {
			return &ctrl.Result{}, statusErr
		}

		return &ctrl.Result{}, err
	} else if !metav1.IsControlledBy(foundDeployment, server) {
		errorMessage := fmt.Sprintf(
			"%s %s already exists and is not controlled by PrefectServer %s",
			"Deployment", desiredDeployment.Name, server.Name,
		)

		if statusErr := r.updateCondition(ctx, server, metav1.Condition{
			Type:    "DeploymentReconciled",
			Status:  metav1.ConditionFalse,
			Reason:  "DeploymentAlreadyExists",
			Message: errorMessage,
		}); statusErr != nil {
			return &ctrl.Result{}, statusErr
		}

		return &ctrl.Result{}, errors.NewBadRequest(errorMessage)
	} else if deploymentNeedsUpdate(&foundDeployment.Spec, &desiredDeployment.Spec, log) {
		log.Info("Updating Deployment", "name", desiredDeployment.Name)
		if statusErr := r.updateCondition(ctx, server, metav1.Condition{
			Type:    "DeploymentReconciled",
			Status:  metav1.ConditionFalse,
			Reason:  "DeploymentNeedsUpdate",
			Message: "Deployment needs to be updated",
		}); statusErr != nil {
			return &ctrl.Result{}, statusErr
		}

		if err = r.Update(ctx, &desiredDeployment); err != nil {
			if statusErr := r.updateCondition(ctx, server, metav1.Condition{
				Type:    "DeploymentReconciled",
				Status:  metav1.ConditionFalse,
				Reason:  "DeploymentUpdateFailed",
				Message: "Deployment update failed: " + err.Error(),
			}); statusErr != nil {
				return &ctrl.Result{}, statusErr
			}
			return &ctrl.Result{}, err
		}

		if statusErr := r.updateCondition(ctx, server, metav1.Condition{
			Type:    "DeploymentReconciled",
			Status:  metav1.ConditionTrue,
			Reason:  "DeploymentUpdated",
			Message: "Deployment was updated",
		}); statusErr != nil {
			return &ctrl.Result{}, statusErr
		}
	} else {
		imageVersion := prefectiov1.VersionFromImage(desiredDeployment.Spec.Template.Spec.Containers[0].Image)
		if server.Status.Version != imageVersion ||
			!meta.IsStatusConditionTrue(server.Status.Conditions, "DeploymentReconciled") {

			server.Status.Version = imageVersion

			if statusErr := r.updateCondition(ctx, server, metav1.Condition{
				Type:    "DeploymentReconciled",
				Status:  metav1.ConditionTrue,
				Reason:  "DeploymentUpdated",
				Message: "Deployment is in the correct state",
			}); statusErr != nil {
				return &ctrl.Result{}, statusErr
			}
		}
	}
	return nil, err
}

func (r *PrefectServerReconciler) reconcileService(ctx context.Context, server *prefectiov1.PrefectServer, desiredService corev1.Service, log logr.Logger) (*ctrl.Result, error) {
	serverNamespacedName := types.NamespacedName{
		Namespace: server.Namespace,
		Name:      server.Name,
	}

	foundService := &corev1.Service{}
	err := r.Get(ctx, serverNamespacedName, foundService)
	if errors.IsNotFound(err) {
		log.Info("Creating Service", "name", desiredService.Name)
		if err = r.Create(ctx, &desiredService); err != nil {
			if statusErr := r.updateCondition(ctx, server, metav1.Condition{
				Type:    "ServiceReconciled",
				Status:  metav1.ConditionFalse,
				Reason:  "ServiceNotCreated",
				Message: "Service was not created: " + err.Error(),
			}); statusErr != nil {
				return &ctrl.Result{}, statusErr
			}

			return &ctrl.Result{}, err
		}
		if statusErr := r.updateCondition(ctx, server, metav1.Condition{
			Type:    "ServiceReconciled",
			Status:  metav1.ConditionTrue,
			Reason:  "ServiceCreated",
			Message: "Service was created",
		}); statusErr != nil {
			return &ctrl.Result{}, statusErr
		}
	} else if err != nil {
		if statusErr := r.updateCondition(ctx, server, metav1.Condition{
			Type:    "ServiceReconciled",
			Status:  metav1.ConditionFalse,
			Reason:  "UnknownError",
			Message: "Unknown error: " + err.Error(),
		}); statusErr != nil {
			return &ctrl.Result{}, statusErr
		}

		return &ctrl.Result{}, err
	} else if !metav1.IsControlledBy(foundService, server) {
		errorMessage := fmt.Sprintf(
			"%s %s already exists and is not controlled by PrefectServer %s",
			"Service", desiredService.Name, server.Name,
		)

		if statusErr := r.updateCondition(ctx, server, metav1.Condition{
			Type:    "ServiceReconciled",
			Status:  metav1.ConditionFalse,
			Reason:  "ServiceAlreadyExists",
			Message: errorMessage,
		}); statusErr != nil {
			return &ctrl.Result{}, statusErr
		}

		return &ctrl.Result{}, errors.NewBadRequest(errorMessage)
	} else if serviceNeedsUpdate(&foundService.Spec, &desiredService.Spec, log) {
		log.Info("Updating Service", "name", desiredService.Name)
		if statusErr := r.updateCondition(ctx, server, metav1.Condition{
			Type:    "ServiceReconciled",
			Status:  metav1.ConditionFalse,
			Reason:  "ServiceNeedsUpdate",
			Message: "Service needs to be updated",
		}); statusErr != nil {
			return &ctrl.Result{}, statusErr
		}

		if err = r.Update(ctx, &desiredService); err != nil {
			if statusErr := r.updateCondition(ctx, server, metav1.Condition{
				Type:    "ServiceReconciled",
				Status:  metav1.ConditionFalse,
				Reason:  "ServiceUpdateFailed",
				Message: "Service update failed: " + err.Error(),
			}); statusErr != nil {
				return &ctrl.Result{}, statusErr
			}

			return &ctrl.Result{}, err
		}

		if statusErr := r.updateCondition(ctx, server, metav1.Condition{
			Type:    "ServiceReconciled",
			Status:  metav1.ConditionTrue,
			Reason:  "ServiceUpdated",
			Message: "Service was updated",
		}); statusErr != nil {
			return &ctrl.Result{}, statusErr
		}
	} else {
		if !meta.IsStatusConditionTrue(server.Status.Conditions, "ServiceReconciled") {
			if statusErr := r.updateCondition(ctx, server, metav1.Condition{
				Type:    "ServiceReconciled",
				Status:  metav1.ConditionTrue,
				Reason:  "ServiceUpdated",
				Message: "Service is in the correct state",
			}); statusErr != nil {
				return &ctrl.Result{}, statusErr
			}
		}
	}

	return nil, nil
}

func (r *PrefectServerReconciler) updateCondition(ctx context.Context, server *prefectiov1.PrefectServer, condition metav1.Condition) error {
	meta.SetStatusCondition(&server.Status.Conditions, condition)
	return r.Status().Update(ctx, server)
}

func pvcNeedsUpdate(current, desired *corev1.PersistentVolumeClaimSpec, log logr.Logger) bool {
	// TODO: check for meaningful updates to the PVC spec
	return false
}

func jobNeedsUpdate(current, desired *batchv1.JobSpec, log logr.Logger) bool {
	// TODO: check for changes to the job spec that require an update
	return false
}

func deploymentNeedsUpdate(current, desired *appsv1.DeploymentSpec, log logr.Logger) bool {
	merged := current.DeepCopy()
	return needsUpdate(current, merged, desired, log)
}

func serviceNeedsUpdate(current, desired *corev1.ServiceSpec, log logr.Logger) bool {
	merged := current.DeepCopy()
	return needsUpdate(current, merged, desired, log)
}

func needsUpdate(current, merged, desired interface{}, log logr.Logger) bool {
	err := mergo.Merge(merged, desired, mergo.WithOverride)
	if err != nil {
		log.Error(err, "Failed to merge objects", "current", current, "desired", desired)
		return true
	}
	return !equality.Semantic.DeepEqual(current, merged)
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

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      server.Name,
			Namespace: server.Namespace,
		},
		Spec: deploymentSpec,
	}

	// Set PrefectServer instance as the owner and controller
	ctrl.SetControllerReference(server, dep, r.Scheme)
	if pvc != nil {
		ctrl.SetControllerReference(server, pvc, r.Scheme)
	}
	if migrationJob != nil {
		ctrl.SetControllerReference(server, migrationJob, r.Scheme)
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
						Env: append(append(server.ToEnvVars(), server.Spec.SQLite.ToEnvVars()...), server.Spec.Settings...),
						Ports: []corev1.ContainerPort{
							{
								Name:          "api",
								ContainerPort: 4200,
								Protocol:      corev1.ProtocolTCP,
							},
						},
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
				Containers: []corev1.Container{
					{
						Name: "prefect-server",

						Image:           server.Image(),
						ImagePullPolicy: corev1.PullIfNotPresent,

						Command: server.Command(),
						Env:     append(append(server.ToEnvVars(), server.Spec.Postgres.ToEnvVars()...), server.Spec.Settings...),
						Ports: []corev1.ContainerPort{
							{
								Name:          "api",
								ContainerPort: 4200,
								Protocol:      corev1.ProtocolTCP,
							},
						},
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
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: server.Namespace,
			Name:      server.Name + "-migration",
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: server.ServerLabels(),
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:    "prefect-server-migration",
							Image:   server.Image(),
							Command: []string{"prefect", "server", "database", "upgrade", "--yes"},
							Env:     append(append(server.ToEnvVars(), server.Spec.Postgres.ToEnvVars()...), server.Spec.Settings...),
						},
					},
					RestartPolicy: corev1.RestartPolicyOnFailure,
				},
			},
		},
	}
}

func (r *PrefectServerReconciler) prefectServerService(server *prefectiov1.PrefectServer) corev1.Service {
	service := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: server.Namespace,
			Name:      server.Name,
		},
		Spec: corev1.ServiceSpec{
			Selector: server.ServerLabels(),
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

	ctrl.SetControllerReference(server, &service, r.Scheme)

	return service
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
