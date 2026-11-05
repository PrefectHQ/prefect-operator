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
	"errors"
	"fmt"
	"reflect"
	"slices"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	prefectiov1 "github.com/PrefectHQ/prefect-operator/api/v1"
	"github.com/PrefectHQ/prefect-operator/internal/prefect"
	"github.com/PrefectHQ/prefect-operator/internal/utils"
)

const (
	// PrefectDeploymentFinalizer is the finalizer used to ensure cleanup of Prefect deployments
	PrefectDeploymentFinalizer = "prefect.io/deployment-cleanup"

	// PrefectDeploymentConditionReady indicates the deployment is ready
	PrefectDeploymentConditionReady = "Ready"

	// PrefectDeploymentConditionSynced indicates the deployment is synced with Prefect API
	PrefectDeploymentConditionSynced = "Synced"

	// PrefectDeploymentConditionServerAvailable indicates the Prefect server is available
	PrefectDeploymentConditionServerAvailable = "ServerAvailable"

	// PrefectDeploymentConditionWorkPoolAvailable indicates the referenced work pool is available
	PrefectDeploymentConditionWorkPoolAvailable = "WorkPoolAvailable"

	// RequeueIntervalError is the interval for requeuing on errors
	RequeueIntervalError = 30 * time.Second
)

// PrefectDeploymentReconciler reconciles a PrefectDeployment object
type PrefectDeploymentReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	// NewPrefectClient returns a client for the Prefect server that a
	// resource references.
	NewPrefectClient prefect.ClientFactory
	// DefaultResyncInterval is the fallback drift-detection interval used when a
	// PrefectDeployment does not set spec.interval.
	DefaultResyncInterval time.Duration
}

// resyncInterval returns the effective drift-detection interval for the
// deployment: its spec.interval when set, otherwise the operator default.
func (r *PrefectDeploymentReconciler) resyncInterval(deployment *prefectiov1.PrefectDeployment) time.Duration {
	return utils.ResyncInterval(deployment.Spec.Interval, r.DefaultResyncInterval)
}

//+kubebuilder:rbac:groups=prefect.io,resources=prefectdeployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=prefect.io,resources=prefectdeployments/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=prefect.io,resources=prefectdeployments/finalizers,verbs=update

// Reconcile handles the reconciliation of a PrefectDeployment
func (r *PrefectDeploymentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.V(1).Info("Reconciling PrefectDeployment", "request", req)

	var deployment prefectiov1.PrefectDeployment
	if err := r.Get(ctx, req.NamespacedName, &deployment); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("PrefectDeployment not found, ignoring", "request", req)
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get PrefectDeployment", "request", req)
		return ctrl.Result{}, err
	}

	// Handle deletion
	if deployment.DeletionTimestamp != nil {
		return r.handleDeletion(ctx, &deployment)
	}

	// Ensure finalizer is present
	if !controllerutil.ContainsFinalizer(&deployment, PrefectDeploymentFinalizer) {
		controllerutil.AddFinalizer(&deployment, PrefectDeploymentFinalizer)
		if err := r.Update(ctx, &deployment); err != nil {
			log.Error(err, "Failed to add finalizer", "deployment", deployment.Name)
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: time.Second}, nil
	}

	specHash, err := utils.Hash(deployment.Spec, 16)
	if err != nil {
		log.Error(err, "Failed to calculate spec hash", "deployment", deployment.Name)
		return ctrl.Result{}, err
	}

	if r.needsSync(&deployment, specHash) {
		log.Info("Starting sync with Prefect API", "deployment", deployment.Name)
		result, err := r.syncWithPrefect(ctx, &deployment)
		if err != nil {
			return result, err
		}
		return result, nil
	}

	return ctrl.Result{RequeueAfter: utils.NextResyncDelay(deployment.Status.LastSyncTime, r.resyncInterval(&deployment))}, nil
}

// needsSync determines if the deployment needs to be synced with Prefect API
func (r *PrefectDeploymentReconciler) needsSync(deployment *prefectiov1.PrefectDeployment, currentSpecHash string) bool {
	if deployment.Status.Id == nil || *deployment.Status.Id == "" {
		return true
	}

	if deployment.Status.SpecHash != currentSpecHash {
		return true
	}

	if deployment.Status.ObservedGeneration < deployment.Generation {
		return true
	}

	// Drift detection: re-check Prefect once the resync interval has elapsed so
	// out-of-band edits/deletes are corrected.
	if deployment.Status.LastSyncTime == nil {
		return true
	}

	return time.Since(deployment.Status.LastSyncTime.Time) >= r.resyncInterval(deployment)
}

