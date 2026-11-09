package controller

import (
	"context"
	"fmt"
	"maps"
	"slices"
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
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	prefectiov1 "github.com/PrefectHQ/prefect-operator/api/v1"
	"github.com/PrefectHQ/prefect-operator/internal/conditions"
	"github.com/PrefectHQ/prefect-operator/internal/constants"
	"github.com/PrefectHQ/prefect-operator/internal/podtemplate"
	"github.com/PrefectHQ/prefect-operator/internal/utils"
)

const (
	// PrefectWorkPoolFinalizer is the finalizer used to ensure cleanup of Prefect work pools
	PrefectWorkPoolFinalizer = "prefect.io/work-pool-cleanup"

	// PrefectDeploymentConditionReady indicates the deployment is ready
	PrefectWorkPoolConditionReady = "Ready"

	// PrefectDeploymentConditionSynced indicates the deployment is synced with Prefect API
	PrefectWorkPoolConditionSynced = "Synced"

	// PrefectWorkPoolConditionServerAvailable indicates the Prefect server is available
	PrefectWorkPoolConditionServerAvailable = "ServerAvailable"
)

// PrefectWorkPoolReconciler reconciles a PrefectWorkPool object
type PrefectWorkPoolReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	// NewPrefectClient returns a client for the Prefect server that a
	// resource references.
	NewPrefectClient prefect.ClientFactory
	// DefaultResyncInterval is the fallback drift-detection interval used when a
	// PrefectWorkPool does not set spec.interval.
	DefaultResyncInterval time.Duration
	// Recorder emits Kubernetes events on the resources this controller
	// reconciles.
	Recorder events.EventRecorder
}

// resyncInterval returns the effective drift-detection interval for the work
// pool: its spec.interval when set, otherwise the operator default.
func (r *PrefectWorkPoolReconciler) resyncInterval(workPool *prefectiov1.PrefectWorkPool) time.Duration {
	return utils.ResyncInterval(workPool.Spec.Interval, r.DefaultResyncInterval)
}

//+kubebuilder:rbac:groups=prefect.io,resources=prefectworkpools,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=prefect.io,resources=prefectworkpools/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=prefect.io,resources=prefectworkpools/finalizers,verbs=update
//+kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch

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
			return ctrl.Result{}, r.syncFailed(&workPool, "BaseJobTemplateError", err)
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

	podTemplate, err := workerPodTemplate(&workPool)
	if err != nil {
		meta.SetStatusCondition(&workPool.Status.Conditions, conditions.InvalidPodTemplate(objName, err))
		r.Recorder.Eventf(&workPool, nil, corev1.EventTypeWarning, "InvalidPodTemplate", eventActionSync, "%v", err)
		// Retrying won't help until the user fixes the template, which triggers
		// a new reconcile, but keep resyncing the work pool with Prefect.
		return ctrl.Result{RequeueAfter: utils.NextResyncDelay(workPool.Status.LastSyncTime, r.resyncInterval(&workPool))}, nil
	}

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

		liveLabels := deploy.Spec.Template.Labels
		liveAnnotations := deploy.Spec.Template.Annotations
		deploy.Spec = appsv1.DeploymentSpec{
			Replicas: &workPool.Spec.Workers,
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
			},
			Selector: &metav1.LabelSelector{
				MatchLabels: workPool.WorkerLabels(),
			},
			Template: *podTemplate.DeepCopy(),
		}
		setPodTemplateMetadata(deploy, liveLabels, liveAnnotations)

		return nil
	})

	log.V(1).Info("CreateOrUpdate", "object", objName, "name", workPool.Name, "result", result)

	meta.SetStatusCondition(
		&workPool.Status.Conditions,
		conditions.GetStatusConditionForOperationResult(result, objName, err),
	)

	if err != nil {
		r.Recorder.Eventf(&workPool, nil, corev1.EventTypeWarning, "WorkerDeploymentError", eventActionSync,
			"Failed to create or update the worker Deployment: %v", err)
		return ctrl.Result{}, err
	}

	if result == controllerutil.OperationResultUpdated {
		imageVersion := prefectiov1.VersionFromImage(podtemplate.FindContainer(&podTemplate.Spec, prefectiov1.ContainerNameWorker).Image)
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

	// Requeue on the resync interval so out-of-band drift in Prefect is corrected
	// even without an incoming watch event.
	return ctrl.Result{RequeueAfter: utils.NextResyncDelay(workPool.Status.LastSyncTime, r.resyncInterval(&workPool))}, nil
}

