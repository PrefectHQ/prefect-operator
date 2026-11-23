package v1

import (
	"context"
	"fmt"
	"net/url"
	"path"
	"strings"
	"unicode"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// PrefectServerReference defines how to connect to a Prefect server
// +kubebuilder:validation:XValidation:rule="!(has(self.apiKey) && has(self.auth))",message="set apiKey or auth, not both"
type PrefectServerReference struct {
	// Namespace is the namespace where the in-cluster Prefect Server is running.
	// It must be a DNS label: lower case letters, digits and hyphens.
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
	Namespace string `json:"namespace,omitempty"`

	// Name is the name of the in-cluster Prefect Server in the given namespace.
	// It must be a DNS label: lower case letters, digits and hyphens.
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
	Name string `json:"name,omitempty"`

	// RemoteAPIURL is the API URL for the remote Prefect Server. Set if using with an external Prefect Server or Prefect Cloud.
	// It must be an http or https URL with a host, and without user information, a query or a fragment.
	// +kubebuilder:validation:MaxLength=2048
	// +kubebuilder:validation:XValidation:rule="isURL(self) && url(self).getScheme() in ['http', 'https'] && url(self).getHostname() != '' && !self.contains('@') && !self.contains('?') && !self.contains('#')",message="remoteApiUrl must be an http or https URL with a host, and without user information, a query or a fragment"
	RemoteAPIURL *string `json:"remoteApiUrl,omitempty"`

	// APIKey is the API key to use to connect to a remote Prefect Server
	APIKey *APIKeySpec `json:"apiKey,omitempty"`

	// Auth is the auth string to send to a Prefect server that requires one,
	// such as an in-cluster server with spec.auth. The operator sends it as HTTP
	// basic authentication, and only to an API URL that the Secret's
	// prefect.io/allowed-api-urls annotation lists, as for apiKey.valueFrom.
	// A work pool's worker gets it as PREFECT_API_AUTH_STRING.
	Auth *ServerAuth `json:"auth,omitempty"`

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

// ServerAuth names the Secret key that holds a Prefect server's auth string,
// in the form "username:password".
// +kubebuilder:validation:XValidation:rule="size(self.secretKeyRef.name) > 0",message="secretKeyRef.name must be set"
// +kubebuilder:validation:XValidation:rule="!has(self.secretKeyRef.optional) || !self.secretKeyRef.optional",message="secretKeyRef.optional must not be true"
type ServerAuth struct {
	// SecretKeyRef selects a key of a Secret in the resource's namespace. It
	// must not be optional: a pod that uses it does not start without the key.
	SecretKeyRef corev1.SecretKeySelector `json:"secretKeyRef"`
}

// envVarSource returns the source of an environment variable that holds the
// auth string. It is never optional, because a server whose auth string is
// missing accepts every request.
func (a *ServerAuth) envVarSource() *corev1.EnvVarSource {
	ref := a.SecretKeyRef
	ref.Optional = nil
	return &corev1.EnvVarSource{SecretKeyRef: &ref}
}

// DefaultClusterDomain is the DNS domain of a cluster that does not set another.
const DefaultClusterDomain = "cluster.local"

// ClusterDomain is the DNS domain of the cluster, without a trailing dot.
// GetAPIURL puts it at the end of an in-cluster server's host. main sets it
// before the manager starts, and nothing may change it after that.
var ClusterDomain = DefaultClusterDomain

// inClusterHostSuffix returns the end of the host in an in-cluster server's
// URL, after the Service's name and namespace. Its trailing dot makes the host
// absolute. A resolver adds each of its search domains to any other host with
// fewer dots than its ndots option, which is 5 in a pod, before it looks up the
// host itself. The first name that exists may not be the Service's.
func inClusterHostSuffix() string {
	return ".svc." + ClusterDomain + "."
}

// IsInClusterHost reports whether host, without a port, is the host that
// GetAPIURL builds for an in-cluster server.
func IsInClusterHost(host string) bool {
	return strings.HasSuffix(strings.ToLower(host), inClusterHostSuffix())
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
		return "http://" + s.Name + "." + s.ServerNamespace(fallbackNamespace) + inClusterHostSuffix() + ":4200/api"
	}

	return ""
}

// ValidateAPIURL returns an error if GetAPIURL would not build a URL the
// operator may send requests to:
//
//   - s is remote, and its API URL is not an http or https URL with a host, or
//     has user information, a query or a fragment.
//   - s is in-cluster, and Name or Namespace is not a DNS label.
//
// The CRD rejects such references, but resources written before those rules
// existed can still have one. GetAPIURL puts Name and Namespace into the URL's
// host, so a Name such as "example.com/x?" would send requests to example.com.
// In a query, the paths that the client appends would not reach the server as
// paths.
func (s *PrefectServerReference) ValidateAPIURL() error {
	if s.IsRemote() {
		if _, ok := parseAPIURL(s.GetAPIURL("")); !ok {
			return fmt.Errorf("the API URL %q is not an http or https URL with a host, "+
				"or has user information, a query or a fragment", s.GetAPIURL(""))
		}
		return nil
	}
	if !s.IsInCluster() {
		return nil
	}
	if errs := validation.IsDNS1123Label(s.Name); len(errs) > 0 {
		return fmt.Errorf("spec.server.name %q is not a DNS label: %s", s.Name, strings.Join(errs, "; "))
	}
	if s.Namespace != "" {
		if errs := validation.IsDNS1123Label(s.Namespace); len(errs) > 0 {
			return fmt.Errorf("spec.server.namespace %q is not a DNS label: %s", s.Namespace, strings.Join(errs, "; "))
		}
	}
	return nil
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

// ServerReference returns the work pool's spec.server.
func (p *PrefectWorkPool) ServerReference() *PrefectServerReference { return &p.Spec.Server }

// ServerReference returns the work queue's spec.server.
func (q *PrefectWorkQueue) ServerReference() *PrefectServerReference { return &q.Spec.Server }

// ServerReference returns the deployment's spec.server.
func (d *PrefectDeployment) ServerReference() *PrefectServerReference { return &d.Spec.Server }

// ServerReference returns the automation's spec.server.
func (a *PrefectAutomation) ServerReference() *PrefectServerReference { return &a.Spec.Server }

// AnnotationAllowedAPIURLs is the annotation that a Secret or ConfigMap must
// have before the operator reads an API key or auth string from it. Its value
// is a comma-separated list of Prefect API URLs. The operator uses the value
// only for a server whose API URL matches one of them: see APIURLAllowed.
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
		return readSecretKey(ctx, k8sClient, namespace, ref, apiURL, "API key")
	}

	if ref := s.APIKey.ValueFrom.ConfigMapKeyRef; ref != nil {
		configMap := &corev1.ConfigMap{}
		if err := k8sClient.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: namespace}, configMap); err != nil {
			return "", fmt.Errorf("failed to get configmap %s: %w", ref.Name, err)
		}
		if err := checkAllowedAPIURL("configmap", ref.Name, configMap.Annotations, apiURL, "API key"); err != nil {
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

// GetAuthString returns the auth string from the Secret that Auth names, or ""
// if Auth is nil. namespace is the namespace of the resource that holds s. It
// returns an error if the Secret, or the key in it, does not exist, or if the
// Secret's AnnotationAllowedAPIURLs annotation does not allow s's API URL. It
// also returns an error if the auth string is empty, because the client sends
// no authentication without one.
func (s *PrefectServerReference) GetAuthString(ctx context.Context, k8sClient client.Client, namespace string) (string, error) {
	if s.Auth == nil {
		return "", nil
	}
	ref := &s.Auth.SecretKeyRef
	authString, err := readSecretKey(ctx, k8sClient, namespace, ref, s.GetAPIURL(namespace), "auth string")
	if err != nil {
		return "", err
	}
	if authString == "" {
		return "", fmt.Errorf("key %s in secret %s is empty", ref.Key, ref.Name)
	}
	return authString, nil
}

// Validate returns an error unless the Secret key that a names, in namespace,
// holds an auth string with a username and a password. The auth string must not
// start or end with whitespace, which the server would make part of the username
// or password. It must be ASCII, because a Prefect server with any other auth
// string returns a 500 for every request. Validate does not check the Secret's
// AnnotationAllowedAPIURLs annotation, which limits where a client sends an auth
// string. A server does not send its own.
func (a *ServerAuth) Validate(ctx context.Context, k8sClient client.Client, namespace string) error {
	ref := &a.SecretKeyRef
	secret := &corev1.Secret{}
	if err := k8sClient.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: namespace}, secret); err != nil {
		return fmt.Errorf("failed to get secret %s: %w", ref.Name, err)
	}
	value, exists := secret.Data[ref.Key]
	if !exists {
		return fmt.Errorf("key %s not found in secret %s", ref.Key, ref.Name)
	}
	authString := strings.TrimSpace(string(value))
	username, password, _ := strings.Cut(authString, ":")
	if username == "" || password == "" {
		return fmt.Errorf("key %s in secret %s must be an auth string in the form \"username:password\"", ref.Key, ref.Name)
	}
	if strings.ContainsFunc(authString, func(r rune) bool { return r > unicode.MaxASCII }) {
		return fmt.Errorf("key %s in secret %s must contain only ASCII characters", ref.Key, ref.Name)
	}
	if authString != string(value) {
		return fmt.Errorf("key %s in secret %s must not start or end with whitespace, such as a newline", ref.Key, ref.Name)
	}
	return nil
}