// syncWithPrefect syncs the deployment with the Prefect API
func (r *PrefectDeploymentReconciler) syncWithPrefect(ctx context.Context, deployment *prefectiov1.PrefectDeployment) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	prefectClient, err := r.NewPrefectClient(ctx, &deployment.Spec.Server, deployment.Namespace)
	if err != nil {
		log.Error(err, "Failed to create Prefect client", "deployment", deployment.Name)
		return ctrl.Result{}, err
	}

	// Requeue until the referenced work pool exists (e.g. its PrefectWorkPool
	// hasn't reconciled yet). Any other failure of the lookup is a sync error.
	workPool, err := prefectClient.GetWorkPool(ctx, deployment.Spec.WorkPool.Name)
	if err != nil {
		log.Error(err, "Failed to get work pool from Prefect", "deployment", deployment.Name, "workPool", deployment.Spec.WorkPool.Name)
		return r.syncFailed(ctx, deployment, "SyncError", err)
	}
	if workPool == nil {
		log.V(1).Info("Work pool not found, requeuing", "deployment", deployment.Name, "workPool", deployment.Spec.WorkPool.Name)
		r.notSynced(ctx, deployment, "WorkPoolNotFound", fmt.Sprintf("Work pool '%s' not found", deployment.Spec.WorkPool.Name))
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	flowID, err := prefect.GetFlowIDFromDeployment(ctx, prefectClient, deployment)
	if err != nil {
		log.Error(err, "Failed to get flow ID", "deployment", deployment.Name)
		return r.syncFailed(ctx, deployment, "FlowIDError", err)
	}

	deploymentSpec, err := prefect.ConvertToDeploymentSpec(deployment, flowID)
	if err != nil {
		log.Error(err, "Failed to convert deployment spec", "deployment", deployment.Name)
		return r.syncFailed(ctx, deployment, "ConversionError", err)
	}

	desiredSchedules := deploymentSpec.Schedules
	deploymentSpec.Schedules = nil

	// Fields declared on the last sync (status.appliedFields) but removed from
	// the spec are cleared in Prefect instead of keeping their old value.
	declared := declaredDeploymentFields(deployment.Spec)
	var clears []string
	for _, f := range deployment.Status.AppliedFields {
		if !slices.Contains(declared, f) {
			clears = append(clears, f)
		}
	}

	var prefectDeployment *prefect.Deployment
	if deployment.Status.Id != nil && *deployment.Status.Id != "" {
		// Skip the update when nothing changed: every update deletes the
		// deployment's future auto-scheduled runs.
		remote, getErr := prefectClient.GetDeployment(ctx, *deployment.Status.Id)
		if getErr == nil && prefect.DeploymentUpToDate(remote, deploymentSpec) && prefect.DeploymentClearsApplied(remote, clears) {
			log.Info("Prefect deployment already up to date, skipping update", "deployment", deployment.Name)
			prefectDeployment = remote
		} else {
			prefectDeployment, err = prefectClient.UpdateDeployment(ctx, *deployment.Status.Id, deploymentSpec, clears)
			if errors.Is(err, prefect.ErrNotFound) {
				prefectDeployment, err = prefectClient.CreateOrUpdateDeployment(ctx, deploymentSpec)
			}
		}
	} else {
		prefectDeployment, err = prefectClient.CreateOrUpdateDeployment(ctx, deploymentSpec)
	}
	if err != nil {
		log.Error(err, "Failed to create or update deployment in Prefect", "deployment", deployment.Name)
		return r.syncFailed(ctx, deployment, "SyncError", err)
	}

	if err := r.reconcileSchedules(ctx, prefectClient, prefectDeployment.ID, desiredSchedules); err != nil {
		log.Error(err, "Failed to reconcile deployment schedules", "deployment", deployment.Name)
		return r.syncFailed(ctx, deployment, "ScheduleSyncError", err)
	}

	prefect.UpdateDeploymentStatus(deployment, prefectDeployment)

	specHash, err := utils.Hash(deployment.Spec, 16)
	if err != nil {
		log.Error(err, "Failed to calculate spec hash", "deployment", deployment.Name)
		return ctrl.Result{}, err
	}
	deployment.Status.SpecHash = specHash
	deployment.Status.AppliedFields = declared
	deployment.Status.ObservedGeneration = deployment.Generation
	// Stamp the sync time so needsSync gates the next Prefect re-check by the
	// resync interval instead of hitting the API on every reconcile.
	now := metav1.Now()
	deployment.Status.LastSyncTime = &now

	r.setCondition(deployment, PrefectDeploymentConditionSynced, metav1.ConditionTrue, "SyncSuccessful", "Deployment successfully synced with Prefect API")
	r.setCondition(deployment, PrefectDeploymentConditionReady, metav1.ConditionTrue, "DeploymentReady", "Deployment is ready and operational")

	if err := r.Status().Update(ctx, deployment); err != nil {
		log.Error(err, "Failed to update deployment status", "deployment", deployment.Name)
		return ctrl.Result{}, err
	}

	log.Info("Successfully synced deployment with Prefect", "deploymentId", prefectDeployment.ID)
	return ctrl.Result{RequeueAfter: utils.JitterResyncInterval(r.resyncInterval(deployment))}, nil
}

