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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
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
					Value: ptr.To("direct-api-key-value"),
				},
			}

			apiKey, err := serverRef.GetAPIKey(ctx, k8sClient, namespace)
			Expect(err).NotTo(HaveOccurred())
			Expect(apiKey).To(Equal("direct-api-key-value"))
		})

		It("Should retrieve API key from Secret when ValueFrom.SecretKeyRef is set", func() {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secret",
					Namespace: namespace,
				},
				Data: map[string][]byte{
					"api-key": []byte("secret-api-key-value"),
				},
			}
			Expect(k8sClient.Create(ctx, secret)).To(Succeed())

			serverRef := &PrefectServerReference{
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
					Name:      "test-configmap",
					Namespace: namespace,
				},
				Data: map[string]string{
					"api-key": "configmap-api-key-value",
				},
			}
			Expect(k8sClient.Create(ctx, configMap)).To(Succeed())

			serverRef := &PrefectServerReference{
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
					Name:      "test-secret",
					Namespace: namespace,
				},
				Data: map[string][]byte{
					"wrong-key": []byte("some-value"),
				},
			}
			Expect(k8sClient.Create(ctx, secret)).To(Succeed())

			serverRef := &PrefectServerReference{
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
					Name:      "test-configmap",
					Namespace: namespace,
				},
				Data: map[string]string{
					"wrong-key": "some-value",
				},
			}
			Expect(k8sClient.Create(ctx, configMap)).To(Succeed())

			serverRef := &PrefectServerReference{
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
				APIKey: &APIKeySpec{
					ValueFrom: &corev1.EnvVarSource{},
				},
			}

			apiKey, err := serverRef.GetAPIKey(ctx, k8sClient, namespace)
			Expect(err).NotTo(HaveOccurred())
			Expect(apiKey).To(Equal(""))
		})
	})

	Context("GetAPIURL method", func() {
		It("Should return remote API URL when RemoteAPIURL is set", func() {
			serverRef := &PrefectServerReference{
				RemoteAPIURL: ptr.To("https://api.prefect.io"),
			}

			apiURL := serverRef.GetAPIURL("test-namespace")
			Expect(apiURL).To(Equal("https://api.prefect.io/api"))
		})

		It("Should append /api to remote URL if not present", func() {
			serverRef := &PrefectServerReference{
				RemoteAPIURL: ptr.To("https://custom-server.com"),
			}

			apiURL := serverRef.GetAPIURL("test-namespace")
			Expect(apiURL).To(Equal("https://custom-server.com/api"))
		})

		It("Should not double-append /api to remote URL", func() {
			serverRef := &PrefectServerReference{
				RemoteAPIURL: ptr.To("https://custom-server.com/api"),
			}

			apiURL := serverRef.GetAPIURL("test-namespace")
			Expect(apiURL).To(Equal("https://custom-server.com/api"))
		})

		It("Should add Prefect Cloud workspace path when AccountID and WorkspaceID are set", func() {
			serverRef := &PrefectServerReference{
				RemoteAPIURL: ptr.To("https://api.prefect.cloud"),
				AccountID:    ptr.To("account-123"),
				WorkspaceID:  ptr.To("workspace-456"),
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
			Expect(apiURL).To(Equal("http://prefect-server.prefect-system.svc:4200/api"))
		})

		It("Should use fallback namespace when Name is set but Namespace is empty", func() {
			serverRef := &PrefectServerReference{
				Name: "prefect-server",
			}

			apiURL := serverRef.GetAPIURL("fallback-namespace")
			Expect(apiURL).To(Equal("http://prefect-server.fallback-namespace.svc:4200/api"))
		})

		It("Should return empty string when neither RemoteAPIURL nor Name is set", func() {
			serverRef := &PrefectServerReference{}

			apiURL := serverRef.GetAPIURL("test-namespace")
			Expect(apiURL).To(Equal(""))
		})

		It("Should prioritize RemoteAPIURL over in-cluster Name", func() {
			serverRef := &PrefectServerReference{
				RemoteAPIURL: ptr.To("https://external.prefect.io"),
				Name:         "prefect-server",
				Namespace:    "prefect-system",
			}

			apiURL := serverRef.GetAPIURL("test-namespace")
			Expect(apiURL).To(Equal("https://external.prefect.io/api"))
		})
	})

	Context("Helper methods", func() {
		It("Should correctly identify remote server references", func() {
			serverRef := &PrefectServerReference{
				RemoteAPIURL: ptr.To("https://api.prefect.io"),
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
				RemoteAPIURL: ptr.To("https://api.prefect.cloud"),
				AccountID:    ptr.To("account-123"),
				WorkspaceID:  ptr.To("workspace-456"),
			}

			Expect(serverRef.IsPrefectCloud()).To(BeTrue())
		})

		It("Should not identify as Prefect Cloud when AccountID is missing", func() {
			serverRef := &PrefectServerReference{
				RemoteAPIURL: ptr.To("https://api.prefect.cloud"),
				WorkspaceID:  ptr.To("workspace-456"),
			}

			Expect(serverRef.IsPrefectCloud()).To(BeFalse())
		})

		It("Should not identify as Prefect Cloud when WorkspaceID is missing", func() {
			serverRef := &PrefectServerReference{
				RemoteAPIURL: ptr.To("https://api.prefect.cloud"),
				AccountID:    ptr.To("account-123"),
			}

			Expect(serverRef.IsPrefectCloud()).To(BeFalse())
		})

		It("Should not identify as Prefect Cloud when AccountID is empty", func() {
			serverRef := &PrefectServerReference{
				RemoteAPIURL: ptr.To("https://api.prefect.cloud"),
				AccountID:    ptr.To(""),
				WorkspaceID:  ptr.To("workspace-456"),
			}

			Expect(serverRef.IsPrefectCloud()).To(BeFalse())
		})

		It("Should not identify as Prefect Cloud when WorkspaceID is empty", func() {
			serverRef := &PrefectServerReference{
				RemoteAPIURL: ptr.To("https://api.prefect.cloud"),
				AccountID:    ptr.To("account-123"),
				WorkspaceID:  ptr.To(""),
			}

			Expect(serverRef.IsPrefectCloud()).To(BeFalse())
		})

		It("Should handle both remote and in-cluster configurations simultaneously", func() {
			serverRef := &PrefectServerReference{
				RemoteAPIURL: ptr.To("https://api.prefect.io"),
				Name:         "prefect-server",
				Namespace:    "prefect-system",
			}

			Expect(serverRef.IsRemote()).To(BeTrue())
			Expect(serverRef.IsInCluster()).To(BeTrue())
		})

		It("Should handle empty RemoteAPIURL pointer", func() {
			serverRef := &PrefectServerReference{
				RemoteAPIURL: ptr.To(""),
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
					Name:      "prefect-api-secret",
					Namespace: namespace,
				},
				Data: map[string][]byte{
					"api-key": []byte("test-api-key-value"),
				},
			}
			Expect(k8sClient.Create(ctx, secret)).To(Succeed())

			// Create a server reference with Secret-based API key
			serverRef := &PrefectServerReference{
				RemoteAPIURL: ptr.To("https://api.prefect.cloud"),
				AccountID:    ptr.To("account-123"),
				WorkspaceID:  ptr.To("workspace-456"),
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
			Expect(apiURL).To(Equal("http://prefect-server.prefect-system.svc:4200/api"))

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
