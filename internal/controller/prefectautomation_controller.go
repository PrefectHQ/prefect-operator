package controller

import (
	"context"
	"errors"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	prefectiov1 "github.com/PrefectHQ/prefect-operator/api/v1"
	"github.com/PrefectHQ/prefect-operator/internal/prefect"
	"github.com/PrefectHQ/prefect-operator/internal/utils"
)

const (
	// PrefectAutomationFinalizer ensures cleanup of the Prefect automation
	PrefectAutomationFinalizer = "prefect.io/automation-cleanup"

	// PrefectAutomationConditionReady indicates the automation is ready
	PrefectAutomationConditionReady = "Ready"

	// PrefectAutomationConditionSynced indicates the automation is synced with the Prefect API
	PrefectAutomationConditionSynced = "Synced"
)

// PrefectAutomationReconciler reconciles a PrefectAutomation object
type PrefectAutomationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	// NewPrefectClient returns a client for the Prefect server that a
	// resource references.
	NewPrefectClient prefect.ClientFactory
	// DefaultResyncInterval is the fallback drift-detection interval used when a
	// PrefectAutomation does not set spec.interval.
	DefaultResyncInterval time.Duration
	// Recorder emits Kubernetes events on the resources this controller
	// reconciles.
	Recorder events.EventRecorder
}

// resyncInterval returns the effective drift-detection interval for the
// automation: its spec.interval when set, otherwise the operator default.
func (r *PrefectAutomationReconciler) resyncInterval(automation *prefectiov1.PrefectAutomation) time.Duration {
	return utils.ResyncInterval(automation.Spec.Interval, r.DefaultResyncInterval)
}

//+kubebuilder:rbac:groups=prefect.io,resources=prefectautomations,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=prefect.io,resources=prefectautomations/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=prefect.io,resources=prefectautomations/finalizers,verbs=update
//+kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch

// Reconcile handles the reconciliation of a PrefectAutomation
func (r *PrefectAutomationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.V(1).Info("Reconciling PrefectAutomation", "request", req)

	var automation prefectiov1.PrefectAutomation
	if err := r.Get(ctx, req.NamespacedName, &automation); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get PrefectAutomation", "request", req)
		return ctrl.Result{}, err
	}

	// Handle deletion
	if automation.DeletionTimestamp != nil {
		return r.handleDeletion(ctx, &automation)
	}

	// Ensure finalizer is present
	if !controllerutil.ContainsFinalizer(&automation, PrefectAutomationFinalizer) {
		controllerutil.AddFinalizer(&automation, PrefectAutomationFinalizer)
		if err := r.Update(ctx, &automation); err != nil {
			log.Error(err, "Failed to add finalizer", "automation", automation.Name)
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: time.Second}, nil
	}

	if err := automation.Validate(); err != nil {
		r.notSynced(ctx, &automation, "InvalidSpec", err.Error())
		return ctrl.Result{}, nil
	}

	specHash, err := utils.Hash(automation.Spec, 16)
	if err != nil {
		log.Error(err, "Failed to calculate spec hash", "automation", automation.Name)
		return ctrl.Result{}, err
	}

	if r.needsSync(&automation, specHash) {
		log.Info("Starting sync with Prefect API", "automation", automation.Name)
		return r.syncWithPrefect(ctx, &automation)
	}

	return ctrl.Result{RequeueAfter: utils.NextResyncDelay(automation.Status.LastSyncTime, r.resyncInterval(&automation))}, nil
}

// needsSync determines if the automation needs to be synced with the Prefect API
func (r *PrefectAutomationReconciler) needsSync(automation *prefectiov1.PrefectAutomation, currentSpecHash string) bool {
	if automation.Status.Id == nil || *automation.Status.Id == "" {
		return true
	}
	if automation.Status.SpecHash != currentSpecHash {
		return true
	}
	if automation.Status.ObservedGeneration < automation.Generation {
		return true
	}
	// Drift detection: re-check Prefect once the resync interval has elapsed so
	// out-of-band edits/deletes are corrected.
	if automation.Status.LastSyncTime == nil {
		return true
	}
	return time.Since(automation.Status.LastSyncTime.Time) >= r.resyncInterval(automation)
}

