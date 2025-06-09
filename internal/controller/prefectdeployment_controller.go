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

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	prefectiov1 "github.com/PrefectHQ/prefect-operator/api/v1"
	"github.com/PrefectHQ/prefect-operator/internal/prefect"
	"github.com/PrefectHQ/prefect-operator/internal/utils"
	"github.com/go-logr/logr"
)

const (
	// PrefectDeploymentConditionReady indicates the deployment is ready
	PrefectDeploymentConditionReady = "Ready"

	// PrefectDeploymentConditionSynced indicates the deployment is synced with Prefect API
	PrefectDeploymentConditionSynced = "Synced"

	// RequeueIntervalReady is the interval for requeuing when deployment is ready
	RequeueIntervalReady = 5 * time.Minute

	// RequeueIntervalError is the interval for requeuing on errors
	RequeueIntervalError = 30 * time.Second

	// RequeueIntervalSync is the interval for requeuing during sync operations
	RequeueIntervalSync = 10 * time.Second
)

// PrefectDeploymentReconciler reconciles a PrefectDeployment object
type PrefectDeploymentReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	PrefectClient prefect.PrefectClient
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

	// Calculate the spec hash
	specHash, err := r.calculateSpecHash(&deployment)
	if err != nil {
		log.Error(err, "Failed to calculate spec hash", "deployment", deployment.Name)
		return ctrl.Result{}, err
	}

	// Check if we need to sync with the Prefect API
	if r.needsSync(&deployment, specHash) {
		log.Info("Starting sync with Prefect API", "deployment", deployment.Name)
		result, err := r.syncWithPrefect(ctx, &deployment)
		if err != nil {
			return result, err
		}
		return result, nil
	}

	return ctrl.Result{RequeueAfter: RequeueIntervalReady}, nil
}

// needsSync determines if the deployment needs to be synced with Prefect API
func (r *PrefectDeploymentReconciler) needsSync(deployment *prefectiov1.PrefectDeployment, currentSpecHash string) bool {
	// Always sync if deployment doesn't exist in Prefect yet
	if deployment.Status.Id == nil || *deployment.Status.Id == "" {
		return true
	}

	// Sync if spec has changed
	if deployment.Status.SpecHash != currentSpecHash {
		return true
	}

	// Sync if observed generation is behind
	if deployment.Status.ObservedGeneration < deployment.Generation {
		return true
	}

	// Sync if last sync was too long ago (drift detection)
	if deployment.Status.LastSyncTime == nil {
		return true
	}

	timeSinceLastSync := time.Since(deployment.Status.LastSyncTime.Time)
	return timeSinceLastSync > 10*time.Minute
}

// syncWithPrefect syncs the deployment with the Prefect API
func (r *PrefectDeploymentReconciler) syncWithPrefect(ctx context.Context, deployment *prefectiov1.PrefectDeployment) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Use injected client if available (for testing), otherwise create a new one
	prefectClient := r.PrefectClient
	if prefectClient == nil {
		var err error
		prefectClient, err = r.createPrefectClient(ctx, deployment, log)
		if err != nil {
			log.Error(err, "Failed to create Prefect client", "deployment", deployment.Name)
			return ctrl.Result{}, err
		}
	}

	// Get flow ID for the deployment
	flowID, err := prefect.GetFlowIDFromDeployment(ctx, prefectClient, deployment)
	if err != nil {
		log.Error(err, "Failed to get flow ID", "deployment", deployment.Name)
		r.setCondition(deployment, PrefectDeploymentConditionSynced, metav1.ConditionFalse, "FlowIDError", err.Error())
		return ctrl.Result{}, err
	}

	// Convert to Prefect deployment spec
	deploymentSpec, err := prefect.ConvertToDeploymentSpec(deployment, flowID)
	if err != nil {
		log.Error(err, "Failed to convert deployment spec", "deployment", deployment.Name)
		r.setCondition(deployment, PrefectDeploymentConditionSynced, metav1.ConditionFalse, "ConversionError", err.Error())
		return ctrl.Result{}, err
	}

	// Create or update deployment in Prefect
	prefectDeployment, err := prefectClient.CreateOrUpdateDeployment(ctx, deploymentSpec)
	if err != nil {
		log.Error(err, "Failed to create or update deployment in Prefect", "deployment", deployment.Name)
		r.setCondition(deployment, PrefectDeploymentConditionSynced, metav1.ConditionFalse, "SyncError", err.Error())
		return ctrl.Result{}, err
	}

	// Update deployment status
	prefect.UpdateDeploymentStatus(deployment, prefectDeployment)

	// Update spec hash
	specHash, err := r.calculateSpecHash(deployment)
	if err != nil {
		log.Error(err, "Failed to calculate spec hash", "deployment", deployment.Name)
		return ctrl.Result{}, err
	}
	deployment.Status.SpecHash = specHash
	deployment.Status.ObservedGeneration = deployment.Generation

	// Set success conditions
	r.setCondition(deployment, PrefectDeploymentConditionSynced, metav1.ConditionTrue, "SyncSuccessful", "Deployment successfully synced with Prefect API")
	r.setCondition(deployment, PrefectDeploymentConditionReady, metav1.ConditionTrue, "DeploymentReady", "Deployment is ready and operational")

	// Update deployment status
	if err := r.Status().Update(ctx, deployment); err != nil {
		log.Error(err, "Failed to update deployment status", "deployment", deployment.Name)
		return ctrl.Result{}, err
	}

	log.Info("Successfully synced deployment with Prefect", "deploymentId", prefectDeployment.ID)
	return ctrl.Result{RequeueAfter: RequeueIntervalReady}, nil
}

