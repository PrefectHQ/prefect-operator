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
	// PrefectWorkQueueFinalizer ensures cleanup of the Prefect work queue
	PrefectWorkQueueFinalizer = "prefect.io/workqueue-cleanup"

	// PrefectWorkQueueConditionReady indicates the work queue is ready
	PrefectWorkQueueConditionReady = "Ready"

	// PrefectWorkQueueConditionSynced indicates the work queue is synced with the Prefect API
	PrefectWorkQueueConditionSynced = "Synced"
)

// PrefectWorkQueueReconciler reconciles a PrefectWorkQueue object
type PrefectWorkQueueReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	PrefectClient prefect.PrefectClient
	// DefaultResyncInterval is the fallback drift-detection interval used when a
	// PrefectWorkQueue does not set spec.interval.
	DefaultResyncInterval time.Duration
}

// resyncInterval returns the effective drift-detection interval for the
// work queue: its spec.interval when set, otherwise the operator default.
func (r *PrefectWorkQueueReconciler) resyncInterval(workQueue *prefectiov1.PrefectWorkQueue) time.Duration {
	return utils.ResyncInterval(workQueue.Spec.Interval, r.DefaultResyncInterval)
}

//+kubebuilder:rbac:groups=prefect.io,resources=prefectworkqueues,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=prefect.io,resources=prefectworkqueues/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=prefect.io,resources=prefectworkqueues/finalizers,verbs=update

// Reconcile handles the reconciliation of a PrefectWorkQueue
func (r *PrefectWorkQueueReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.V(1).Info("Reconciling PrefectWorkQueue", "request", req)

	var workQueue prefectiov1.PrefectWorkQueue
	if err := r.Get(ctx, req.NamespacedName, &workQueue); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get PrefectWorkQueue", "request", req)
		return ctrl.Result{}, err
	}

	// Handle deletion
	if workQueue.DeletionTimestamp != nil {
		return r.handleDeletion(ctx, &workQueue)
	}

	// Ensure finalizer is present
	if !controllerutil.ContainsFinalizer(&workQueue, PrefectWorkQueueFinalizer) {
		controllerutil.AddFinalizer(&workQueue, PrefectWorkQueueFinalizer)
		if err := r.Update(ctx, &workQueue); err != nil {
			log.Error(err, "Failed to add finalizer", "workQueue", workQueue.Name)
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: time.Second}, nil
	}

	specHash, err := utils.Hash(workQueue.Spec, 16)
	if err != nil {
		log.Error(err, "Failed to calculate spec hash", "workQueue", workQueue.Name)
		return ctrl.Result{}, err
	}

	if r.needsSync(&workQueue, specHash) {
		log.Info("Starting sync with Prefect API", "workQueue", workQueue.Name)
		return r.syncWithPrefect(ctx, &workQueue)
	}

	return ctrl.Result{RequeueAfter: utils.NextResyncDelay(workQueue.Status.LastSyncTime, r.resyncInterval(&workQueue))}, nil
}

// needsSync determines if the work queue needs to be synced with the Prefect API
func (r *PrefectWorkQueueReconciler) needsSync(workQueue *prefectiov1.PrefectWorkQueue, currentSpecHash string) bool {
	if workQueue.Status.Id == nil || *workQueue.Status.Id == "" {
		return true
	}
	if workQueue.Status.SpecHash != currentSpecHash {
		return true
	}
	if workQueue.Status.ObservedGeneration < workQueue.Generation {
		return true
	}
	// Drift detection: re-check Prefect once the resync interval has elapsed so
	// out-of-band edits/deletes are corrected.
	if workQueue.Status.LastSyncTime == nil {
		return true
	}
	return time.Since(workQueue.Status.LastSyncTime.Time) >= r.resyncInterval(workQueue)
}

