package v1

import (
	"context"
	"fmt"
	"net/url"
	"path"
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

// APIKeySpec is the API key to use to connect to a remote Prefect Server.
// Set one of Value or ValueFrom.
// +kubebuilder:validation:XValidation:rule="!(has(self.value) && has(self.valueFrom))",message="set apiKey.value or apiKey.valueFrom, not both"
type APIKeySpec struct {
	// Value is the literal value of the API key
	Value *string `json:"value,omitempty"`

	// ValueFrom is a reference to a Secret or ConfigMap key holding the API key,
	// in the resource's namespace. The Secret or ConfigMap must have the
	// annotation prefect.io/allowed-api-urls, a comma-separated list of the
	// Prefect API URLs that the key may be used for. The chart's admission
	// policy also requires that you may get the Secret or ConfigMap.
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
		return "http://" + s.Name + "." + s.ServerNamespace(fallbackNamespace) + ".svc:4200/api"
	}

	return ""
}

// ServerNamespace returns the namespace of the in-cluster server that s
// names: Namespace, or fallbackNamespace when Namespace is empty.
func (s *PrefectServerReference) ServerNamespace(fallbackNamespace string) string {
	if s.Namespace == "" {
		return fallbackNamespace
	}
	return s.Namespace
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

// AnnotationAllowedAPIURLs is the annotation that a Secret or ConfigMap must
// have before the operator reads an API key from it. Its value is a
// comma-separated list of Prefect API URLs. The operator uses the key only for
// a server whose API URL matches one of them: see APIURLAllowed.
const AnnotationAllowedAPIURLs = "prefect.io/allowed-api-urls"

// The operator reads API key Secrets by name only. Its client does not cache
// Secrets, so it needs no list or watch.
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get

// GetAPIKey returns the API key from the configured source. namespace is the
// namespace of the resource that holds s. It returns an error if the Secret or
// ConfigMap, or the key in it, does not exist, or if the object's
// AnnotationAllowedAPIURLs annotation does not allow s's API URL.
func (s *PrefectServerReference) GetAPIKey(ctx context.Context, k8sClient client.Client, namespace string) (string, error) {
	if s.APIKey == nil {
		return "", nil
	}

	// ValueFrom wins over Value, as it does in the worker's PREFECT_API_KEY,
	// so that the operator checks the annotation on the object the worker
	// reads. Objects created before the CRD rejected both can still have both.
	if s.APIKey.ValueFrom == nil {
		if s.APIKey.Value != nil {
			return *s.APIKey.Value, nil
		}
		return "", nil
	}

	apiURL := s.GetAPIURL(namespace)

	if ref := s.APIKey.ValueFrom.SecretKeyRef; ref != nil {
		secret := &corev1.Secret{}
		if err := k8sClient.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: namespace}, secret); err != nil {
			return "", fmt.Errorf("failed to get secret %s: %w", ref.Name, err)
		}
		if err := checkAllowedAPIURL("secret", ref.Name, secret.Annotations, apiURL); err != nil {
			return "", err
		}
		value, exists := secret.Data[ref.Key]
		if !exists {
			return "", fmt.Errorf("key %s not found in secret %s", ref.Key, ref.Name)
		}
		return string(value), nil
	}

	if ref := s.APIKey.ValueFrom.ConfigMapKeyRef; ref != nil {
		configMap := &corev1.ConfigMap{}
		if err := k8sClient.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: namespace}, configMap); err != nil {
			return "", fmt.Errorf("failed to get configmap %s: %w", ref.Name, err)
		}
		if err := checkAllowedAPIURL("configmap", ref.Name, configMap.Annotations, apiURL); err != nil {
			return "", err
		}
		value, exists := configMap.Data[ref.Key]
		if !exists {
			return "", fmt.Errorf("key %s not found in configmap %s", ref.Key, ref.Name)
		}
		return value, nil
	}

	return "", nil
}

// checkAllowedAPIURL returns an error unless the AnnotationAllowedAPIURLs
// annotation in annotations allows apiURL. kind and name name the object that
// has the annotations, for the error message.
func checkAllowedAPIURL(kind, name string, annotations map[string]string, apiURL string) error {
	allowed, ok := annotations[AnnotationAllowedAPIURLs]
	if !ok {
		return fmt.Errorf("%s %s does not have the annotation %s, which must list the Prefect API URLs that its API key may be used for",
			kind, name, AnnotationAllowedAPIURLs)
	}
	if !APIURLAllowed(allowed, apiURL) {
		return fmt.Errorf("%s %s does not allow its API key to be used for %q: its annotation %s is %q",
			kind, name, apiURL, AnnotationAllowedAPIURLs, allowed)
	}
	return nil
}

// APIURLAllowed reports whether apiURL matches an entry in allowed, a
// comma-separated list of URLs. An entry matches when its scheme and host,
// including any port, equal apiURL's, and its path is apiURL's path or a parent
// of it. Paths are compared after resolving "." and ".." segments, so
// "https://api.prefect.cloud/api/accounts/a" matches
// "https://api.prefect.cloud/api/accounts/a/workspaces/w" but not
// "https://api.prefect.cloud/api/accounts/ab" or
// "https://api.prefect.cloud/api/accounts/a/../b". An entry with user
// information, a query or a fragment matches nothing.
func APIURLAllowed(allowed, apiURL string) bool {
	target, ok := parseAPIURL(apiURL)
	if !ok {
		return false
	}
	for entry := range strings.SplitSeq(allowed, ",") {
		want, ok := parseAPIURL(strings.TrimSpace(entry))
		if !ok || want.Scheme != target.Scheme || want.Host != target.Host {
			continue
		}
		if want.Path == "/" || want.Path == target.Path || strings.HasPrefix(target.Path, want.Path+"/") {
			return true
		}
	}
	return false
}

// parseAPIURL parses an absolute http or https URL, with its scheme and host in
// lower case and its path cleaned. It returns false for any other URL, or one
// with user information, a query or a fragment.
func parseAPIURL(raw string) (*url.URL, bool) {
	u, err := url.Parse(raw)
	if err != nil || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" {
		return nil, false
	}
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)
	if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, false
	}
	u.Path = path.Clean("/" + u.Path)
	return u, true
}