// createPrefectClient creates a new Prefect client
func (r *PrefectDeploymentReconciler) createPrefectClient(ctx context.Context, deployment *prefectiov1.PrefectDeployment, log logr.Logger) (prefect.PrefectClient, error) {
	// Get API key
	apiKey, err := r.getAPIKey(ctx, deployment.Spec.Server.APIKey, deployment.Namespace)
	if err != nil {
		return nil, err
	}

	// Create client
	return prefect.NewClientFromServerReference(&deployment.Spec.Server, apiKey, log)
}

// getAPIKey retrieves the API key from the configured source
func (r *PrefectDeploymentReconciler) getAPIKey(ctx context.Context, apiKeySpec *prefectiov1.APIKeySpec, namespace string) (string, error) {
	if apiKeySpec == nil {
		return "", nil // No API key configured
	}

	// Direct value takes precedence
	if apiKeySpec.Value != nil {
		return *apiKeySpec.Value, nil
	}

	// Get from environment variable source
	if apiKeySpec.ValueFrom != nil {
		if apiKeySpec.ValueFrom.SecretKeyRef != nil {
			secretRef := apiKeySpec.ValueFrom.SecretKeyRef
			secret := &corev1.Secret{}
			secretKey := types.NamespacedName{
				Name:      secretRef.Name,
				Namespace: namespace,
			}

			if err := r.Get(ctx, secretKey, secret); err != nil {
				return "", fmt.Errorf("failed to get secret %s: %w", secretRef.Name, err)
			}

			value, exists := secret.Data[secretRef.Key]
			if !exists {
				return "", fmt.Errorf("key %s not found in secret %s", secretRef.Key, secretRef.Name)
			}

			return string(value), nil
		}

		if apiKeySpec.ValueFrom.ConfigMapKeyRef != nil {
			configMapRef := apiKeySpec.ValueFrom.ConfigMapKeyRef
			configMap := &corev1.ConfigMap{}
			configMapKey := types.NamespacedName{
				Name:      configMapRef.Name,
				Namespace: namespace,
			}

			if err := r.Get(ctx, configMapKey, configMap); err != nil {
				return "", fmt.Errorf("failed to get configmap %s: %w", configMapRef.Name, err)
			}

			value, exists := configMap.Data[configMapRef.Key]
			if !exists {
				return "", fmt.Errorf("key %s not found in configmap %s", configMapRef.Key, configMapRef.Name)
			}

			return value, nil
		}
	}

	return "", nil // No API key configured
}

// calculateSpecHash calculates a hash of the deployment spec for change detection
func (r *PrefectDeploymentReconciler) calculateSpecHash(deployment *prefectiov1.PrefectDeployment) (string, error) {
	return utils.Hash(deployment.Spec, 16)
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

// SetupWithManager sets up the controller with the Manager.
func (r *PrefectDeploymentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&prefectiov1.PrefectDeployment{}).
		Complete(r)
}
