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

	"github.com/PrefectHQ/prefect-operator/internal/prefect"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	prefectiov1 "github.com/PrefectHQ/prefect-operator/api/v1"
	"github.com/PrefectHQ/prefect-operator/internal/conditions"
	"github.com/PrefectHQ/prefect-operator/internal/constants"
	"github.com/PrefectHQ/prefect-operator/internal/utils"
)

const (
	// PrefectWorkPoolFinalizer is the finalizer used to ensure cleanup of Prefect work pools
	PrefectWorkPoolFinalizer = "prefect.io/work-pool-cleanup"

	// PrefectDeploymentConditionReady indicates the deployment is ready
	PrefectWorkPoolConditionReady = "Ready"

	// PrefectDeploymentConditionSynced indicates the deployment is synced with Prefect API
	PrefectWorkPoolConditionSynced = "Synced"
)

// PrefectWorkPoolReconciler reconciles a PrefectWorkPool object
type PrefectWorkPoolReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	PrefectClient prefect.PrefectClient
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
	log := log.FromContext(ctx)
	log.V(1).Info("Reconciling PrefectWorkPool")

	var workPool prefectiov1.PrefectWorkPool
	err := r.Get(ctx, req.NamespacedName, &workPool)
	if errors.IsNotFound(err) {
		return ctrl.Result{}, nil
	} else if err != nil {
		return ctrl.Result{}, err
	}

	currentStatus := workPool.Status

	// Defer a final status update at the end of the reconciliation loop, so that any of the
	// individual reconciliation functions can update the status as they see fit.
	defer func() {
		// Skip status update if nothing changed, to avoid conflicts
		if equality.Semantic.DeepEqual(workPool.Status, currentStatus) {
			return
		}

		if statusErr := r.Status().Update(ctx, &workPool); statusErr != nil {
			log.Error(statusErr, "Failed to update WorkPool status")
		}
	}()

	// Handle deletion
	if workPool.DeletionTimestamp != nil {
		return r.handleDeletion(ctx, &workPool)
	}

	// Ensure a finalizer is present
	if !controllerutil.ContainsFinalizer(&workPool, PrefectWorkPoolFinalizer) {
		controllerutil.AddFinalizer(&workPool, PrefectWorkPoolFinalizer)
		if err := r.Update(ctx, &workPool); err != nil {
			log.Error(err, "Failed to add finalizer", "workPool", workPool.Name)
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: time.Second}, nil
	}

	var baseJobTemplateConfigMap corev1.ConfigMap

	if workPool.Spec.BaseJobTemplate != nil && workPool.Spec.BaseJobTemplate.ConfigMap != nil {
		configMapRef := workPool.Spec.BaseJobTemplate.ConfigMap
		err := r.Get(ctx, types.NamespacedName{Name: configMapRef.Name, Namespace: workPool.Namespace}, &baseJobTemplateConfigMap)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	specHash, err := utils.Hash(workPool.Spec, 16)
	if err != nil {
		log.Error(err, "Failed to calculate spec hash", "workPool", workPool.Name)
		return ctrl.Result{}, err
	}

	if r.needsSync(&workPool, specHash, &baseJobTemplateConfigMap) {
		log.Info("Starting sync with Prefect API", "deployment", workPool.Name)
		err := r.syncWithPrefect(ctx, &workPool, &baseJobTemplateConfigMap)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	objName := constants.Deployment

	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: workPool.Namespace,
			Name:      workPool.Name,
			Labels:    workPool.WorkerLabels(),
		},
	}

	result, err := controllerutil.CreateOrUpdate(ctx, r.Client, deploy, func() error {
		if err := ctrl.SetControllerReference(&workPool, deploy, r.Scheme); err != nil {
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

							Args: workPool.EntrypointArguments(),
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

	log.V(1).Info("CreateOrUpdate", "object", objName, "name", workPool.Name, "result", result)

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

	r.setCondition(&workPool, PrefectWorkPoolConditionReady, metav1.ConditionTrue, "WorkPoolReady", "Work pool is ready and operational")

	return ctrl.Result{}, nil
}

func (r *PrefectWorkPoolReconciler) needsSync(workPool *prefectiov1.PrefectWorkPool, currentSpecHash string, baseJobTemplateConfigMap *corev1.ConfigMap) bool {
	if workPool.Status.Id == nil || *workPool.Status.Id == "" {
		return true
	}

	if workPool.Status.SpecHash != currentSpecHash {
		return true
	}

	if workPool.Status.ObservedGeneration < workPool.Generation {
		return true
	}

	if workPool.Status.BaseJobTemplateVersion != baseJobTemplateConfigMap.ResourceVersion {
		return true
	}

	// Drift detection: sync if last sync was too long ago
	if workPool.Status.LastSyncTime == nil {
		return true
	}

	timeSinceLastSync := time.Since(workPool.Status.LastSyncTime.Time)
	return timeSinceLastSync > 10*time.Minute
}

func (r *PrefectWorkPoolReconciler) syncWithPrefect(ctx context.Context, workPool *prefectiov1.PrefectWorkPool, baseJobTemplateConfigMap *corev1.ConfigMap) error {
	name := workPool.Name
	log := log.FromContext(ctx)

	prefectClient, err := r.getPrefectClient(ctx, workPool)
	if err != nil {
		log.Error(err, "Failed to create Prefect client", "workPool", name)
		return err
	}

	prefectWorkPool, err := prefectClient.GetWorkPool(ctx, name)
	if err != nil {
		log.Error(err, "Failed to get work pool in Prefect", "workPool", name)
		return err
	}

	var baseJobTemplate []byte

	if baseJobTemplateConfigMap.Name != "" {
		key := workPool.Spec.BaseJobTemplate.ConfigMap.Key

		baseJobTemplateJson, exists := baseJobTemplateConfigMap.Data[key]
		if !exists {
			return fmt.Errorf("can't find key %s in ConfigMap %s", key, baseJobTemplateConfigMap.Name)
		}

		baseJobTemplate = []byte(baseJobTemplateJson)
	}

	if prefectWorkPool == nil {
		workPoolSpec, err := prefect.ConvertToWorkPoolSpec(ctx, workPool, baseJobTemplate, prefectClient)
		if err != nil {
			log.Error(err, "Failed to convert work pool spec", "workPool", name)
			r.setCondition(workPool, PrefectWorkPoolConditionSynced, metav1.ConditionFalse, "ConversionError", err.Error())
			return err
		}

		prefectWorkPool, err = prefectClient.CreateWorkPool(ctx, workPoolSpec)
		if err != nil {
			log.Error(err, "Failed to create work pool in Prefect", "workPool", name)
			r.setCondition(workPool, PrefectWorkPoolConditionSynced, metav1.ConditionFalse, "SyncError", err.Error())
			return err
		}
	} else {
		workPoolSpec, err := prefect.ConvertToWorkPoolUpdateSpec(ctx, workPool, baseJobTemplate, prefectClient)
		if err != nil {
			log.Error(err, "Failed to convert work pool spec", "workPool", name)
			r.setCondition(workPool, PrefectWorkPoolConditionSynced, metav1.ConditionFalse, "ConversionError", err.Error())
			return err
		}

		err = prefectClient.UpdateWorkPool(ctx, workPool.Name, workPoolSpec)
		if err != nil {
			log.Error(err, "Failed to update work pool in Prefect", "workPool", name)
			r.setCondition(workPool, PrefectWorkPoolConditionSynced, metav1.ConditionFalse, "SyncError", err.Error())
			return err
		}
	}

	prefect.UpdateWorkPoolStatus(workPool, prefectWorkPool)

	specHash, err := utils.Hash(workPool.Spec, 16)
	if err != nil {
		log.Error(err, "Failed to calculate spec hash", "workPool", workPool.Name)
		return err
	}

	now := metav1.Now()

	workPool.Status.SpecHash = specHash
	workPool.Status.ObservedGeneration = workPool.Generation
	workPool.Status.LastSyncTime = &now
	workPool.Status.BaseJobTemplateVersion = baseJobTemplateConfigMap.ResourceVersion

	r.setCondition(workPool, PrefectWorkPoolConditionSynced, metav1.ConditionTrue, "SyncSuccessful", "Work pool successfully synced with Prefect API")

	return nil
}

// setCondition sets a condition on the deployment status
func (r *PrefectWorkPoolReconciler) setCondition(workPool *prefectiov1.PrefectWorkPool, conditionType string, status metav1.ConditionStatus, reason, message string) {
	condition := metav1.Condition{
		Type:               conditionType,
		Status:             status,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	}

	meta.SetStatusCondition(&workPool.Status.Conditions, condition)
}

// handleDeletion handles the cleanup of a PrefectWorkPool that is being deleted
func (r *PrefectWorkPoolReconciler) handleDeletion(ctx context.Context, workPool *prefectiov1.PrefectWorkPool) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("Handling deletion of PrefectWorkPool", "workPool", workPool.Name)

	// If the finalizer is not present, nothing to do
	if !controllerutil.ContainsFinalizer(workPool, PrefectWorkPoolFinalizer) {
		return ctrl.Result{}, nil
	}

	if workPool.Name != "" {
		// Create a Prefect client for cleanup
		prefectClient := r.PrefectClient
		if prefectClient == nil {
			var err error
			prefectClient, err = prefect.NewClientFromK8s(ctx, &workPool.Spec.Server, r.Client, workPool.Namespace, log)
			if err != nil {
				log.Error(err, "Failed to create Prefect client for deletion", "workPool", workPool.Name)
				// Continue with finalizer removal even if client creation fails
				// to avoid blocking deletion indefinitely
			} else {
				// Attempt to delete from Prefect API
				if err := prefectClient.DeleteWorkPool(ctx, workPool.Name); err != nil {
					log.Error(err, "Failed to delete workPool from Prefect API", "workPool", workPool.Name)
					// Continue with finalizer removal even if Prefect deletion fails
					// to avoid blocking Kubernetes deletion indefinitely
				} else {
					log.Info("Successfully deleted workPool from Prefect API", "workPool", workPool.Name)
				}
			}
		}
	}

	// Remove the finalizer to allow Kubernetes to complete deletion
	controllerutil.RemoveFinalizer(workPool, PrefectWorkPoolFinalizer)
	if err := r.Update(ctx, workPool); err != nil {
		log.Error(err, "Failed to remove finalizer", "deployment", workPool.Name)
		return ctrl.Result{RequeueAfter: 15 * time.Second}, err
	}

	log.Info("Finalizer removed, deletion will proceed", "deployment", workPool.Name)
	return ctrl.Result{}, nil
}

func (r *PrefectWorkPoolReconciler) getPrefectClient(ctx context.Context, workPool *prefectiov1.PrefectWorkPool) (prefect.PrefectClient, error) {
	log := log.FromContext(ctx)
	name := workPool.Name

	// Use injected client if available (for testing)
	prefectClient := r.PrefectClient

	if prefectClient == nil {
		var err error
		prefectClient, err = prefect.NewClientFromK8s(ctx, &workPool.Spec.Server, r.Client, workPool.Namespace, log)
		if err != nil {
			log.Error(err, "Failed to create Prefect client", "workPool", name)
			return nil, err
		}
	}

	return prefectClient, nil
}

func (r *PrefectWorkPoolReconciler) mapConfigMapToWorkPools(ctx context.Context, obj client.Object) []reconcile.Request {
	configMap, ok := obj.(*corev1.ConfigMap)
	if !ok {
		return nil
	}

	workPools := &prefectiov1.PrefectWorkPoolList{}
	if err := r.List(ctx, workPools, client.InNamespace(configMap.Namespace)); err != nil {
		return nil
	}

	if len(workPools.Items) == 0 {
		return nil
	}

	var requests []reconcile.Request
	for _, workPool := range workPools.Items {
		if workPool.Spec.BaseJobTemplate != nil &&
			workPool.Spec.BaseJobTemplate.ConfigMap != nil &&
			workPool.Spec.BaseJobTemplate.ConfigMap.Name == configMap.Name {
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      workPool.Name,
					Namespace: workPool.Namespace,
				},
			})
		}
	}

	return requests
}

// SetupWithManager sets up the controller with the Manager.
func (r *PrefectWorkPoolReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&prefectiov1.PrefectWorkPool{}).
		Owns(&appsv1.Deployment{}).
		Watches(&corev1.ConfigMap{}, handler.EnqueueRequestsFromMapFunc(r.mapConfigMapToWorkPools)).
		Complete(r)
}
