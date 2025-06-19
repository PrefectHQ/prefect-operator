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

package v1

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// PrefectServerReference defines how to connect to a Prefect server
type PrefectServerReference struct {
	// Namespace is the namespace where the in-cluster Prefect Server is running
	Namespace string `json:"namespace,omitempty"`

	// Name is the name of the in-cluster Prefect Server in the given namespace
	Name string `json:"name,omitempty"`

	// RemoteAPIURL is the API URL for the remote Prefect Server. Set if using with an external Prefect Server or Prefect Cloud
	RemoteAPIURL *string `json:"remoteApiUrl,omitempty"`

	// APIKey is the API key to use to connect to a remote Prefect Server
	APIKey *APIKeySpec `json:"apiKey,omitempty"`

	// AccountID is the ID of the account to use to connect to Prefect Cloud
	AccountID *string `json:"accountId,omitempty"`

	// WorkspaceID is the ID of the workspace to use to connect to Prefect Cloud
	WorkspaceID *string `json:"workspaceId,omitempty"`
}

// APIKeySpec is the API key to use to connect to a remote Prefect Server
type APIKeySpec struct {
	// Value is the literal value of the API key
	Value *string `json:"value,omitempty"`

	// ValueFrom is a reference to a secret containing the API key
	ValueFrom *corev1.EnvVarSource `json:"valueFrom,omitempty"`
}

// GetAPIURL returns the API URL for this server reference with optional namespace fallback
func (s *PrefectServerReference) GetAPIURL(fallbackNamespace string) string {
	if s.RemoteAPIURL != nil && *s.RemoteAPIURL != "" {
		remote := *s.RemoteAPIURL
		if !strings.HasSuffix(remote, "/api") {
			remote = fmt.Sprintf("%s/api", remote)
		}

		if s.AccountID != nil && s.WorkspaceID != nil {
			remote = fmt.Sprintf("%s/accounts/%s/workspaces/%s", remote, *s.AccountID, *s.WorkspaceID)
		}
		return remote
	}

	// For in-cluster servers, construct the service URL
	if s.Name != "" {
		serverNamespace := s.Namespace
		if serverNamespace == "" {
			serverNamespace = fallbackNamespace
		}
		return "http://" + s.Name + "." + serverNamespace + ".svc:4200/api"
	}

	return ""
}

// IsRemote returns true if this references a remote Prefect server
func (s *PrefectServerReference) IsRemote() bool {
	return s.RemoteAPIURL != nil && *s.RemoteAPIURL != ""
}

// IsInCluster returns true if this references an in-cluster Prefect server
func (s *PrefectServerReference) IsInCluster() bool {
	return s.Name != ""
}

// IsPrefectCloud returns true if this is configured for Prefect Cloud
func (s *PrefectServerReference) IsPrefectCloud() bool {
	return s.AccountID != nil && *s.AccountID != "" && s.WorkspaceID != nil && *s.WorkspaceID != ""
}

// GetAPIKey retrieves the API key from the configured source
func (s *PrefectServerReference) GetAPIKey(ctx context.Context, k8sClient client.Client, namespace string) (string, error) {
	if s.APIKey == nil {
		return "", nil
	}

	if s.APIKey.Value != nil {
		return *s.APIKey.Value, nil
	}

	if s.APIKey.ValueFrom != nil {
		if s.APIKey.ValueFrom.SecretKeyRef != nil {
			secretRef := s.APIKey.ValueFrom.SecretKeyRef
			secret := &corev1.Secret{}
			secretKey := types.NamespacedName{
				Name:      secretRef.Name,
				Namespace: namespace,
			}

			if err := k8sClient.Get(ctx, secretKey, secret); err != nil {
				return "", fmt.Errorf("failed to get secret %s: %w", secretRef.Name, err)
			}

			value, exists := secret.Data[secretRef.Key]
			if !exists {
				return "", fmt.Errorf("key %s not found in secret %s", secretRef.Key, secretRef.Name)
			}

			return string(value), nil
		}

		if s.APIKey.ValueFrom.ConfigMapKeyRef != nil {
			configMapRef := s.APIKey.ValueFrom.ConfigMapKeyRef
			configMap := &corev1.ConfigMap{}
			configMapKey := types.NamespacedName{
				Name:      configMapRef.Name,
				Namespace: namespace,
			}

			if err := k8sClient.Get(ctx, configMapKey, configMap); err != nil {
				return "", fmt.Errorf("failed to get configmap %s: %w", configMapRef.Name, err)
			}

			value, exists := configMap.Data[configMapRef.Key]
			if !exists {
				return "", fmt.Errorf("key %s not found in configmap %s", configMapRef.Key, configMapRef.Name)
			}

			return value, nil
		}
	}

	return "", nil
}
