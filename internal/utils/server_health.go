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

package utils

import (
	"context"
	"fmt"
	"net/http"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	prefectiov1 "github.com/PrefectHQ/prefect-operator/api/v1"
)

// CheckPrefectServerHealth verifies that a Prefect server is ready to accept API requests
// This works for both in-cluster servers and external servers (including Prefect Cloud)
func CheckPrefectServerHealth(ctx context.Context, serverRef *prefectiov1.PrefectServerReference, k8sClient client.Client, namespace string) (bool, error) {
	// For in-cluster servers, check Kubernetes deployment readiness first
	if IsInClusterServer(serverRef) {
		ready, err := checkInClusterDeploymentReady(ctx, k8sClient, serverRef, namespace)
		if err != nil {
			return false, fmt.Errorf("failed to check in-cluster deployment: %w", err)
		}
		if !ready {
			return false, nil
		}

		// For in-cluster servers, just checking deployment readiness is sufficient
		// The existing client creation logic handles port-forwarding complexity
		// Trying to do HTTP health checks from outside the cluster is problematic
		return true, nil
	}

	// For external servers, check API health directly
	apiURL := serverRef.GetAPIURL(namespace)
	if apiURL == "" {
		return false, fmt.Errorf("unable to determine API URL for server")
	}

	// Get API key if available for authentication
	headers := make(map[string]string)
	if apiKey, err := serverRef.GetAPIKey(ctx, k8sClient, namespace); err == nil && apiKey != "" {
		headers["Authorization"] = "Bearer " + apiKey
	}

	return checkAPIHealth(ctx, apiURL, headers)
}

// IsInClusterServer determines if the server reference points to an in-cluster server
func IsInClusterServer(serverRef *prefectiov1.PrefectServerReference) bool {
	return serverRef.Name != "" && serverRef.RemoteAPIURL == nil
}

// checkInClusterDeploymentReady verifies that the in-cluster Prefect server deployment is ready
func checkInClusterDeploymentReady(ctx context.Context, k8sClient client.Client, serverRef *prefectiov1.PrefectServerReference, namespace string) (bool, error) {
	serverNamespace := serverRef.Namespace
	if serverNamespace == "" {
		serverNamespace = namespace
	}

	deployment := &appsv1.Deployment{}
	err := k8sClient.Get(ctx, types.NamespacedName{
		Name:      serverRef.Name,
		Namespace: serverNamespace,
	}, deployment)
	if err != nil {
		return false, fmt.Errorf("failed to get deployment %s/%s: %w", serverNamespace, serverRef.Name, err)
	}

	// Check if deployment has ready replicas
	return deployment.Status.ReadyReplicas > 0, nil
}

// checkAPIHealth performs a lightweight health check against the Prefect API
func checkAPIHealth(ctx context.Context, apiURL string, headers map[string]string) (bool, error) {
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	healthURL := apiURL + "/health"
	req, err := http.NewRequestWithContext(ctx, "GET", healthURL, nil)
	if err != nil {
		return false, fmt.Errorf("failed to create health check request: %w", err)
	}

	// Add authentication headers if provided
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := client.Do(req)
	if err != nil {
		// For debugging: return the error so we can see what's happening
		return false, fmt.Errorf("health check failed for %s: %w", healthURL, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Prefect health endpoint should return 200 OK when healthy
	if resp.StatusCode != 200 {
		return false, fmt.Errorf("health check returned status %d for %s", resp.StatusCode, healthURL)
	}
	return true, nil
}
