package v1

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("PrefectServerReference", func() {
	var (
		ctx       context.Context
		k8sClient client.Client
		namespace string
	)

	BeforeEach(func() {
		ctx = context.Background()
		scheme := runtime.NewScheme()
		Expect(corev1.AddToScheme(scheme)).To(Succeed())
		k8sClient = fake.NewClientBuilder().WithScheme(scheme).Build()
		namespace = "test-namespace"
	})

	Context("GetAPIKey method", func() {

		It("Should return empty string when APIKey is nil", func() {
			serverRef := &PrefectServerReference{
				APIKey: nil,
			}

			apiKey, err := serverRef.GetAPIKey(ctx, k8sClient, namespace)
			Expect(err).NotTo(HaveOccurred())
			Expect(apiKey).To(Equal(""))
		})

		It("Should return direct value when APIKey.Value is set", func() {
			serverRef := &PrefectServerReference{
				APIKey: &APIKeySpec{
					Value: new("direct-api-key-value"),
				},
			}

			apiKey, err := serverRef.GetAPIKey(ctx, k8sClient, namespace)
			Expect(err).NotTo(HaveOccurred())
			Expect(apiKey).To(Equal("direct-api-key-value"))
		})

		It("Should read and check the Secret when both Value and ValueFrom are set", func() {
			Expect(k8sClient.Create(ctx, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "test-secret", Namespace: namespace},
				Data:       map[string][]byte{"api-key": []byte("secret-api-key-value")},
			})).To(Succeed())

			serverRef := &PrefectServerReference{
				RemoteAPIURL: new("https://api.prefect.cloud"),
				APIKey: &APIKeySpec{
					Value: new("direct-api-key-value"),
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "test-secret"},
							Key:                  "api-key",
						},
					},
				},
			}

			_, err := serverRef.GetAPIKey(ctx, k8sClient, namespace)
			Expect(err).To(MatchError(ContainSubstring(AnnotationAllowedAPIURLs)))
		})

		It("Should retrieve API key from Secret when ValueFrom.SecretKeyRef is set", func() {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-secret",
					Namespace:   namespace,
					Annotations: map[string]string{AnnotationAllowedAPIURLs: "https://api.prefect.cloud"},
				},
				Data: map[string][]byte{
					"api-key": []byte("secret-api-key-value"),
				},
			}
			Expect(k8sClient.Create(ctx, secret)).To(Succeed())

			serverRef := &PrefectServerReference{
				RemoteAPIURL: new("https://api.prefect.cloud"),
				APIKey: &APIKeySpec{
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "test-secret"},
							Key:                  "api-key",
						},
					},
				},
			}

			apiKey, err := serverRef.GetAPIKey(ctx, k8sClient, namespace)
			Expect(err).NotTo(HaveOccurred())
			Expect(apiKey).To(Equal("secret-api-key-value"))
		})

		It("Should retrieve API key from ConfigMap when ValueFrom.ConfigMapKeyRef is set", func() {
			configMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-configmap",
					Namespace:   namespace,
					Annotations: map[string]string{AnnotationAllowedAPIURLs: "https://api.prefect.cloud"},
				},
				Data: map[string]string{
					"api-key": "configmap-api-key-value",
				},
			}
			Expect(k8sClient.Create(ctx, configMap)).To(Succeed())

			serverRef := &PrefectServerReference{
				RemoteAPIURL: new("https://api.prefect.cloud"),
				APIKey: &APIKeySpec{
					ValueFrom: &corev1.EnvVarSource{
						ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "test-configmap"},
							Key:                  "api-key",
						},
					},
				},
			}

			apiKey, err := serverRef.GetAPIKey(ctx, k8sClient, namespace)
			Expect(err).NotTo(HaveOccurred())
			Expect(apiKey).To(Equal("configmap-api-key-value"))
		})

		It("Should return error when Secret does not exist", func() {
			serverRef := &PrefectServerReference{
				RemoteAPIURL: new("https://api.prefect.cloud"),
				APIKey: &APIKeySpec{
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "nonexistent-secret"},
							Key:                  "api-key",
						},
					},
				},
			}

			apiKey, err := serverRef.GetAPIKey(ctx, k8sClient, namespace)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to get secret nonexistent-secret"))
			Expect(apiKey).To(Equal(""))
		})

		It("Should return error when ConfigMap does not exist", func() {
			serverRef := &PrefectServerReference{
				RemoteAPIURL: new("https://api.prefect.cloud"),
				APIKey: &APIKeySpec{
					ValueFrom: &corev1.EnvVarSource{
						ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "nonexistent-configmap"},
							Key:                  "api-key",
						},
					},
				},
			}

			apiKey, err := serverRef.GetAPIKey(ctx, k8sClient, namespace)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to get configmap nonexistent-configmap"))
			Expect(apiKey).To(Equal(""))
		})

		It("Should return error when key does not exist in Secret", func() {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-secret",
					Namespace:   namespace,
					Annotations: map[string]string{AnnotationAllowedAPIURLs: "https://api.prefect.cloud"},
				},
				Data: map[string][]byte{
					"wrong-key": []byte("some-value"),
				},
			}
			Expect(k8sClient.Create(ctx, secret)).To(Succeed())

			serverRef := &PrefectServerReference{
				RemoteAPIURL: new("https://api.prefect.cloud"),
				APIKey: &APIKeySpec{
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "test-secret"},
							Key:                  "api-key",
						},
					},
				},
			}

			apiKey, err := serverRef.GetAPIKey(ctx, k8sClient, namespace)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("key api-key not found in secret test-secret"))
			Expect(apiKey).To(Equal(""))
		})

		It("Should return error when key does not exist in ConfigMap", func() {
			configMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-configmap",
					Namespace:   namespace,
					Annotations: map[string]string{AnnotationAllowedAPIURLs: "https://api.prefect.cloud"},
				},
				Data: map[string]string{
					"wrong-key": "some-value",
				},
			}
			Expect(k8sClient.Create(ctx, configMap)).To(Succeed())

			serverRef := &PrefectServerReference{
				RemoteAPIURL: new("https://api.prefect.cloud"),
				APIKey: &APIKeySpec{
					ValueFrom: &corev1.EnvVarSource{
						ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "test-configmap"},
							Key:                  "api-key",
						},
					},
				},
			}

			apiKey, err := serverRef.GetAPIKey(ctx, k8sClient, namespace)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("key api-key not found in configmap test-configmap"))
			Expect(apiKey).To(Equal(""))
		})

		It("Should return empty string when ValueFrom is set but no SecretKeyRef or ConfigMapKeyRef", func() {
			serverRef := &PrefectServerReference{
				RemoteAPIURL: new("https://api.prefect.cloud"),
				APIKey: &APIKeySpec{
					ValueFrom: &corev1.EnvVarSource{},
				},
			}

			apiKey, err := serverRef.GetAPIKey(ctx, k8sClient, namespace)
			Expect(err).NotTo(HaveOccurred())
			Expect(apiKey).To(Equal(""))
		})
	})

	Context("Checking where an API key may be used", func() {
		newServerRef := func(apiURL string, valueFrom *corev1.EnvVarSource) *PrefectServerReference {
			return &PrefectServerReference{RemoteAPIURL: new(apiURL), APIKey: &APIKeySpec{ValueFrom: valueFrom}}
		}
		secretRef := &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: "api-key-secret"}, Key: "api-key",
		}}
		configMapRef := &corev1.EnvVarSource{ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: "api-key-config"}, Key: "api-key",
		}}

		createKeyObjects := func(annotations map[string]string) {
			Expect(k8sClient.Create(ctx, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "api-key-secret", Namespace: namespace, Annotations: annotations},
				Data:       map[string][]byte{"api-key": []byte("secret-api-key-value")},
			})).To(Succeed())
			Expect(k8sClient.Create(ctx, &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "api-key-config", Namespace: namespace, Annotations: annotations},
				Data:       map[string]string{"api-key": "configmap-api-key-value"},
			})).To(Succeed())
		}

		It("Should refuse a Secret or ConfigMap without the annotation", func() {
			createKeyObjects(map[string]string{"other": "annotation"})

			_, err := newServerRef("https://api.prefect.cloud", secretRef).GetAPIKey(ctx, k8sClient, namespace)
			Expect(err).To(MatchError(
				"secret api-key-secret does not have the annotation prefect.io/allowed-api-urls, " +
					"which must list the Prefect API URLs that its API key may be used for"))
			_, err = newServerRef("https://api.prefect.cloud", configMapRef).GetAPIKey(ctx, k8sClient, namespace)
			Expect(err).To(MatchError(ContainSubstring("configmap api-key-config does not have the annotation")))
		})

		It("Should refuse an API URL that the annotation doesn't allow", func() {
			createKeyObjects(map[string]string{AnnotationAllowedAPIURLs: "https://api.prefect.cloud"})

			apiKey, err := newServerRef("https://prefect.example.com", secretRef).GetAPIKey(ctx, k8sClient, namespace)
			Expect(err).To(MatchError(
				`secret api-key-secret does not allow its API key to be used for "https://prefect.example.com/api": ` +
					`its annotation prefect.io/allowed-api-urls is "https://api.prefect.cloud"`))
			Expect(apiKey).To(BeEmpty())
			_, err = newServerRef("https://prefect.example.com", configMapRef).GetAPIKey(ctx, k8sClient, namespace)
			Expect(err).To(MatchError(ContainSubstring("configmap api-key-config does not allow its API key")))
		})

		It("Should check the API URL with the account and workspace", func() {
			createKeyObjects(map[string]string{AnnotationAllowedAPIURLs: "https://api.prefect.cloud/api/accounts/a"})
			serverRef := newServerRef("https://api.prefect.cloud", secretRef)
			serverRef.AccountID = new("a")
			serverRef.WorkspaceID = new("w")

			Expect(serverRef.GetAPIKey(ctx, k8sClient, namespace)).To(Equal("secret-api-key-value"))
			serverRef.AccountID = new("b")
			_, err := serverRef.GetAPIKey(ctx, k8sClient, namespace)
			Expect(err).To(MatchError(ContainSubstring("does not allow its API key")))
		})

		It("Should check an in-cluster server's service URL", func() {
			createKeyObjects(map[string]string{AnnotationAllowedAPIURLs: "http://prefect.prefect-system.svc.cluster.local:4200"})
			serverRef := &PrefectServerReference{Name: "prefect", Namespace: "prefect-system", APIKey: &APIKeySpec{ValueFrom: secretRef}}

			Expect(serverRef.GetAPIKey(ctx, k8sClient, namespace)).To(Equal("secret-api-key-value"))
			serverRef.Name = "other"
			_, err := serverRef.GetAPIKey(ctx, k8sClient, namespace)
			Expect(err).To(MatchError(ContainSubstring("does not allow its API key")))
		})

		DescribeTable("APIURLAllowed",
			func(allowed, apiURL string, want bool) {
				Expect(APIURLAllowed(allowed, apiURL)).To(Equal(want))
			},
			Entry("the same URL", "https://api.prefect.cloud/api", "https://api.prefect.cloud/api", true),
			Entry("a host without a path", "https://api.prefect.cloud", "https://api.prefect.cloud/api/accounts/a", true),
			Entry("a parent path", "https://api.prefect.cloud/api/accounts/a", "https://api.prefect.cloud/api/accounts/a/workspaces/w", true),
			Entry("an absolute host", "http://p.b.svc.cluster.local:4200", "http://p.b.svc.cluster.local.:4200/api", true),
			Entry("an entry with an absolute host", "http://p.b.svc.cluster.local.:4200", "http://p.b.svc.cluster.local:4200/api", true),
			Entry("an absolute host without a port", "https://api.prefect.cloud", "https://api.prefect.cloud./api", true),
			Entry("an absolute host with another port", "http://p.b.svc.cluster.local:4200", "http://p.b.svc.cluster.local.:80/api", false),
			Entry("a host without the cluster domain", "http://p.b.svc:4200", "http://p.b.svc.cluster.local.:4200/api", false),
			Entry("a trailing slash", "https://api.prefect.cloud/api/accounts/a/", "https://api.prefect.cloud/api/accounts/a/workspaces/w", true),
			Entry("scheme and host in another case", "HTTPS://API.Prefect.Cloud", "https://api.prefect.cloud/api", true),
			Entry("the second of several entries", "https://a.example.com, https://api.prefect.cloud", "https://api.prefect.cloud/api", true),
			Entry("a path that only shares a prefix", "https://api.prefect.cloud/api/accounts/a", "https://api.prefect.cloud/api/accounts/ab", false),
			Entry("a dot-dot segment that leaves the path", "https://api.prefect.cloud/api/accounts/a", "https://api.prefect.cloud/api/accounts/a/../b", false),
			Entry("an encoded dot-dot segment", "https://api.prefect.cloud/api/accounts/a", "https://api.prefect.cloud/api/accounts/a/%2e%2e/b", false),
			Entry("a host with the allowed host as a prefix", "https://api.prefect.cloud", "https://api.prefect.cloud.example.com/api", false),
			Entry("a host with the allowed host as a suffix", "https://prefect.cloud", "https://api.prefect.cloud/api", false),
			Entry("user information before the host", "https://api.prefect.cloud", "https://api.prefect.cloud@example.com/api", false),
			Entry("another scheme", "https://api.prefect.cloud", "http://api.prefect.cloud/api", false),
			Entry("another port", "https://api.prefect.cloud", "https://api.prefect.cloud:8443/api", false),
			Entry("an entry with a query", "https://api.prefect.cloud?x=1", "https://api.prefect.cloud/api", false),
			Entry("an entry without a scheme", "api.prefect.cloud", "https://api.prefect.cloud/api", false),
			Entry("an empty annotation", "", "https://api.prefect.cloud/api", false),
			Entry("an empty API URL", "https://api.prefect.cloud", "", false),
			Entry("an entry with a port but no host name", "http://:4200", "http://:4200/api", false),
		)
	})

	Context("GetAPIURL method", func() {
		It("Should return remote API URL when RemoteAPIURL is set", func() {
			serverRef := &PrefectServerReference{
				RemoteAPIURL: new("https://api.prefect.io"),
			}

			apiURL := serverRef.GetAPIURL("test-namespace")
			Expect(apiURL).To(Equal("https://api.prefect.io/api"))
		})

		It("Should append /api to remote URL if not present", func() {
			serverRef := &PrefectServerReference{
				RemoteAPIURL: new("https://custom-server.com"),
			}

			apiURL := serverRef.GetAPIURL("test-namespace")
			Expect(apiURL).To(Equal("https://custom-server.com/api"))
		})

		It("Should not double-append /api to remote URL", func() {
			serverRef := &PrefectServerReference{
				RemoteAPIURL: new("https://custom-server.com/api"),
			}

			apiURL := serverRef.GetAPIURL("test-namespace")
			Expect(apiURL).To(Equal("https://custom-server.com/api"))
		})

		It("Should add Prefect Cloud workspace path when AccountID and WorkspaceID are set", func() {
			serverRef := &PrefectServerReference{
				RemoteAPIURL: new("https://api.prefect.cloud"),
				AccountID:    new("account-123"),
				WorkspaceID:  new("workspace-456"),
			}

			apiURL := serverRef.GetAPIURL("test-namespace")
			Expect(apiURL).To(Equal("https://api.prefect.cloud/api/accounts/account-123/workspaces/workspace-456"))
		})

		It("Should build in-cluster service URL when Name is set", func() {
			serverRef := &PrefectServerReference{
				Name:      "prefect-server",
				Namespace: "prefect-system",
			}

			apiURL := serverRef.GetAPIURL("test-namespace")
			Expect(apiURL).To(Equal("http://prefect-server.prefect-system.svc.cluster.local.:4200/api"))
		})

		It("Should end the in-cluster host with the cluster domain", func() {
			DeferCleanup(func(domain string) { ClusterDomain = domain }, ClusterDomain)
			ClusterDomain = "k8s.example.com"

			serverRef := &PrefectServerReference{Name: "prefect-server"}
			Expect(serverRef.GetAPIURL("team-a")).To(Equal("http://prefect-server.team-a.svc.k8s.example.com.:4200/api"))
		})

		It("Should use fallback namespace when Name is set but Namespace is empty", func() {
			serverRef := &PrefectServerReference{
				Name: "prefect-server",
			}

			apiURL := serverRef.GetAPIURL("fallback-namespace")
			Expect(apiURL).To(Equal("http://prefect-server.fallback-namespace.svc.cluster.local.:4200/api"))
		})

		It("Should return empty string when neither RemoteAPIURL nor Name is set", func() {
			serverRef := &PrefectServerReference{}

			apiURL := serverRef.GetAPIURL("test-namespace")
			Expect(apiURL).To(Equal(""))
		})

		It("Should prioritize RemoteAPIURL over in-cluster Name", func() {
			serverRef := &PrefectServerReference{
				RemoteAPIURL: new("https://external.prefect.io"),
				Name:         "prefect-server",
				Namespace:    "prefect-system",
			}

			apiURL := serverRef.GetAPIURL("test-namespace")
			Expect(apiURL).To(Equal("https://external.prefect.io/api"))
		})
	})

	Context("ValidateAPIURL method", func() {
		It("Should accept an http or https URL, and an in-cluster reference", func() {
			Expect((&PrefectServerReference{RemoteAPIURL: new("https://api.prefect.cloud")}).ValidateAPIURL()).To(Succeed())
			Expect((&PrefectServerReference{RemoteAPIURL: new("http://prefect.example.com:4200/api")}).ValidateAPIURL()).To(Succeed())
			Expect((&PrefectServerReference{Name: "prefect-server"}).ValidateAPIURL()).To(Succeed())
		})

		It("Should refuse a URL with user information, a query or a fragment, or another scheme", func() {
			for _, remote := range []string{
				"ftp://prefect.example.com",
				"https://user@prefect.example.com",
				"http://internal.example.com/target?discard=/api",
				"https://prefect.example.com/api#fragment",
				"http://:4200",
				"http://.:4200",
			} {
				Expect((&PrefectServerReference{RemoteAPIURL: new(remote)}).ValidateAPIURL()).
					To(MatchError(ContainSubstring("is not an http or https URL")), remote)
			}
		})

		It("Should refuse an in-cluster name or namespace that isn't a DNS label", func() {
			for _, serverRef := range []PrefectServerReference{
				{Name: "elsewhere.example.com/x?"},
				{Name: "Prefect"},
				{Name: "prefect", Namespace: "other.example.com/x?"},
			} {
				Expect(serverRef.ValidateAPIURL()).To(MatchError(ContainSubstring("is not a DNS label")), serverRef.Name)
			}
			Expect((&PrefectServerReference{Name: "prefect", Namespace: "prefect-system"}).ValidateAPIURL()).To(Succeed())
		})

		It("Should check only the remote URL when a reference has both", func() {
			serverRef := &PrefectServerReference{RemoteAPIURL: new("https://api.prefect.cloud"), Name: "not/a/label"}
			Expect(serverRef.ValidateAPIURL()).To(Succeed())
		})

		It("Should refuse a query in the account ID", func() {
			serverRef := &PrefectServerReference{
				RemoteAPIURL: new("https://api.prefect.cloud"),
				AccountID:    new("account?x="),
				WorkspaceID:  new("workspace"),
			}
			Expect(serverRef.ValidateAPIURL()).To(HaveOccurred())
		})
	})

	Context("Helper methods", func() {
		It("Should correctly identify remote server references", func() {
			serverRef := &PrefectServerReference{
				RemoteAPIURL: new("https://api.prefect.io"),
			}

			Expect(serverRef.IsRemote()).To(BeTrue())
			Expect(serverRef.IsInCluster()).To(BeFalse())
		})

		It("Should correctly identify in-cluster server references", func() {
			serverRef := &PrefectServerReference{
				Name:      "prefect-server",
				Namespace: "prefect-system",
			}

			Expect(serverRef.IsRemote()).To(BeFalse())
			Expect(serverRef.IsInCluster()).To(BeTrue())
		})

		It("Should correctly identify Prefect Cloud configuration", func() {
			serverRef := &PrefectServerReference{
				RemoteAPIURL: new("https://api.prefect.cloud"),
				AccountID:    new("account-123"),
				WorkspaceID:  new("workspace-456"),
			}

			Expect(serverRef.IsPrefectCloud()).To(BeTrue())
		})

		It("Should not identify as Prefect Cloud when AccountID is missing", func() {
			serverRef := &PrefectServerReference{
				RemoteAPIURL: new("https://api.prefect.cloud"),
				WorkspaceID:  new("workspace-456"),
			}

			Expect(serverRef.IsPrefectCloud()).To(BeFalse())
		})

		It("Should not identify as Prefect Cloud when WorkspaceID is missing", func() {
			serverRef := &PrefectServerReference{
				RemoteAPIURL: new("https://api.prefect.cloud"),
				AccountID:    new("account-123"),
			}

			Expect(serverRef.IsPrefectCloud()).To(BeFalse())
		})

		It("Should not identify as Prefect Cloud when AccountID is empty", func() {
			serverRef := &PrefectServerReference{
				RemoteAPIURL: new("https://api.prefect.cloud"),
				AccountID:    new(""),
				WorkspaceID:  new("workspace-456"),
			}

			Expect(serverRef.IsPrefectCloud()).To(BeFalse())
		})

		It("Should not identify as Prefect Cloud when WorkspaceID is empty", func() {
			serverRef := &PrefectServerReference{
				RemoteAPIURL: new("https://api.prefect.cloud"),
				AccountID:    new("account-123"),
				WorkspaceID:  new(""),
			}

			Expect(serverRef.IsPrefectCloud()).To(BeFalse())
		})

		It("Should handle both remote and in-cluster configurations simultaneously", func() {
			serverRef := &PrefectServerReference{
				RemoteAPIURL: new("https://api.prefect.io"),
				Name:         "prefect-server",
				Namespace:    "prefect-system",
			}

			Expect(serverRef.IsRemote()).To(BeTrue())
			Expect(serverRef.IsInCluster()).To(BeTrue())
		})

		It("Should handle empty RemoteAPIURL pointer", func() {
			serverRef := &PrefectServerReference{
				RemoteAPIURL: new(""),
				Name:         "prefect-server",
			}

			Expect(serverRef.IsRemote()).To(BeFalse())
			Expect(serverRef.IsInCluster()).To(BeTrue())
		})
	})

	Context("Client creation integration", func() {
		It("Should successfully integrate with prefect client creation", func() {
			// Create a Secret for API key
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "prefect-api-secret",
					Namespace:   namespace,
					Annotations: map[string]string{AnnotationAllowedAPIURLs: "https://api.prefect.cloud"},
				},
				Data: map[string][]byte{
					"api-key": []byte("test-api-key-value"),
				},
			}
			Expect(k8sClient.Create(ctx, secret)).To(Succeed())

			// Create a server reference with Secret-based API key
			serverRef := &PrefectServerReference{
				RemoteAPIURL: new("https://api.prefect.cloud"),
				AccountID:    new("account-123"),
				WorkspaceID:  new("workspace-456"),
				APIKey: &APIKeySpec{
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "prefect-api-secret"},
							Key:                  "api-key",
						},
					},
				},
			}

			// Test that GetAPIKey works correctly
			apiKey, err := serverRef.GetAPIKey(ctx, k8sClient, namespace)
			Expect(err).NotTo(HaveOccurred())
			Expect(apiKey).To(Equal("test-api-key-value"))

			// Test that GetAPIURL works correctly
			apiURL := serverRef.GetAPIURL(namespace)
			Expect(apiURL).To(Equal("https://api.prefect.cloud/api/accounts/account-123/workspaces/workspace-456"))

			// Verify combination works for Prefect Cloud
			Expect(serverRef.IsPrefectCloud()).To(BeTrue())
			Expect(serverRef.IsRemote()).To(BeTrue())
			Expect(serverRef.IsInCluster()).To(BeFalse())
		})

		It("Should work with in-cluster server reference", func() {
			serverRef := &PrefectServerReference{
				Name:      "prefect-server",
				Namespace: "prefect-system",
			}

			// Test API URL generation for in-cluster
			apiURL := serverRef.GetAPIURL("fallback-namespace")
			Expect(apiURL).To(Equal("http://prefect-server.prefect-system.svc.cluster.local.:4200/api"))

			// Test API key retrieval when no key is configured
			apiKey, err := serverRef.GetAPIKey(ctx, k8sClient, namespace)
			Expect(err).NotTo(HaveOccurred())
			Expect(apiKey).To(Equal(""))

			// Verify classification
			Expect(serverRef.IsPrefectCloud()).To(BeFalse())
			Expect(serverRef.IsRemote()).To(BeFalse())
			Expect(serverRef.IsInCluster()).To(BeTrue())
		})
	})
})
