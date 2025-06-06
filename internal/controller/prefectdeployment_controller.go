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
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

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

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *PrefectDeploymentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)

	log.Info("Reconciling PrefectDeployment")

	deployment := &prefectiov1.PrefectDeployment{}
	err := r.Get(ctx, req.NamespacedName, deployment)
	if errors.IsNotFound(err) {
		return ctrl.Result{}, nil
	} else if err != nil {
		return ctrl.Result{}, err
	}

	// Defer a final status update at the end of the reconciliation loop
	defer func() {
		if statusErr := r.Status().Update(ctx, deployment); statusErr != nil {
			log.Error(statusErr, "Failed to update PrefectDeployment status")
		}
	}()

	// Check if we need to sync with Prefect API based on spec changes
	currentSpecHash, err := r.calculateSpecHash(deployment)
	if err != nil {
		log.Error(err, "Failed to calculate spec hash")
		r.setCondition(deployment, PrefectDeploymentConditionReady, metav1.ConditionFalse, "HashError", err.Error())
		return ctrl.Result{RequeueAfter: RequeueIntervalError}, nil
	}

	// Determine if we need to make API calls to Prefect
	needsSync := r.needsSync(deployment, currentSpecHash)

	if needsSync {
		log.Info("Deployment needs sync with Prefect API", "reason", r.getSyncReason(deployment, currentSpecHash))
		return r.syncWithPrefect(ctx, deployment, currentSpecHash, log), nil
	}

	// No sync needed, update status if deployment is ready
	if deployment.Status.Id != nil && *deployment.Status.Id != "" {
		r.setCondition(deployment, PrefectDeploymentConditionReady, metav1.ConditionTrue, "DeploymentReady", "Deployment is ready and synced")
		r.setCondition(deployment, PrefectDeploymentConditionSynced, metav1.ConditionTrue, "SyncComplete", "Deployment is synced with Prefect")
		deployment.Status.Ready = true
		deployment.Status.ObservedGeneration = deployment.Generation
	}

	// Requeue after a longer interval for ready deployments to periodically check for drift
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

// getSyncReason returns a human-readable reason for why sync is needed
func (r *PrefectDeploymentReconciler) getSyncReason(deployment *prefectiov1.PrefectDeployment, currentSpecHash string) string {
	if deployment.Status.Id == nil || *deployment.Status.Id == "" {
		return "deployment does not exist in Prefect"
	}
	if deployment.Status.SpecHash != currentSpecHash {
		return "spec has changed"
	}
	if deployment.Status.ObservedGeneration < deployment.Generation {
		return "generation has changed"
	}
	if deployment.Status.LastSyncTime == nil {
		return "never synced"
	}
	timeSinceLastSync := time.Since(deployment.Status.LastSyncTime.Time)
	if timeSinceLastSync > 10*time.Minute {
		return fmt.Sprintf("last sync was %v ago", timeSinceLastSync)
	}
	return "unknown"
}

// syncWithPrefect handles synchronization with the Prefect API
func (r *PrefectDeploymentReconciler) syncWithPrefect(ctx context.Context, deployment *prefectiov1.PrefectDeployment, specHash string, log logr.Logger) ctrl.Result {
	log.Info("Starting sync with Prefect API")

	// Set syncing condition
	r.setCondition(deployment, PrefectDeploymentConditionSynced, metav1.ConditionFalse, "Syncing", "Syncing deployment with Prefect API")
	deployment.Status.Ready = false

	// Create Prefect client if not provided (for testing)
	prefectClient := r.PrefectClient
	if prefectClient == nil {
		var err error
		prefectClient, err = r.createPrefectClient(ctx, deployment, log)
		if err != nil {
			r.setCondition(deployment, PrefectDeploymentConditionSynced, metav1.ConditionFalse, "ClientError", err.Error())
			return ctrl.Result{RequeueAfter: RequeueIntervalError}
		}
	}

	// Get flow ID for the deployment
	flowID, err := prefect.GetFlowIDFromDeployment(ctx, prefectClient, deployment)
	if err != nil {
		r.setCondition(deployment, PrefectDeploymentConditionSynced, metav1.ConditionFalse, "FlowIDError", err.Error())
		return ctrl.Result{RequeueAfter: RequeueIntervalError}
	}

	// Convert K8s deployment to Prefect API format
	deploymentSpec, err := prefect.ConvertToDeploymentSpec(deployment, flowID)
	if err != nil {
		r.setCondition(deployment, PrefectDeploymentConditionSynced, metav1.ConditionFalse, "ConversionError", err.Error())
		return ctrl.Result{RequeueAfter: RequeueIntervalError}
	}

	// Create or update deployment in Prefect
	prefectDeployment, err := prefectClient.CreateOrUpdateDeployment(ctx, deploymentSpec)
	if err != nil {
		r.setCondition(deployment, PrefectDeploymentConditionSynced, metav1.ConditionFalse, "APIError", err.Error())
		return ctrl.Result{RequeueAfter: RequeueIntervalError}
	}

	// Update K8s deployment status
	prefect.UpdateDeploymentStatus(deployment, prefectDeployment)
	deployment.Status.SpecHash = specHash
	deployment.Status.ObservedGeneration = deployment.Generation
	now := metav1.Now()
	deployment.Status.LastSyncTime = &now

	r.setCondition(deployment, PrefectDeploymentConditionSynced, metav1.ConditionTrue, "SyncComplete", "Successfully synced with Prefect API")
	r.setCondition(deployment, PrefectDeploymentConditionReady, metav1.ConditionTrue, "DeploymentReady", "Deployment is ready")
	deployment.Status.Ready = true

	log.Info("Successfully synced deployment with Prefect", "deploymentId", prefectDeployment.ID)
	return ctrl.Result{RequeueAfter: RequeueIntervalReady}
}

// createPrefectClient creates a Prefect API client based on the deployment's server configuration
func (r *PrefectDeploymentReconciler) createPrefectClient(ctx context.Context, deployment *prefectiov1.PrefectDeployment, log logr.Logger) (prefect.PrefectClient, error) {
	serverRef := deployment.Spec.Server

	// Get API URL
	apiURL := serverRef.GetAPIURL(deployment.Namespace)
	if apiURL == "" {
		return nil, fmt.Errorf("no API URL configured for Prefect server")
	}

	// Get API key if configured
	var apiKey string
	if serverRef.APIKey != nil {
		var err error
		apiKey, err = r.getAPIKey(ctx, serverRef.APIKey, deployment.Namespace)
		if err != nil {
			return nil, fmt.Errorf("failed to get API key: %w", err)
		}
	}

	log.Info("Creating Prefect client", "apiURL", apiURL, "hasAPIKey", apiKey != "")
	return prefect.NewClient(apiURL, apiKey, log), nil
}

// getAPIKey retrieves the API key from the configured source
func (r *PrefectDeploymentReconciler) getAPIKey(ctx context.Context, apiKeySpec *prefectiov1.APIKeySpec, namespace string) (string, error) {
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