// syncWithPrefect creates, adopts, or updates the work queue in the Prefect API.
// The queue is keyed by (workPoolName, name): each sync reads it by name,
// creates it when absent (which also recreates after an out-of-band delete),
// and PATCHes only when a declared field drifts or a previously-declared field
// was removed and needs its default restored.
func (r *PrefectWorkQueueReconciler) syncWithPrefect(ctx context.Context, workQueue *prefectiov1.PrefectWorkQueue) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	prefectClient := r.PrefectClient
	if prefectClient == nil {
		var err error
		prefectClient, err = prefect.NewClientFromK8s(ctx, &workQueue.Spec.Server, r.Client, workQueue.Namespace, log)
		if err != nil {
			log.Error(err, "Failed to create Prefect client", "workQueue", workQueue.Name)
			return ctrl.Result{}, err
		}
	}

	spec := &prefect.WorkQueueSpec{
		Name:             workQueue.Spec.Name,
		Description:      workQueue.Spec.Description,
		IsPaused:         workQueue.Spec.IsPaused,
		ConcurrencyLimit: workQueue.Spec.ConcurrencyLimit,
		Priority:         workQueue.Spec.Priority,
	}

	// A spec.name change switches which queue is managed: adoption is decided
	// again and pending clears (applied to the old queue) don't carry over.
	renamed := workQueue.Status.ManagedName != "" && workQueue.Status.ManagedName != workQueue.Spec.Name
	adoptionDecided := workQueue.Status.Adopted != nil && !renamed

	remote, err := prefectClient.GetWorkQueue(ctx, workQueue.Spec.WorkPoolName, workQueue.Spec.Name)
	adopted := false
	if err == nil && remote == nil {
		remote, err = prefectClient.CreateWorkQueue(ctx, workQueue.Spec.WorkPoolName, spec)
		if errors.Is(err, prefect.ErrWorkPoolNotFound) {
			// The pool hasn't been created yet (e.g. its PrefectWorkPool hasn't
			// reconciled); requeue until it exists, mirroring how the automation
			// controller waits for a referenced deployment.
			log.V(1).Info("Work pool not found, requeuing", "workQueue", workQueue.Name, "workPool", workQueue.Spec.WorkPoolName)
			r.setCondition(workQueue, PrefectWorkQueueConditionSynced, metav1.ConditionFalse, "WorkPoolNotFound", err.Error())
			workQueue.Status.Ready = false
			if updateErr := r.Status().Update(ctx, workQueue); updateErr != nil {
				log.Error(updateErr, "Failed to update work queue status", "workQueue", workQueue.Name)
			}
			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
		}
		if err == nil {
			log.Info("Created work queue", "workQueue", workQueue.Name, "prefectId", remote.ID)
		}
	} else if err == nil && !adoptionDecided {
		// The queue already exists — created implicitly by a deployment
		// referencing it, or previously managed. Adopt it rather than fail.
		adopted = true
		log.Info("Adopted existing work queue", "workQueue", workQueue.Name, "prefectId", remote.ID)
	}

	// Fields removed from the spec since the last sync are reset to their
	// create-time defaults. Priority is excluded: it has no default to restore
	// (Prefect keeps pool priorities unique and sequential), so removing it
	// keeps the last value.
	declared := declaredWorkQueueFields(workQueue.Spec)
	var clears []string
	if !renamed {
		for _, f := range workQueue.Status.AppliedFields {
			if f != prefect.WorkQueueFieldPriority && !slices.Contains(declared, f) {
				clears = append(clears, f)
			}
		}
	}

	if err == nil && (len(clears) > 0 || !workQueueMatches(workQueue.Spec, remote)) {
		err = prefectClient.UpdateWorkQueue(ctx, workQueue.Spec.WorkPoolName, workQueue.Spec.Name, spec, clears)
		if errors.Is(err, prefect.ErrWorkQueueNotFound) {
			// Deleted between the GET and the PATCH; the next pass recreates it.
			r.setCondition(workQueue, PrefectWorkQueueConditionSynced, metav1.ConditionFalse, "Recreating", "Work queue was deleted in Prefect; recreating")
			workQueue.Status.Ready = false
			if updateErr := r.Status().Update(ctx, workQueue); updateErr != nil {
				log.Error(updateErr, "Failed to update work queue status", "workQueue", workQueue.Name)
			}
			return ctrl.Result{RequeueAfter: time.Second}, nil
		}
		if err == nil {
			log.Info("Updated work queue", "workQueue", workQueue.Name, "prefectId", remote.ID, "cleared", clears)
		}
	}

	if err != nil {
		log.Error(err, "Failed to sync work queue with Prefect", "workQueue", workQueue.Name)
		r.setCondition(workQueue, PrefectWorkQueueConditionSynced, metav1.ConditionFalse, "SyncError", err.Error())
		workQueue.Status.Ready = false
		if updateErr := r.Status().Update(ctx, workQueue); updateErr != nil {
			log.Error(updateErr, "Failed to update work queue status", "workQueue", workQueue.Name)
		}
		return ctrl.Result{RequeueAfter: RequeueIntervalError}, nil
	}

	// Both err-free paths above guarantee remote is non-nil: the create branch
	// only clears err when CreateWorkQueue returned a queue, and the adopt
	// branch required remote != nil to be taken.
	workQueue.Status.Id = &remote.ID
	if !adoptionDecided {
		workQueue.Status.Adopted = &adopted
	}
	workQueue.Status.ManagedName = workQueue.Spec.Name
	workQueue.Status.AppliedFields = declared
	workQueue.Status.Ready = true
	workQueue.Status.SpecHash, err = utils.Hash(workQueue.Spec, 16)
	if err != nil {
		log.Error(err, "Failed to calculate spec hash", "workQueue", workQueue.Name)
		return ctrl.Result{}, err
	}
	workQueue.Status.ObservedGeneration = workQueue.Generation
	// Stamp the sync time so needsSync gates the next Prefect re-check by the
	// resync interval instead of hitting the API on every reconcile.
	now := metav1.Now()
	workQueue.Status.LastSyncTime = &now

	r.setCondition(workQueue, PrefectWorkQueueConditionSynced, metav1.ConditionTrue, "SyncSuccessful", "Work queue successfully synced with Prefect API")
	r.setCondition(workQueue, PrefectWorkQueueConditionReady, metav1.ConditionTrue, "WorkQueueReady", "Work queue is ready and operational")

	if err := r.Status().Update(ctx, workQueue); err != nil {
		log.Error(err, "Failed to update work queue status", "workQueue", workQueue.Name)
		return ctrl.Result{}, err
	}

	log.Info("Successfully synced work queue with Prefect", "workQueueId", remote.ID)
	return ctrl.Result{RequeueAfter: utils.JitterResyncInterval(r.resyncInterval(workQueue))}, nil
}

