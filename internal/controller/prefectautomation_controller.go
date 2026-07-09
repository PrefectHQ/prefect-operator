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
	Scheme        *runtime.Scheme
	PrefectClient prefect.PrefectClient
	// DefaultResyncInterval is the fallback drift-detection interval used when a
	// PrefectAutomation does not set spec.interval.
	DefaultResyncInterval time.Duration
}

// resyncInterval returns the effective drift-detection interval for the
// automation: its spec.interval when set, otherwise the operator default.
func (r *PrefectAutomationReconciler) resyncInterval(automation *prefectiov1.PrefectAutomation) time.Duration {
	return utils.ResyncInterval(automation.Spec.Interval, r.DefaultResyncInterval)
}

//+kubebuilder:rbac:groups=prefect.io,resources=prefectautomations,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=prefect.io,resources=prefectautomations/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=prefect.io,resources=prefectautomations/finalizers,verbs=update

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
		r.setCondition(&automation, PrefectAutomationConditionSynced, metav1.ConditionFalse, "InvalidSpec", err.Error())
		automation.Status.Ready = false
		if updateErr := r.Status().Update(ctx, &automation); updateErr != nil {
			log.Error(updateErr, "Failed to update automation status", "automation", automation.Name)
		}
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

	return ctrl.Result{RequeueAfter: utils.JitterResyncInterval(r.resyncInterval(&automation))}, nil
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
	return time.Since(automation.Status.LastSyncTime.Time) > r.resyncInterval(automation)
}

// syncWithPrefect creates or updates the automation in the Prefect API
func (r *PrefectAutomationReconciler) syncWithPrefect(ctx context.Context, automation *prefectiov1.PrefectAutomation) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	prefectClient := r.PrefectClient
	if prefectClient == nil {
		var err error
		prefectClient, err = prefect.NewClientFromK8s(ctx, &automation.Spec.Server, r.Client, automation.Namespace, log)
		if err != nil {
			log.Error(err, "Failed to create Prefect client", "automation", automation.Name)
			return ctrl.Result{}, err
		}
	}

	// Resolve any deploymentName references to deployment IDs. If a referenced
	// deployment doesn't exist yet (e.g. its PrefectDeployment hasn't reconciled),
	// requeue until it does.
	deploymentIDs := map[string]string{}
	for _, name := range prefect.DeploymentNamesReferenced(automation) {
		dep, derr := prefectClient.FindDeploymentByName(ctx, name)
		if derr != nil {
			log.V(1).Info("Referenced deployment not found, requeuing", "automation", automation.Name, "deployment", name)
			r.setCondition(automation, PrefectAutomationConditionSynced, metav1.ConditionFalse, "DeploymentNotFound", fmt.Sprintf("referenced deployment %q not found yet", name))
			automation.Status.Ready = false
			if updateErr := r.Status().Update(ctx, automation); updateErr != nil {
				log.Error(updateErr, "Failed to update automation status", "automation", automation.Name)
			}
			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
		}
		deploymentIDs[name] = dep.ID
	}

	automationSpec, err := prefect.ConvertToAutomationSpec(automation, deploymentIDs)
	if err != nil {
		log.Error(err, "Failed to convert automation spec", "automation", automation.Name)
		r.setCondition(automation, PrefectAutomationConditionSynced, metav1.ConditionFalse, "ConversionError", err.Error())
		if updateErr := r.Status().Update(ctx, automation); updateErr != nil {
			log.Error(updateErr, "Failed to update automation status", "automation", automation.Name)
		}
		return ctrl.Result{}, nil
	}

	var result *prefect.Automation
	if automation.Status.Id != nil && *automation.Status.Id != "" {
		result, err = prefectClient.UpdateAutomation(ctx, *automation.Status.Id, automationSpec)
		// If the automation was deleted out-of-band, clear the stale ID and
		// requeue so the next pass recreates it instead of looping on SyncError.
		if errors.Is(err, prefect.ErrAutomationNotFound) {
			log.Info("Automation no longer exists in Prefect, recreating", "automation", automation.Name, "prefectId", *automation.Status.Id)
			automation.Status.Id = nil
			r.setCondition(automation, PrefectAutomationConditionSynced, metav1.ConditionFalse, "Recreating", "Automation was deleted in Prefect; recreating")
			automation.Status.Ready = false
			if updateErr := r.Status().Update(ctx, automation); updateErr != nil {
				log.Error(updateErr, "Failed to update automation status", "automation", automation.Name)
			}
			return ctrl.Result{RequeueAfter: time.Second}, nil
		}
	} else {
		result, err = prefectClient.CreateAutomation(ctx, automationSpec)
	}
	if err != nil {
		log.Error(err, "Failed to sync automation with Prefect", "automation", automation.Name)
		r.setCondition(automation, PrefectAutomationConditionSynced, metav1.ConditionFalse, "SyncError", err.Error())
		if updateErr := r.Status().Update(ctx, automation); updateErr != nil {
			log.Error(updateErr, "Failed to update automation status", "automation", automation.Name)
		}
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

func (r *PrefectAutomationReconciler) setCondition(automation *prefectiov1.PrefectAutomation, conditionType string, status metav1.ConditionStatus, reason, message string) {
	meta.SetStatusCondition(&automation.Status.Conditions, metav1.Condition{
		Type:               conditionType,
		Status:             status,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	})
}

// handleDeletion cleans up the automation in Prefect and removes the finalizer
func (r *PrefectAutomationReconciler) handleDeletion(ctx context.Context, automation *prefectiov1.PrefectAutomation) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("Handling deletion of PrefectAutomation", "automation", automation.Name)

	if !controllerutil.ContainsFinalizer(automation, PrefectAutomationFinalizer) {
		return ctrl.Result{}, nil
	}

	if automation.Status.Id != nil && *automation.Status.Id != "" {
		prefectClient := r.PrefectClient
		if prefectClient == nil {
			var err error
			prefectClient, err = prefect.NewClientFromK8s(ctx, &automation.Spec.Server, r.Client, automation.Namespace, log)
			if err != nil {
				log.Error(err, "Failed to create Prefect client for deletion", "automation", automation.Name)
				prefectClient = nil
			}
		}
		if prefectClient != nil {
			if err := prefectClient.DeleteAutomation(ctx, *automation.Status.Id); err != nil {
				// Don't block K8s deletion on a failed remote delete.
				log.Error(err, "Failed to delete automation from Prefect API", "automation", automation.Name, "prefectId", *automation.Status.Id)
			} else {
				log.Info("Successfully deleted automation from Prefect API", "automation", automation.Name, "prefectId", *automation.Status.Id)
			}
		}
	}

	controllerutil.RemoveFinalizer(automation, PrefectAutomationFinalizer)
	if err := r.Update(ctx, automation); err != nil {
		log.Error(err, "Failed to remove finalizer", "automation", automation.Name)
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PrefectAutomationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&prefectiov1.PrefectAutomation{}).
		Complete(r)
}
