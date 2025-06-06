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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
)

var _ = Describe("PrefectDeployment type", func() {
	It("can be deep copied", func() {
		original := &PrefectDeployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-deployment",
				Namespace: "default",
			},
			Spec: PrefectDeploymentSpec{
				Server: PrefectServerReference{
					RemoteAPIURL: ptr.To("https://api.prefect.cloud/api/accounts/abc/workspaces/def"),
					AccountID:    ptr.To("abc-123"),
					WorkspaceID:  ptr.To("def-456"),
					APIKey: &APIKeySpec{
						ValueFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{Name: "prefect-api-key"},
								Key:                  "api-key",
							},
						},
					},
				},
				WorkPool: PrefectWorkPoolReference{
					Namespace: ptr.To("default"),
					Name:      "kubernetes-work-pool",
					WorkQueue: ptr.To("default"),
				},
				Deployment: PrefectDeploymentConfiguration{
					Description: ptr.To("Test deployment"),
					Tags:        []string{"test", "kubernetes"},
					Labels: map[string]string{
						"environment": "test",
						"team":        "platform",
					},
					Entrypoint: "flows.py:my_flow",
					Path:       ptr.To("/opt/prefect/flows"),
					Paused:     ptr.To(false),
				},
			},
			Status: PrefectDeploymentStatus{
				Id:     ptr.To("deployment-123"),
				FlowId: ptr.To("flow-456"),
				Ready:  true,
			},
		}

		copied := original.DeepCopy()

		Expect(copied).To(Equal(original))
		Expect(copied).NotTo(BeIdenticalTo(original))
	})

	Context("PrefectServerReference", func() {
		It("should support Prefect Cloud configuration", func() {
			serverRef := PrefectServerReference{
				RemoteAPIURL: ptr.To("https://api.prefect.cloud/api/accounts/abc/workspaces/def"),
				AccountID:    ptr.To("abc-123"),
				WorkspaceID:  ptr.To("def-456"),
				APIKey: &APIKeySpec{
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "prefect-api-key"},
							Key:                  "api-key",
						},
					},
				},
			}

			Expect(serverRef.RemoteAPIURL).To(Equal(ptr.To("https://api.prefect.cloud/api/accounts/abc/workspaces/def")))
			Expect(serverRef.AccountID).To(Equal(ptr.To("abc-123")))
			Expect(serverRef.WorkspaceID).To(Equal(ptr.To("def-456")))
			Expect(serverRef.APIKey).NotTo(BeNil())
		})

		It("should support self-hosted Prefect server configuration", func() {
			serverRef := PrefectServerReference{
				RemoteAPIURL: ptr.To("https://prefect.example.com/api"),
				APIKey: &APIKeySpec{
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "prefect-api-key"},
							Key:                  "api-key",
						},
					},
				},
			}

			Expect(serverRef.RemoteAPIURL).To(Equal(ptr.To("https://prefect.example.com/api")))
			Expect(serverRef.AccountID).To(BeNil())
			Expect(serverRef.WorkspaceID).To(BeNil())
			Expect(serverRef.APIKey).NotTo(BeNil())
		})
	})

	Context("PrefectWorkPoolReference", func() {
		It("should support namespaced work pool reference", func() {
			workPoolRef := PrefectWorkPoolReference{
				Namespace: ptr.To("prefect-system"),
				Name:      "kubernetes-work-pool",
				WorkQueue: ptr.To("high-priority"),
			}

			Expect(workPoolRef.Namespace).To(Equal(ptr.To("prefect-system")))
			Expect(workPoolRef.Name).To(Equal("kubernetes-work-pool"))
			Expect(workPoolRef.WorkQueue).To(Equal(ptr.To("high-priority")))
		})

		It("should support work pool reference without namespace", func() {
			workPoolRef := PrefectWorkPoolReference{
				Name:      "process-work-pool",
				WorkQueue: ptr.To("default"),
			}

			Expect(workPoolRef.Namespace).To(BeNil())
			Expect(workPoolRef.Name).To(Equal("process-work-pool"))
			Expect(workPoolRef.WorkQueue).To(Equal(ptr.To("default")))
		})
	})

	Context("PrefectDeploymentConfiguration", func() {
		It("should support basic deployment configuration", func() {
			deploymentConfig := PrefectDeploymentConfiguration{
				Description: ptr.To("My test flow deployment"),
				Tags:        []string{"test", "ci/cd", "production"},
				Labels: map[string]string{
					"environment": "production",
					"team":        "data-engineering",
					"version":     "1.0.0",
				},
				Entrypoint: "flows/etl.py:main_flow",
				Path:       ptr.To("/opt/prefect/flows"),
				Paused:     ptr.To(false),
			}

			Expect(deploymentConfig.Description).To(Equal(ptr.To("My test flow deployment")))
			Expect(deploymentConfig.Tags).To(Equal([]string{"test", "ci/cd", "production"}))
			Expect(deploymentConfig.Labels).To(HaveKeyWithValue("environment", "production"))
			Expect(deploymentConfig.Labels).To(HaveKeyWithValue("team", "data-engineering"))
			Expect(deploymentConfig.Entrypoint).To(Equal("flows/etl.py:main_flow"))
			Expect(deploymentConfig.Path).To(Equal(ptr.To("/opt/prefect/flows")))
			Expect(deploymentConfig.Paused).To(Equal(ptr.To(false)))
		})

		It("should support deployment with schedules", func() {
			deploymentConfig := PrefectDeploymentConfiguration{
				Entrypoint: "flows.py:my_flow",
				Schedules: []PrefectSchedule{
					{
						Slug: "daily-schedule",
						Schedule: PrefectScheduleConfig{
							Interval:         ptr.To(86400), // 24 hours in seconds
							AnchorDate:       ptr.To("2024-01-01T00:00:00Z"),
							Timezone:         ptr.To("UTC"),
							Active:           ptr.To(true),
							MaxScheduledRuns: ptr.To(10),
						},
					},
					{
						Slug: "hourly-schedule",
						Schedule: PrefectScheduleConfig{
							Interval:   ptr.To(3600), // 1 hour in seconds
							AnchorDate: ptr.To("2024-01-01T00:00:00Z"),
							Timezone:   ptr.To("UTC"),
							Active:     ptr.To(false),
						},
					},
				},
			}

			Expect(deploymentConfig.Schedules).To(HaveLen(2))
			Expect(deploymentConfig.Schedules[0].Slug).To(Equal("daily-schedule"))
			Expect(deploymentConfig.Schedules[0].Schedule.Interval).To(Equal(ptr.To(86400)))
			Expect(deploymentConfig.Schedules[1].Slug).To(Equal("hourly-schedule"))
			Expect(deploymentConfig.Schedules[1].Schedule.Active).To(Equal(ptr.To(false)))
		})

		It("should support deployment with concurrency limits", func() {
			deploymentConfig := PrefectDeploymentConfiguration{
				Entrypoint:       "flows.py:my_flow",
				ConcurrencyLimit: ptr.To(5),
				GlobalConcurrencyLimit: &PrefectGlobalConcurrencyLimit{
					Active:             ptr.To(true),
					Name:               "global-etl-limit",
					Limit:              ptr.To(10),
					SlotDecayPerSecond: ptr.To("0.1"),
					CollisionStrategy:  ptr.To("CANCEL_NEW"),
				},
			}

			Expect(deploymentConfig.ConcurrencyLimit).To(Equal(ptr.To(5)))
			Expect(deploymentConfig.GlobalConcurrencyLimit).NotTo(BeNil())
			Expect(deploymentConfig.GlobalConcurrencyLimit.Name).To(Equal("global-etl-limit"))
			Expect(deploymentConfig.GlobalConcurrencyLimit.Limit).To(Equal(ptr.To(10)))
			Expect(deploymentConfig.GlobalConcurrencyLimit.CollisionStrategy).To(Equal(ptr.To("CANCEL_NEW")))
			Expect(deploymentConfig.GlobalConcurrencyLimit.SlotDecayPerSecond).To(Equal(ptr.To("0.1")))
		})

		It("should support deployment with version info", func() {
			deploymentConfig := PrefectDeploymentConfiguration{
				Entrypoint: "flows.py:my_flow",
				VersionInfo: &PrefectVersionInfo{
					Type:    ptr.To("git"),
					Version: ptr.To("v1.2.3"),
				},
			}

			Expect(deploymentConfig.VersionInfo).NotTo(BeNil())
			Expect(deploymentConfig.VersionInfo.Type).To(Equal(ptr.To("git")))
			Expect(deploymentConfig.VersionInfo.Version).To(Equal(ptr.To("v1.2.3")))
		})

		It("should support deployment with parameters and job variables", func() {
			parameters := &runtime.RawExtension{
				Raw: []byte(`{"param1": "value1", "param2": 42}`),
			}
			jobVariables := &runtime.RawExtension{
				Raw: []byte(`{"env": "production", "replicas": 3}`),
			}
			parameterSchema := &runtime.RawExtension{
				Raw: []byte(`{"type": "object", "properties": {"param1": {"type": "string"}}}`),
			}

			deploymentConfig := PrefectDeploymentConfiguration{
				Entrypoint:             "flows.py:my_flow",
				Parameters:             parameters,
				JobVariables:           jobVariables,
				ParameterOpenApiSchema: parameterSchema,
				EnforceParameterSchema: ptr.To(true),
			}

			Expect(deploymentConfig.Parameters).To(Equal(parameters))
			Expect(deploymentConfig.JobVariables).To(Equal(jobVariables))
			Expect(deploymentConfig.ParameterOpenApiSchema).To(Equal(parameterSchema))
			Expect(deploymentConfig.EnforceParameterSchema).To(Equal(ptr.To(true)))
		})

		It("should support deployment with pull steps", func() {
			pullSteps := []runtime.RawExtension{
				{Raw: []byte(`{"prefect.deployments.steps.git_clone": {"repository": "https://github.com/example/repo.git"}}`)},
				{Raw: []byte(`{"prefect.deployments.steps.pip_install_requirements": {"requirements_file": "requirements.txt"}}`)},
			}

			deploymentConfig := PrefectDeploymentConfiguration{
				Entrypoint: "flows.py:my_flow",
				PullSteps:  pullSteps,
			}

			Expect(deploymentConfig.PullSteps).To(HaveLen(2))
			Expect(deploymentConfig.PullSteps[0].Raw).To(ContainSubstring("git_clone"))
			Expect(deploymentConfig.PullSteps[1].Raw).To(ContainSubstring("pip_install_requirements"))
		})
	})

	Context("PrefectDeploymentStatus", func() {
		It("should track deployment status correctly", func() {
			status := PrefectDeploymentStatus{
				Id:                 ptr.To("deployment-123"),
				FlowId:             ptr.To("flow-456"),
				Ready:              true,
				SpecHash:           "abc123def456",
				ObservedGeneration: 2,
			}

			Expect(status.Id).To(Equal(ptr.To("deployment-123")))
			Expect(status.FlowId).To(Equal(ptr.To("flow-456")))
			Expect(status.Ready).To(BeTrue())
			Expect(status.SpecHash).To(Equal("abc123def456"))
			Expect(status.ObservedGeneration).To(Equal(int64(2)))
		})

		It("should support status conditions", func() {
			status := PrefectDeploymentStatus{
				Ready: false,
				Conditions: []metav1.Condition{
					{
						Type:               "Ready",
						Status:             metav1.ConditionFalse,
						LastTransitionTime: metav1.Now(),
						Reason:             "SyncInProgress",
						Message:            "Syncing deployment with Prefect API",
					},
					{
						Type:               "Synced",
						Status:             metav1.ConditionTrue,
						LastTransitionTime: metav1.Now(),
						Reason:             "SyncComplete",
						Message:            "Successfully synced with Prefect API",
					},
				},
			}

			Expect(status.Conditions).To(HaveLen(2))
			Expect(status.Conditions[0].Type).To(Equal("Ready"))
			Expect(status.Conditions[0].Status).To(Equal(metav1.ConditionFalse))
			Expect(status.Conditions[1].Type).To(Equal("Synced"))
			Expect(status.Conditions[1].Status).To(Equal(metav1.ConditionTrue))
		})
	})
})
