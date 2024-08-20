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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	prefectiov1 "github.com/PrefectHQ/prefect-operator/api/v1"
)

// PrefectWorkPoolReconciler reconciles a PrefectWorkPool object
type PrefectWorkPoolReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=prefect.io,resources=prefectworkpools,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=prefect.io,resources=prefectworkpools/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=prefect.io,resources=prefectworkpools/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.17.3/pkg/reconcile
func (r *PrefectWorkPoolReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)

	log.Info("Reconciling PrefectWorkPool")

	workPool := &prefectiov1.PrefectWorkPool{}
	err := r.Get(ctx, req.NamespacedName, workPool)
	if errors.IsNotFound(err) {
		return ctrl.Result{}, nil
	} else if err != nil {
		return ctrl.Result{}, err
	}

	desiredDeployment := r.prefectWorkerDeployment(workPool)

	workPoolNamespacedName := types.NamespacedName{
		Namespace: workPool.Namespace,
		Name:      workPool.Name,
	}

	foundDeployment := &appsv1.Deployment{}
	err = r.Get(ctx, workPoolNamespacedName, foundDeployment)
	if errors.IsNotFound(err) {
		log.Info("Creating Deployment", "name", desiredDeployment.Name)
		if err = r.Create(ctx, &desiredDeployment); err != nil {
			meta.SetStatusCondition(&workPool.Status.Conditions, metav1.Condition{
				Type:    "DeploymentReconciled",
				Status:  metav1.ConditionFalse,
				Reason:  "DeploymentNotCreated",
				Message: "Deployment was not created: " + err.Error(),
			})
			if statusErr := r.Status().Update(ctx, workPool); statusErr != nil {
				return ctrl.Result{}, statusErr
			}
			return ctrl.Result{}, err
		}
		meta.SetStatusCondition(&workPool.Status.Conditions, metav1.Condition{
			Type:    "DeploymentReconciled",
			Status:  metav1.ConditionTrue,
			Reason:  "DeploymentCreated",
			Message: "Deployment was created",
		})
		if statusErr := r.Status().Update(ctx, workPool); statusErr != nil {
			return ctrl.Result{}, statusErr
		}
	} else if err != nil {
		meta.SetStatusCondition(&workPool.Status.Conditions, metav1.Condition{
			Type:    "DeploymentReconciled",
			Status:  metav1.ConditionFalse,
			Reason:  "UnknownError",
			Message: "Unknown error: " + err.Error(),
		})
		if statusErr := r.Status().Update(ctx, workPool); statusErr != nil {
			return ctrl.Result{}, statusErr
		}

		return ctrl.Result{}, err
	} else if !metav1.IsControlledBy(foundDeployment, workPool) {
		errorMessage := fmt.Sprintf(
			"%s %s already exists and is not controlled by PrefectWorkPool %s",
			"Deployment", desiredDeployment.Name, workPool.Name,
		)

		meta.SetStatusCondition(&workPool.Status.Conditions, metav1.Condition{
			Type:    "DeploymentReconciled",
			Status:  metav1.ConditionFalse,
			Reason:  "DeploymentAlreadyExists",
			Message: errorMessage,
		})
		if statusErr := r.Status().Update(ctx, workPool); statusErr != nil {
			return ctrl.Result{}, statusErr
		}

		return ctrl.Result{}, errors.NewBadRequest(errorMessage)
	} else if deploymentNeedsUpdate(&foundDeployment.Spec, &desiredDeployment.Spec, log) {
		log.Info("Updating Deployment", "name", desiredDeployment.Name)

		meta.SetStatusCondition(&workPool.Status.Conditions, metav1.Condition{
			Type:    "DeploymentReconciled",
			Status:  metav1.ConditionFalse,
			Reason:  "DeploymentNeedsUpdate",
			Message: "Deployment needs to be updated",
		})
		if statusErr := r.Status().Update(ctx, workPool); statusErr != nil {
			return ctrl.Result{}, statusErr
		}

		if err = r.Update(ctx, &desiredDeployment); err != nil {
			meta.SetStatusCondition(&workPool.Status.Conditions, metav1.Condition{
				Type:    "DeploymentReconciled",
				Status:  metav1.ConditionFalse,
				Reason:  "DeploymentUpdateFailed",
				Message: "Deployment update failed: " + err.Error(),
			})
			if statusErr := r.Status().Update(ctx, workPool); statusErr != nil {
				return ctrl.Result{}, statusErr
			}
			return ctrl.Result{}, err
		}

		meta.SetStatusCondition(&workPool.Status.Conditions, metav1.Condition{
			Type:    "DeploymentReconciled",
			Status:  metav1.ConditionTrue,
			Reason:  "DeploymentUpdated",
			Message: "Deployment was updated",
		})
		if statusErr := r.Status().Update(ctx, workPool); statusErr != nil {
			return ctrl.Result{}, statusErr
		}
	} else {
		workPool.Status.Version = prefectiov1.VersionFromImage(desiredDeployment.Spec.Template.Spec.Containers[0].Image)
		workPool.Status.ReadyWorkers = foundDeployment.Status.ReadyReplicas

		meta.SetStatusCondition(&workPool.Status.Conditions, metav1.Condition{
			Type:    "DeploymentReconciled",
			Status:  metav1.ConditionTrue,
			Reason:  "DeploymentUpdated",
			Message: "Deployment is in the correct state",
		})
		if statusErr := r.Status().Update(ctx, workPool); statusErr != nil {
			return ctrl.Result{}, statusErr
		}
	}

	return ctrl.Result{}, nil
}

func (r *PrefectWorkPoolReconciler) prefectWorkerDeployment(workPool *prefectiov1.PrefectWorkPool) appsv1.Deployment {
	dep := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      workPool.Name,
			Namespace: workPool.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &workPool.Spec.Workers,
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
			},
			Selector: &metav1.LabelSelector{
				MatchLabels: workPool.WorkerLabels(),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: workPool.WorkerLabels(),
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
							Name: "prefect-worker",

							Image:           workPool.Image(),
							ImagePullPolicy: corev1.PullIfNotPresent,

							Command: workPool.Command(),
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "prefect-data",
									MountPath: "/var/lib/prefect/",
								},
							},
							Env: append(workPool.ToEnvVars(), workPool.Spec.Settings...),

							// StartupProbe:   workPool.StartupProbe(),
							// ReadinessProbe: workPool.ReadinessProbe(),
							// LivenessProbe:  workPool.LivenessProbe(),

							TerminationMessagePath:   "/dev/termination-log",
							TerminationMessagePolicy: corev1.TerminationMessageReadFile,
						},
					},
				},
			},
		},
	}

	// Set PrefectWorkPool instance as the owner and controller
	ctrl.SetControllerReference(workPool, &dep, r.Scheme)

	return dep
}

// SetupWithManager sets up the controller with the Manager.
func (r *PrefectWorkPoolReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&prefectiov1.PrefectWorkPool{}).
		Owns(&appsv1.Deployment{}).
		Complete(r)
}