func (r *PrefectDeploymentReconciler) reconcileSchedules(ctx context.Context, client *prefect.Client, deploymentID string, desired []prefect.DeploymentSchedule) error {
	current, err := client.GetDeployment(ctx, deploymentID)
	if err != nil {
		return fmt.Errorf("get current schedules: %w", err)
	}
	if current == nil {
		return fmt.Errorf("get current schedules: deployment %s: %w", deploymentID, prefect.ErrNotFound)
	}

	desiredBySlug := make(map[string]prefect.DeploymentSchedule, len(desired))
	var toCreate []prefect.DeploymentSchedule
	for _, d := range desired {
		if d.Slug == nil {
			toCreate = append(toCreate, d)
			continue
		}
		desiredBySlug[*d.Slug] = d
	}

	for _, e := range current.Schedules {
		d, ok := desiredBySlug[slugValue(e.Slug)]
		if e.Slug == nil || !ok {
			if err := client.DeleteDeploymentSchedule(ctx, deploymentID, e.ID); err != nil {
				return fmt.Errorf("delete schedule: %w", err)
			}
			continue
		}
		delete(desiredBySlug, *e.Slug)
		d = withScheduleDefaults(d)
		if e.MaxScheduledRuns != nil && d.MaxScheduledRuns == nil {
			// Prefect ignores a null in a schedule update, so clearing the
			// limit means replacing the schedule.
			if err := client.DeleteDeploymentSchedule(ctx, deploymentID, e.ID); err != nil {
				return fmt.Errorf("delete schedule: %w", err)
			}
			toCreate = append(toCreate, d)
			continue
		}
		if scheduleNeedsUpdate(e, d) {
			if err := client.UpdateDeploymentSchedule(ctx, deploymentID, e.ID, prefect.DeploymentScheduleUpdate{
				Schedule:         &d.Schedule,
				Active:           d.Active,
				MaxScheduledRuns: d.MaxScheduledRuns,
				Parameters:       d.Parameters,
			}); err != nil {
				return fmt.Errorf("update schedule: %w", err)
			}
		}
	}

	for _, d := range desiredBySlug {
		toCreate = append(toCreate, d)
	}
	if len(toCreate) > 0 {
		if err := client.CreateDeploymentSchedules(ctx, deploymentID, toCreate); err != nil {
			return fmt.Errorf("create schedules: %w", err)
		}
	}
	return nil
}

func slugValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// withScheduleDefaults fills in the values Prefect gives the schedule fields
// that the spec leaves unset, so that removing a field from the spec resets it
// in Prefect. An interval schedule's anchor date stays unset: Prefect defaults
// it to the time the schedule was created, which the spec can't know.
func withScheduleDefaults(s prefect.DeploymentSchedule) prefect.DeploymentSchedule {
	if s.Active == nil {
		s.Active = new(true)
	}
	if s.Parameters == nil {
		s.Parameters = map[string]any{}
	}
	if s.Schedule.Cron != nil && s.Schedule.DayOr == nil {
		s.Schedule.DayOr = new(true)
	}
	// A cron schedule has no timezone by default. Interval and rrule
	// schedules default to UTC.
	if s.Schedule.Timezone == nil && (s.Schedule.Interval != nil || s.Schedule.RRule != nil) {
		s.Schedule.Timezone = new("UTC")
	}
	return s
}