// readSecretKey returns the value of ref's key in the Secret in namespace. It
// returns an error if the Secret or the key does not exist, or if the Secret's
// AnnotationAllowedAPIURLs annotation does not allow apiURL. credential names
// the value in error messages, such as "API key".
func readSecretKey(ctx context.Context, k8sClient client.Client, namespace string, ref *corev1.SecretKeySelector, apiURL, credential string) (string, error) {
	secret := &corev1.Secret{}
	if err := k8sClient.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: namespace}, secret); err != nil {
		return "", fmt.Errorf("failed to get secret %s: %w", ref.Name, err)
	}
	if err := checkAllowedAPIURL("secret", ref.Name, secret.Annotations, apiURL, credential); err != nil {
		return "", err
	}
	value, exists := secret.Data[ref.Key]
	if !exists {
		return "", fmt.Errorf("key %s not found in secret %s", ref.Key, ref.Name)
	}
	return string(value), nil
}

// checkAllowedAPIURL returns an error unless the AnnotationAllowedAPIURLs
// annotation in annotations allows apiURL. kind and name name the object that
// has the annotations, and credential the value it holds, for the error message.
func checkAllowedAPIURL(kind, name string, annotations map[string]string, apiURL, credential string) error {
	allowed, ok := annotations[AnnotationAllowedAPIURLs]
	if !ok {
		return fmt.Errorf("%s %s does not have the annotation %s, which must list the Prefect API URLs that its %s may be used for",
			kind, name, AnnotationAllowedAPIURLs, credential)
	}
	if !APIURLAllowed(allowed, apiURL) {
		return fmt.Errorf("%s %s does not allow its %s to be used for %q: its annotation %s is %q",
			kind, name, credential, apiURL, AnnotationAllowedAPIURLs, allowed)
	}
	return nil
}

// APIURLAllowed reports whether apiURL matches an entry in allowed, a
// comma-separated list of URLs. An entry matches when its scheme and host,
// including any port, equal apiURL's, and its path is apiURL's path or a parent
// of it. A dot at the end of a host name is ignored. Paths are compared after resolving "." and ".." segments, so
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
// lower case, its host name without a trailing dot, and its path cleaned. It returns false for any other URL, or one
// without a host name, such as "http://:4200", or with user information, a
// query or a fragment.
func parseAPIURL(raw string) (*url.URL, bool) {
	u, err := url.Parse(raw)
	if err != nil || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" {
		return nil, false
	}
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)
	if name := u.Hostname(); strings.HasSuffix(name, ".") {
		u.Host = strings.Replace(u.Host, name, strings.TrimSuffix(name, "."), 1)
	}
	if (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return nil, false
	}
	u.Path = path.Clean("/" + u.Path)
	return u, true
}