// workerPodTemplate builds the pod template of the worker Deployment and
// applies the work pool's deploymentTemplate to it.
func workerPodTemplate(workPool *prefectiov1.PrefectWorkPool) (corev1.PodTemplateSpec, error) {
	base := corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels: workPool.WorkerLabels(),
		},
		Spec: corev1.PodSpec{
			ServiceAccountName: workPool.ServiceAccount(),
			Affinity:           workPool.Spec.Affinity,
			Tolerations:        workPool.Spec.Tolerations,
			NodeSelector:       workPool.Spec.NodeSelector,
			Volumes: []corev1.Volume{
				{
					Name: constants.PrefectDataVolumeName,
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
			},
			Containers: append([]corev1.Container{
				{
					Name: prefectiov1.ContainerNameWorker,

					Image:           workPool.Image(),
					ImagePullPolicy: corev1.PullIfNotPresent,

					Args: workPool.EntrypointArguments(),
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      constants.PrefectDataVolumeName,
							MountPath: constants.PrefectHomeMountPath,
						},
					},
					Env: append(workPool.ToEnvVars(), workPool.Spec.Settings...),

					Resources: workPool.Spec.Resources,

					StartupProbe:   workPool.StartupProbe(),
					ReadinessProbe: workPool.ReadinessProbe(),
					LivenessProbe:  workPool.LivenessProbe(),

					TerminationMessagePath:   constants.TerminationMessagePath,
					TerminationMessagePolicy: corev1.TerminationMessageReadFile,
				},
			}, workPool.Spec.ExtraContainers...),
		},
	}

	return podtemplate.Apply(base, workPool.Spec.DeploymentTemplate, deploymentTemplateField, podtemplate.Protected{
		Labels:  slices.Collect(maps.Keys(workPool.WorkerLabels())),
		Volumes: []string{constants.PrefectDataVolumeName},
		Containers: map[string]podtemplate.ProtectedContainer{
			prefectiov1.ContainerNameWorker: {
				VolumeMounts: []string{constants.PrefectDataVolumeName},
				Env:          workPool.DerivedEnvVarNames(),
			},
		},
	})
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

	// Drift detection: re-check Prefect once the resync interval has elapsed so
	// out-of-band edits/deletes are corrected.
	if workPool.Status.LastSyncTime == nil {
		return true
	}

	return time.Since(workPool.Status.LastSyncTime.Time) >= r.resyncInterval(workPool)
}