// declaredWorkQueueFields lists the optional spec fields currently set, by
// their CRD JSON names. Recorded in status so the next sync can tell a field
// that was never declared from one that was declared and then removed.
func declaredWorkQueueFields(spec prefectiov1.PrefectWorkQueueSpec) []string {
	var fields []string
	if spec.ConcurrencyLimit != nil {
		fields = append(fields, prefect.WorkQueueFieldConcurrencyLimit)
	}
	if spec.Priority != nil {
		fields = append(fields, prefect.WorkQueueFieldPriority)
	}
	if spec.Description != nil {
		fields = append(fields, prefect.WorkQueueFieldDescription)
	}
	if spec.IsPaused != nil {
		fields = append(fields, prefect.WorkQueueFieldIsPaused)
	}
	return fields
}

// workQueueMatches reports whether the remote queue already has the declared
// values. Fields left unset in the spec are not compared (and not sent); a
// field that was previously declared and then removed is handled separately
// via the applied-fields clear list, so an unset field here never means
// "reset it".
func workQueueMatches(desired prefectiov1.PrefectWorkQueueSpec, remote *prefect.WorkQueue) bool {
	if desired.ConcurrencyLimit != nil &&
		(remote.ConcurrencyLimit == nil || *remote.ConcurrencyLimit != *desired.ConcurrencyLimit) {
		return false
	}
	if desired.Priority != nil && (remote.Priority == nil || *remote.Priority != *desired.Priority) {
		return false
	}
	if desired.Description != nil && (remote.Description == nil || *remote.Description != *desired.Description) {
		return false
	}
	if desired.IsPaused != nil && (remote.IsPaused == nil || *remote.IsPaused != *desired.IsPaused) {
		return false
	}
	return true
}

func (r *PrefectWorkQueueReconciler) setCondition(workQueue *prefectiov1.PrefectWorkQueue, conditionType string, status metav1.ConditionStatus, reason, message string) {
	meta.SetStatusCondition(&workQueue.Status.Conditions, metav1.Condition{
		Type:               conditionType,
		Status:             status,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	})
}

// handleDeletion cleans up the work queue in Prefect and removes the finalizer.
// Only queues this resource CREATED are deleted remotely: an adopted queue was
// created by something else (typically implicitly, by a deployment referencing
// it), and deleting a CR that was only added to put a limit on such a queue
// must not take down the queue and everything routing through it.
func (r *PrefectWorkQueueReconciler) handleDeletion(ctx context.Context, workQueue *prefectiov1.PrefectWorkQueue) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("Handling deletion of PrefectWorkQueue", "workQueue", workQueue.Name)

	if !controllerutil.ContainsFinalizer(workQueue, PrefectWorkQueueFinalizer) {
		return ctrl.Result{}, nil
	}

	adopted := workQueue.Status.Adopted != nil && *workQueue.Status.Adopted
	if adopted {
		log.Info("Leaving adopted work queue in place in Prefect", "workQueue", workQueue.Name, "workPool", workQueue.Spec.WorkPoolName)
	}

	// Delete the queue the last sync managed; after an unsynced rename,
	// spec.name points at a queue this resource never touched.
	managedName := workQueue.Status.ManagedName
	if managedName == "" {
		managedName = workQueue.Spec.Name
	}

	if !adopted && workQueue.Status.Id != nil && *workQueue.Status.Id != "" {
		prefectClient := r.PrefectClient
		if prefectClient == nil {
			var err error
			prefectClient, err = prefect.NewClientFromK8s(ctx, &workQueue.Spec.Server, r.Client, workQueue.Namespace, log)
			if err != nil {
				log.Error(err, "Failed to create Prefect client for deletion", "workQueue", workQueue.Name)
				prefectClient = nil
			}
		}
		if prefectClient != nil {
			if err := prefectClient.DeleteWorkQueue(ctx, workQueue.Spec.WorkPoolName, managedName); err != nil {
				// Don't block K8s deletion on a failed remote delete (e.g. the
				// pool-scoped route's rejection of deleting a pool's default
				// queue).
				log.Error(err, "Failed to delete work queue from Prefect API", "workQueue", workQueue.Name, "workPool", workQueue.Spec.WorkPoolName)
			} else {
				log.Info("Successfully deleted work queue from Prefect API", "workQueue", workQueue.Name, "workPool", workQueue.Spec.WorkPoolName)
			}
		}
	}

	controllerutil.RemoveFinalizer(workQueue, PrefectWorkQueueFinalizer)
	if err := r.Update(ctx, workQueue); err != nil {
		log.Error(err, "Failed to remove finalizer", "workQueue", workQueue.Name)
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PrefectWorkQueueReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&prefectiov1.PrefectWorkQueue{}).
		Complete(r)
}
