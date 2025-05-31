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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("PrefectWorkPool type", func() {
	It("can be deep copied", func() {
		original := &PrefectWorkPool{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
			Spec: PrefectWorkPoolSpec{
				Version: ptr.To("0.0.1"),
				Image:   ptr.To("prefecthq/prefect:0.0.1"),
				Server: PrefectServerReference{
					Namespace: "default",
					Name:      "prefect",
				},
				Type:    "kubernetes",
				Workers: int32(2),
			}}

		copied := original.DeepCopy()

		Expect(copied).To(Equal(original))
		Expect(copied).NotTo(BeIdenticalTo(original))
	})

	Context("when providing custom resource specifications ToEnvVars", func() {
		It("should set EXTRA_PIP_PACKAGES based on s.Spec.Settings", func() {
			// Prepare test input
			workPool := &PrefectWorkPool{
				Spec: PrefectWorkPoolSpec{
					Settings: []corev1.EnvVar{
						{
							Name:  "EXTRA_PIP_PACKAGES",
							Value: "pytest==7.0.0",
						},
					},
				},
			}

			// Execute test
			result := workPool.ToEnvVars()

			// Verify results
			Expect(result).To(ContainElement(corev1.EnvVar{
				Name:  "EXTRA_PIP_PACKAGES",
				Value: "pytest==7.0.0",
			}))
		})

		It("should set EXTRA_PIP_PACKAGES based on s.Spec.type", func() {
			workPool := &PrefectWorkPool{
				Spec: PrefectWorkPoolSpec{
					Type: "kubernetes",
				},
			}

			result := workPool.ToEnvVars()

			Expect(result).To(ContainElement(corev1.EnvVar{
				Name:  "EXTRA_PIP_PACKAGES",
				Value: "prefect[kubernetes]",
			}))
		})

		It("should prioritize s.Spec.Settings over s.Spec.type", func() {
			workPool := &PrefectWorkPool{
				Spec: PrefectWorkPoolSpec{
					Type: "kubernetes",
					Settings: []corev1.EnvVar{
						{
							Name:  "EXTRA_PIP_PACKAGES",
							Value: "custom-package==1.0.0",
						},
					},
				},
			}

			result := workPool.ToEnvVars()

			Expect(result).To(ContainElement(corev1.EnvVar{
				Name:  "EXTRA_PIP_PACKAGES",
				Value: "custom-package==1.0.0",
			}))
		})

		It("should gracefully handle inappropriately defined s.Spec.type", func() {
			workPool := &PrefectWorkPool{
				Spec: PrefectWorkPoolSpec{
					Type: "invalid-type",
				},
			}

			result := workPool.ToEnvVars()

			Expect(result).To(ContainElement(corev1.EnvVar{
				Name:  "EXTRA_PIP_PACKAGES",
				Value: "",
			}))
		})

		It("should set default environment variables", func() {
			workPool := &PrefectWorkPool{
				Spec: PrefectWorkPoolSpec{},
			}

			result := workPool.ToEnvVars()

			expectedVars := []corev1.EnvVar{
				{
					Name:  "PREFECT_HOME",
					Value: "/var/lib/prefect/",
				},
				{
					Name:  "PREFECT_WORKER_WEBSERVER_PORT",
					Value: "8080",
				},
			}

			for _, expectedVar := range expectedVars {
				Expect(result).To(ContainElement(expectedVar))
			}
		})

		It("should handle API key configuration with ValueFrom", func() {
			workPool := &PrefectWorkPool{
				Spec: PrefectWorkPoolSpec{
					Server: PrefectServerReference{
						APIKey: &APIKeySpec{
							ValueFrom: &corev1.EnvVarSource{
								SecretKeyRef: &corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "secret-name",
									},
									Key: "api-key",
								},
							},
						},
					},
				},
			}

			result := workPool.ToEnvVars()

			Expect(result).To(ContainElement(corev1.EnvVar{
				Name:      "PREFECT_API_KEY",
				ValueFrom: workPool.Spec.Server.APIKey.ValueFrom,
			}))
		})

		It("should handle API key configuration with direct Value", func() {
			apiKeyValue := "test-api-key"
			workPool := &PrefectWorkPool{
				Spec: PrefectWorkPoolSpec{
					Server: PrefectServerReference{
						APIKey: &APIKeySpec{
							Value: &apiKeyValue,
						},
					},
				},
			}

			result := workPool.ToEnvVars()

			Expect(result).To(ContainElement(corev1.EnvVar{
				Name:  "PREFECT_API_KEY",
				Value: apiKeyValue,
			}))
		})
	})
})