// syncWithPrefect creates or updates the automation in the Prefect API
func (r *PrefectAutomationReconciler) syncWithPrefect(ctx context.Context, automation *prefectiov1.PrefectAutomation) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	prefectClient, err := r.NewPrefectClient(ctx, &automation.Spec.Server, automation.Namespace)
	if err != nil {
		log.Error(err, "Failed to create Prefect client", "automation", automation.Name)
		r.notSynced(ctx, automation, "PrefectClientError", err.Error())
		return ctrl.Result{}, err
	}

	// Resolve any deploymentName references to deployment IDs. If a referenced
	// deployment doesn't exist yet (e.g. its PrefectDeployment hasn't reconciled),
	// requeue until it does. Any other failure, including a name that matches
	// more than one deployment, is a sync error.
	deploymentIDs := map[string]string{}
	for _, name := range prefect.DeploymentNamesReferenced(automation) {
		dep, derr := prefectClient.FindDeploymentByName(ctx, name)
		if derr != nil && !errors.Is(derr, prefect.ErrNotFound) {
			log.Error(derr, "Failed to resolve referenced deployment", "automation", automation.Name, "deployment", name)
			r.notSynced(ctx, automation, "SyncError", derr.Error())
			return ctrl.Result{RequeueAfter: RequeueIntervalError}, nil
		}
		if derr != nil {
			log.V(1).Info("Referenced deployment not found, requeuing", "automation", automation.Name, "deployment", name)
			r.notSynced(ctx, automation, "DeploymentNotFound", fmt.Sprintf("referenced deployment %q not found yet", name))
			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
		}
		deploymentIDs[name] = dep.ID
	}

	automationSpec, err := prefect.ConvertToAutomationSpec(automation, deploymentIDs)
	if err != nil {
		log.Error(err, "Failed to convert automation spec", "automation", automation.Name)
		r.notSynced(ctx, automation, "ConversionError", err.Error())
		return ctrl.Result{}, nil
	}

	var result *prefect.Automation
	if automation.Status.Id != nil && *automation.Status.Id != "" {
		// Skip the update when nothing changed: every update resets the trigger's
		// server-assigned ID, which discards the trigger's proactive state.
		remote, getErr := prefectClient.GetAutomation(ctx, *automation.Status.Id)
		if getErr == nil && prefect.AutomationUpToDate(remote, automationSpec) {
			log.Info("Prefect automation already up to date, skipping update", "automation", automation.Name)
			result = remote
		} else {
			result, err = prefectClient.UpdateAutomation(ctx, *automation.Status.Id, automationSpec)
		}
		// If the automation was deleted out-of-band, clear the stale ID and
		// requeue so the next pass recreates it instead of looping on SyncError.
		if errors.Is(err, prefect.ErrNotFound) {
			log.Info("Automation no longer exists in Prefect, recreating", "automation", automation.Name, "prefectId", *automation.Status.Id)
			automation.Status.Id = nil
			r.notSynced(ctx, automation, "Recreating", "Automation was deleted in Prefect; recreating")
			return ctrl.Result{RequeueAfter: time.Second}, nil
		}
	} else {
		result, err = prefectClient.CreateAutomation(ctx, automationSpec)
	}
	if err != nil {
		log.Error(err, "Failed to sync automation with Prefect", "automation", automation.Name)
		r.notSynced(ctx, automation, "SyncError", err.Error())
		return ctrl.Result{RequeueAfter: RequeueIntervalError}, nil
	}

	prefect.UpdateAutomationStatus(automation, result)
	automation.Status.SpecHash, err = utils.Hash(automation.Spec, 16)
	if err != nil {
		log.Error(err, "Failed to calculate spec hash", "automation", automation.Name)
		return ctrl.Result{}, err
	}
	automation.Status.ObservedGeneration = automation.Generation
	// Stamp the sync time so needsSync gates the next Prefect re-check by the
	// resync interval instead of hitting the API on every reconcile.
	now := metav1.Now()
	automation.Status.LastSyncTime = &now

	r.setCondition(automation, PrefectAutomationConditionSynced, metav1.ConditionTrue, "SyncSuccessful", "Automation successfully synced with Prefect API")
	r.setCondition(automation, PrefectAutomationConditionReady, metav1.ConditionTrue, "AutomationReady", "Automation is ready and operational")

	if err := r.Status().Update(ctx, automation); err != nil {
		log.Error(err, "Failed to update automation status", "automation", automation.Name)
		return ctrl.Result{}, err
	}

	log.Info("Successfully synced automation with Prefect", "automationId", result.ID)
	return ctrl.Result{RequeueAfter: utils.JitterResyncInterval(r.resyncInterval(automation))}, nil
}

// notSynced records why the automation is not synced in the Synced condition
// and a Warning event, marks it not ready, and saves the status so both show
// on the resource.
func (r *PrefectAutomationReconciler) notSynced(ctx context.Context, automation *prefectiov1.PrefectAutomation, reason, message string) {
	r.setCondition(automation, PrefectAutomationConditionSynced, metav1.ConditionFalse, reason, message)
	r.Recorder.Eventf(automation, nil, corev1.EventTypeWarning, reason, eventActionSync, "%s", message)
	automation.Status.Ready = false
	if err := r.Status().Update(ctx, automation); err != nil {
		log.FromContext(ctx).Error(err, "Failed to update automation status", "automation", automation.Name)
	}
}

func (r *PrefectAutomationReconciler) setCondition(automation *prefectiov1.PrefectAutomation, conditionType string, status metav1.ConditionStatus, reason, message string) {
	meta.SetStatusCondition(&automation.Status.Conditions, metav1.Condition{
		Type:               conditionType,
		Status:             status,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	})
}

// handleDeletion deletes the automation in Prefect, if a sync created it, and
// removes the finalizer.
func (r *PrefectAutomationReconciler) handleDeletion(ctx context.Context, automation *prefectiov1.PrefectAutomation) (ctrl.Result, error) {
	var deleteFromPrefect func(context.Context) error
	if id := automation.Status.Id; id != nil && *id != "" {
		deleteFromPrefect = func(ctx context.Context) error {
			prefectClient, err := r.NewPrefectClient(ctx, &automation.Spec.Server, automation.Namespace)
			if err != nil {
				return fmt.Errorf("creating Prefect client: %w", err)
			}
			return prefectClient.DeleteAutomation(ctx, *id)
		}
	}
	return finalize(ctx, r.Client, r.Recorder, automation, PrefectAutomationFinalizer, deleteFromPrefect)
}

// SetupWithManager sets up the controller with the Manager.
func (r *PrefectAutomationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&prefectiov1.PrefectAutomation{}).
		Complete(r)
}