// scheduleNeedsUpdate reports whether a schedule in Prefect differs from the
// desired one, which withScheduleDefaults has filled in.
func scheduleNeedsUpdate(current, desired prefect.DeploymentSchedule) bool {
	c, d := current.Schedule, desired.Schedule
	if d.AnchorDate != nil && (c.AnchorDate == nil || !c.AnchorDate.Equal(*d.AnchorDate)) {
		return true
	}
	return !equalPtr(c.Cron, d.Cron) ||
		!equalPtr(c.RRule, d.RRule) ||
		!equalPtr(c.Interval, d.Interval) ||
		!equalPtr(c.Timezone, d.Timezone) ||
		!equalPtr(c.DayOr, d.DayOr) ||
		!equalPtr(current.Active, desired.Active) ||
		!equalPtr(current.MaxScheduledRuns, desired.MaxScheduledRuns) ||
		!equalParameters(current.Parameters, desired.Parameters)
}

func equalPtr[T comparable](a, b *T) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// equalParameters treats no parameters and an empty set as the same.
func equalParameters(a, b map[string]any) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	return reflect.DeepEqual(a, b)
}

// syncFailed records why a sync failed and returns err so the request is
// retried with backoff.
func (r *PrefectDeploymentReconciler) syncFailed(ctx context.Context, deployment *prefectiov1.PrefectDeployment, reason string, err error) (ctrl.Result, error) {
	r.notSynced(ctx, deployment, reason, err.Error())
	return ctrl.Result{}, err
}

// notSynced records why the deployment is not synced in the Synced condition,
// marks it not ready, and saves the status so both show on the resource.
func (r *PrefectDeploymentReconciler) notSynced(ctx context.Context, deployment *prefectiov1.PrefectDeployment, reason, message string) {
	r.setCondition(deployment, PrefectDeploymentConditionSynced, metav1.ConditionFalse, reason, message)
	deployment.Status.Ready = false
	if err := r.Status().Update(ctx, deployment); err != nil {
		log.FromContext(ctx).Error(err, "Failed to update deployment status", "deployment", deployment.Name)
	}
}

// setCondition sets a condition on the deployment status
func (r *PrefectDeploymentReconciler) setCondition(deployment *prefectiov1.PrefectDeployment, conditionType string, status metav1.ConditionStatus, reason, message string) {
	condition := metav1.Condition{
		Type:               conditionType,
		Status:             status,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	}

	meta.SetStatusCondition(&deployment.Status.Conditions, condition)
}

// declaredDeploymentFields lists the clear-tracked spec fields currently set,
// by their CRD JSON names, for status.appliedFields.
func declaredDeploymentFields(spec prefectiov1.PrefectDeploymentSpec) []string {
	var fields []string
	if spec.Deployment.ConcurrencyLimit != nil {
		fields = append(fields, prefect.DeploymentFieldConcurrencyLimit)
	}
	return fields
}

// handleDeletion handles the cleanup of a PrefectDeployment that is being deleted
func (r *PrefectDeploymentReconciler) handleDeletion(ctx context.Context, deployment *prefectiov1.PrefectDeployment) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("Handling deletion of PrefectDeployment", "deployment", deployment.Name)

	// If finalizer is not present, nothing to do
	if !controllerutil.ContainsFinalizer(deployment, PrefectDeploymentFinalizer) {
		return ctrl.Result{}, nil
	}

	// Only attempt cleanup if we have a deployment ID in Prefect
	if deployment.Status.Id != nil && *deployment.Status.Id != "" {
		// Remove the finalizer even when the Prefect delete fails, so a Prefect
		// outage can't block Kubernetes deletion indefinitely.
		prefectClient, err := r.NewPrefectClient(ctx, &deployment.Spec.Server, deployment.Namespace)
		if err != nil {
			log.Error(err, "Failed to create Prefect client for deletion", "deployment", deployment.Name)
		} else if err := prefectClient.DeleteDeployment(ctx, *deployment.Status.Id); err != nil {
			log.Error(err, "Failed to delete deployment from Prefect API", "deployment", deployment.Name, "prefectId", *deployment.Status.Id)
		} else {
			log.Info("Successfully deleted deployment from Prefect API", "deployment", deployment.Name, "prefectId", *deployment.Status.Id)
		}
	}

	// Remove finalizer to allow Kubernetes to complete deletion
	controllerutil.RemoveFinalizer(deployment, PrefectDeploymentFinalizer)
	if err := r.Update(ctx, deployment); err != nil {
		log.Error(err, "Failed to remove finalizer", "deployment", deployment.Name)
		return ctrl.Result{}, err
	}

	log.Info("Finalizer removed, deletion will proceed", "deployment", deployment.Name)
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PrefectDeploymentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&prefectiov1.PrefectDeployment{}).
		Complete(r)
}