func (r *PrefectWorkPoolReconciler) syncWithPrefect(ctx context.Context, workPool *prefectiov1.PrefectWorkPool, baseJobTemplateConfigMap *corev1.ConfigMap) error {
	name := workPool.Name
	log := log.FromContext(ctx)

	prefectClient, err := r.NewPrefectClient(ctx, &workPool.Spec.Server, workPool.Namespace)
	if err != nil {
		log.Error(err, "Failed to create Prefect client", "workPool", name)
		return r.syncFailed(workPool, "PrefectClientError", err)
	}

	prefectWorkPool, err := prefectClient.GetWorkPool(ctx, name)
	if err != nil {
		log.Error(err, "Failed to get work pool in Prefect", "workPool", name)
		return r.syncFailed(workPool, "SyncError", err)
	}

	// A pool that exists before this resource's first sync was created by
	// something else. A resource that synced before status.adopted existed
	// has a status ID, so it's recorded as having created its pool, and
	// deleting it still deletes the pool, as it did before.
	adopted := prefectWorkPool != nil && (workPool.Status.Id == nil || *workPool.Status.Id == "")

	var baseJobTemplate []byte

	if baseJobTemplateConfigMap.Name != "" {
		key := workPool.Spec.BaseJobTemplate.ConfigMap.Key

		baseJobTemplateJson, exists := baseJobTemplateConfigMap.Data[key]
		if !exists {
			return r.syncFailed(workPool, "BaseJobTemplateError", fmt.Errorf("can't find key %s in ConfigMap %s", key, baseJobTemplateConfigMap.Name))
		}

		baseJobTemplate = []byte(baseJobTemplateJson)
	}

	if prefectWorkPool == nil {
		workPoolSpec, err := prefect.ConvertToWorkPoolSpec(ctx, workPool, baseJobTemplate, prefectClient)
		if err != nil {
			log.Error(err, "Failed to convert work pool spec", "workPool", name)
			return r.syncFailed(workPool, "ConversionError", err)
		}

		prefectWorkPool, err = prefectClient.CreateWorkPool(ctx, workPoolSpec)
		if err != nil {
			log.Error(err, "Failed to create work pool in Prefect", "workPool", name)
			return r.syncFailed(workPool, "SyncError", err)
		}
	} else {
		workPoolSpec, err := prefect.ConvertToWorkPoolUpdateSpec(ctx, workPool, baseJobTemplate, prefectClient)
		if err != nil {
			log.Error(err, "Failed to convert work pool spec", "workPool", name)
			return r.syncFailed(workPool, "ConversionError", err)
		}

		err = prefectClient.UpdateWorkPool(ctx, workPool.Name, workPoolSpec)
		if err != nil {
			log.Error(err, "Failed to update work pool in Prefect", "workPool", name)
			return r.syncFailed(workPool, "SyncError", err)
		}
	}

	prefect.UpdateWorkPoolStatus(workPool, prefectWorkPool)

	// Adoption is decided once. A pool recreated after an out-of-band delete
	// keeps the first decision.
	if workPool.Status.Adopted == nil {
		workPool.Status.Adopted = &adopted
		if adopted {
			log.Info("Adopted existing work pool", "prefectId", prefectWorkPool.ID)
		}
	}

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

// syncFailed records why a sync failed in the Synced condition, which
// Reconcile saves on its way out, and in a Warning event, and returns err.
func (r *PrefectWorkPoolReconciler) syncFailed(workPool *prefectiov1.PrefectWorkPool, reason string, err error) error {
	r.setCondition(workPool, PrefectWorkPoolConditionSynced, metav1.ConditionFalse, reason, err.Error())
	r.Recorder.Eventf(workPool, nil, corev1.EventTypeWarning, reason, eventActionSync, "%v", err)
	return err
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

// handleDeletion deletes the work pool in Prefect, if this resource created
// it, and removes the finalizer. An adopted pool was created by something
// else, such as a person in the Prefect UI, and may run work that this
// resource doesn't know about, so it's left in place.
func (r *PrefectWorkPoolReconciler) handleDeletion(ctx context.Context, workPool *prefectiov1.PrefectWorkPool) (ctrl.Result, error) {
	var deleteFromPrefect func(context.Context) error
	switch {
	case workPool.Status.Adopted != nil && *workPool.Status.Adopted:
		log.FromContext(ctx).Info("Leaving adopted work pool in place in Prefect")
	case workPool.Status.Id != nil && *workPool.Status.Id != "":
		deleteFromPrefect = func(ctx context.Context) error {
			prefectClient, err := r.NewPrefectClient(ctx, &workPool.Spec.Server, workPool.Namespace)
			if err != nil {
				return fmt.Errorf("creating Prefect client: %w", err)
			}
			return prefectClient.DeleteWorkPool(ctx, workPool.Name)
		}
	}
	return finalize(ctx, r.Client, r.Recorder, workPool, PrefectWorkPoolFinalizer, deleteFromPrefect)
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
