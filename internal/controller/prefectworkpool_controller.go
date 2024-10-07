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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	prefectiov1 "github.com/PrefectHQ/prefect-operator/api/v1"
	"github.com/PrefectHQ/prefect-operator/internal/conditions"
	"github.com/PrefectHQ/prefect-operator/internal/constants"
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

	// Defer a final status update at the end of the reconciliation loop, so that any of the
	// individual reconciliation functions can update the status as they see fit.
	defer func() {
		if statusErr := r.Status().Update(ctx, workPool); statusErr != nil {
			log.Error(statusErr, "Failed to update WorkPool status")
		}
	}()

	objName := constants.Deployment

	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: workPool.Namespace,
			Name:      workPool.Name,
		},
	}

	result, err := controllerutil.CreateOrUpdate(ctx, r.Client, deploy, func() error {
		if err := ctrl.SetControllerReference(workPool, deploy, r.Scheme); err != nil {
			return err
		}

		deploy.Spec = appsv1.DeploymentSpec{
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
					Containers: append([]corev1.Container{
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

							Resources: workPool.Spec.Resources,

							StartupProbe:   workPool.StartupProbe(),
							ReadinessProbe: workPool.ReadinessProbe(),
							LivenessProbe:  workPool.LivenessProbe(),

							TerminationMessagePath:   "/dev/termination-log",
							TerminationMessagePolicy: corev1.TerminationMessageReadFile,
						},
					}, workPool.Spec.ExtraContainers...),
				},
			},
		}

		return nil
	})

	log.Info("CreateOrUpdate", "object", objName, "name", workPool.Name, "result", result)

	meta.SetStatusCondition(
		&workPool.Status.Conditions,
		conditions.GetStatusConditionForOperationResult(result, objName, err),
	)

	if err != nil {
		return ctrl.Result{}, err
	}

	if result == controllerutil.OperationResultUpdated {
		imageVersion := prefectiov1.VersionFromImage(deploy.Spec.Template.Spec.Containers[0].Image)
		readyWorkers := deploy.Status.ReadyReplicas
		ready := readyWorkers > 0

		if workPool.Status.Version != imageVersion ||
			workPool.Status.ReadyWorkers != readyWorkers ||
			workPool.Status.Ready != ready {
			workPool.Status.Version = imageVersion
			workPool.Status.ReadyWorkers = readyWorkers
			workPool.Status.Ready = ready
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PrefectWorkPoolReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&prefectiov1.PrefectWorkPool{}).
		Owns(&appsv1.Deployment{}).
		Complete(r)
}
