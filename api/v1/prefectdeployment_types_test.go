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
					RemoteAPIURL: new("https://api.prefect.cloud/api/accounts/abc/workspaces/def"),
					AccountID:    new("abc-123"),
					WorkspaceID:  new("def-456"),
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
					Namespace: new("default"),
					Name:      "kubernetes-work-pool",
					WorkQueue: new("default"),
				},
				Deployment: PrefectDeploymentConfiguration{
					Description: new("Test deployment"),
					Tags:        []string{"test", "kubernetes"},
					Labels: map[string]string{
						"environment": "test",
						"team":        "platform",
					},
					Entrypoint: "flows.py:my_flow",
					Path:       new("/opt/prefect/flows"),
					Paused:     new(false),
				},
			},
			Status: PrefectDeploymentStatus{
				Id:     new("deployment-123"),
				FlowId: new("flow-456"),
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
				RemoteAPIURL: new("https://api.prefect.cloud/api/accounts/abc/workspaces/def"),
				AccountID:    new("abc-123"),
				WorkspaceID:  new("def-456"),
				APIKey: &APIKeySpec{
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "prefect-api-key"},
							Key:                  "api-key",
						},
					},
				},
			}

			Expect(serverRef.RemoteAPIURL).To(Equal(new("https://api.prefect.cloud/api/accounts/abc/workspaces/def")))
			Expect(serverRef.AccountID).To(Equal(new("abc-123")))
			Expect(serverRef.WorkspaceID).To(Equal(new("def-456")))
			Expect(serverRef.APIKey).NotTo(BeNil())
		})

		It("should support self-hosted Prefect server configuration", func() {
			serverRef := PrefectServerReference{
				RemoteAPIURL: new("https://prefect.example.com/api"),
				APIKey: &APIKeySpec{
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "prefect-api-key"},
							Key:                  "api-key",
						},
					},
				},
			}

			Expect(serverRef.RemoteAPIURL).To(Equal(new("https://prefect.example.com/api")))
			Expect(serverRef.AccountID).To(BeNil())
			Expect(serverRef.WorkspaceID).To(BeNil())
			Expect(serverRef.APIKey).NotTo(BeNil())
		})
	})

	Context("PrefectWorkPoolReference", func() {
		It("should support namespaced work pool reference", func() {
			workPoolRef := PrefectWorkPoolReference{
				Namespace: new("prefect-system"),
				Name:      "kubernetes-work-pool",
				WorkQueue: new("high-priority"),
			}

			Expect(workPoolRef.Namespace).To(Equal(new("prefect-system")))
			Expect(workPoolRef.Name).To(Equal("kubernetes-work-pool"))
			Expect(workPoolRef.WorkQueue).To(Equal(new("high-priority")))
		})

		It("should support work pool reference without namespace", func() {
			workPoolRef := PrefectWorkPoolReference{
				Name:      "process-work-pool",
				WorkQueue: new("default"),
			}

			Expect(workPoolRef.Namespace).To(BeNil())
			Expect(workPoolRef.Name).To(Equal("process-work-pool"))
			Expect(workPoolRef.WorkQueue).To(Equal(new("default")))
		})
	})

	Context("PrefectDeploymentConfiguration", func() {
		It("should support basic deployment configuration", func() {
			deploymentConfig := PrefectDeploymentConfiguration{
				Description: new("My test flow deployment"),
				Tags:        []string{"test", "ci/cd", "production"},
				Labels: map[string]string{
					"environment": "production",
					"team":        "data-engineering",
					"version":     "1.0.0",
				},
				Entrypoint: "flows/etl.py:main_flow",
				Path:       new("/opt/prefect/flows"),
				Paused:     new(false),
			}

			Expect(deploymentConfig.Description).To(Equal(new("My test flow deployment")))
			Expect(deploymentConfig.Tags).To(Equal([]string{"test", "ci/cd", "production"}))
			Expect(deploymentConfig.Labels).To(HaveKeyWithValue("environment", "production"))
			Expect(deploymentConfig.Labels).To(HaveKeyWithValue("team", "data-engineering"))
			Expect(deploymentConfig.Entrypoint).To(Equal("flows/etl.py:main_flow"))
			Expect(deploymentConfig.Path).To(Equal(new("/opt/prefect/flows")))
			Expect(deploymentConfig.Paused).To(Equal(new(false)))
		})

		It("should support deployment with legacy nested schedules", func() {
			// This test verifies backward compatibility - we'll remove this once we migrate
			deploymentConfig := PrefectDeploymentConfiguration{
				Entrypoint: "flows.py:my_flow",
				Schedules: []PrefectSchedule{
					{
						Slug:             "daily-schedule",
						Interval:         new(86400), // 24 hours in seconds
						AnchorDate:       new("2024-01-01T00:00:00Z"),
						Timezone:         new("UTC"),
						Active:           new(true),
						MaxScheduledRuns: new(10),
					},
					{
						Slug:       "hourly-schedule",
						Interval:   new(3600), // 1 hour in seconds
						AnchorDate: new("2024-01-01T00:00:00Z"),
						Timezone:   new("UTC"),
						Active:     new(false),
					},
				},
			}

			Expect(deploymentConfig.Schedules).To(HaveLen(2))
			Expect(deploymentConfig.Schedules[0].Slug).To(Equal("daily-schedule"))
			Expect(deploymentConfig.Schedules[0].Interval).To(Equal(new(86400)))
			Expect(deploymentConfig.Schedules[1].Slug).To(Equal("hourly-schedule"))
			Expect(deploymentConfig.Schedules[1].Active).To(Equal(new(false)))
		})

		It("should support deployment with flattened interval schedules", func() {
			deploymentConfig := PrefectDeploymentConfiguration{
				Entrypoint: "flows.py:my_flow",
				Schedules: []PrefectSchedule{
					{
						Slug:             "daily-interval",
						Interval:         new(86400), // 24 hours in seconds
						AnchorDate:       new("2024-01-01T00:00:00Z"),
						Timezone:         new("UTC"),
						Active:           new(true),
						MaxScheduledRuns: new(10),
					},
					{
						Slug:       "hourly-interval",
						Interval:   new(3600), // 1 hour in seconds
						AnchorDate: new("2024-01-01T00:00:00Z"),
						Timezone:   new("UTC"),
						Active:     new(false),
					},
				},
			}

			Expect(deploymentConfig.Schedules).To(HaveLen(2))
			Expect(deploymentConfig.Schedules[0].Slug).To(Equal("daily-interval"))
			Expect(deploymentConfig.Schedules[0].Interval).To(Equal(new(86400)))
			Expect(deploymentConfig.Schedules[0].AnchorDate).To(Equal(new("2024-01-01T00:00:00Z")))
			Expect(deploymentConfig.Schedules[0].Timezone).To(Equal(new("UTC")))
			Expect(deploymentConfig.Schedules[0].Active).To(Equal(new(true)))
			Expect(deploymentConfig.Schedules[0].MaxScheduledRuns).To(Equal(new(10)))

			Expect(deploymentConfig.Schedules[1].Slug).To(Equal("hourly-interval"))
			Expect(deploymentConfig.Schedules[1].Interval).To(Equal(new(3600)))
			Expect(deploymentConfig.Schedules[1].Active).To(Equal(new(false)))
		})

		It("should support deployment with cron schedules", func() {
			deploymentConfig := PrefectDeploymentConfiguration{
				Entrypoint: "flows.py:my_flow",
				Schedules: []PrefectSchedule{
					{
						Slug:     "daily-9am",
						Cron:     new("0 9 * * *"),
						DayOr:    new(true),
						Timezone: new("America/New_York"),
						Active:   new(true),
					},
					{
						Slug:             "every-5-minutes",
						Cron:             new("*/5 * * * *"),
						Timezone:         new("UTC"),
						Active:           new(true),
						MaxScheduledRuns: new(100),
					},
				},
			}

			Expect(deploymentConfig.Schedules).To(HaveLen(2))
			Expect(deploymentConfig.Schedules[0].Slug).To(Equal("daily-9am"))
			Expect(deploymentConfig.Schedules[0].Cron).To(Equal(new("0 9 * * *")))
			Expect(deploymentConfig.Schedules[0].DayOr).To(Equal(new(true)))
			Expect(deploymentConfig.Schedules[0].Timezone).To(Equal(new("America/New_York")))
			Expect(deploymentConfig.Schedules[0].Active).To(Equal(new(true)))

			Expect(deploymentConfig.Schedules[1].Slug).To(Equal("every-5-minutes"))
			Expect(deploymentConfig.Schedules[1].Cron).To(Equal(new("*/5 * * * *")))
			Expect(deploymentConfig.Schedules[1].Timezone).To(Equal(new("UTC")))
			Expect(deploymentConfig.Schedules[1].MaxScheduledRuns).To(Equal(new(100)))
		})

		It("should support deployment with rrule schedules", func() {
			deploymentConfig := PrefectDeploymentConfiguration{
				Entrypoint: "flows.py:my_flow",
				Schedules: []PrefectSchedule{
					{
						Slug:     "weekly-monday",
						RRule:    new("RRULE:FREQ=WEEKLY;BYDAY=MO"),
						Timezone: new("UTC"),
						Active:   new(true),
					},
					{
						Slug:             "monthly-first-friday",
						RRule:            new("RRULE:FREQ=MONTHLY;BYDAY=1FR"),
						Timezone:         new("America/Los_Angeles"),
						Active:           new(true),
						MaxScheduledRuns: new(12),
					},
				},
			}

			Expect(deploymentConfig.Schedules).To(HaveLen(2))
			Expect(deploymentConfig.Schedules[0].Slug).To(Equal("weekly-monday"))
			Expect(deploymentConfig.Schedules[0].RRule).To(Equal(new("RRULE:FREQ=WEEKLY;BYDAY=MO")))
			Expect(deploymentConfig.Schedules[0].Timezone).To(Equal(new("UTC")))
			Expect(deploymentConfig.Schedules[0].Active).To(Equal(new(true)))

			Expect(deploymentConfig.Schedules[1].Slug).To(Equal("monthly-first-friday"))
			Expect(deploymentConfig.Schedules[1].RRule).To(Equal(new("RRULE:FREQ=MONTHLY;BYDAY=1FR")))
			Expect(deploymentConfig.Schedules[1].Timezone).To(Equal(new("America/Los_Angeles")))
			Expect(deploymentConfig.Schedules[1].MaxScheduledRuns).To(Equal(new(12)))
		})

		It("should support deployment with mixed schedule types", func() {
			deploymentConfig := PrefectDeploymentConfiguration{
				Entrypoint: "flows.py:my_flow",
				Schedules: []PrefectSchedule{
					{
						Slug:       "hourly-interval",
						Interval:   new(3600),
						AnchorDate: new("2024-01-01T00:00:00Z"),
						Timezone:   new("UTC"),
						Active:     new(true),
					},
					{
						Slug:     "daily-cron",
						Cron:     new("0 9 * * *"),
						Timezone: new("America/New_York"),
						Active:   new(true),
					},
					{
						Slug:     "weekly-rrule",
						RRule:    new("RRULE:FREQ=WEEKLY;BYDAY=MO,WE,FR"),
						Timezone: new("Europe/London"),
						Active:   new(true),
					},
				},
			}

			Expect(deploymentConfig.Schedules).To(HaveLen(3))

			// Interval schedule
			Expect(deploymentConfig.Schedules[0].Interval).To(Equal(new(3600)))
			Expect(deploymentConfig.Schedules[0].Cron).To(BeNil())
			Expect(deploymentConfig.Schedules[0].RRule).To(BeNil())

			// Cron schedule
			Expect(deploymentConfig.Schedules[1].Cron).To(Equal(new("0 9 * * *")))
			Expect(deploymentConfig.Schedules[1].Interval).To(BeNil())
			Expect(deploymentConfig.Schedules[1].RRule).To(BeNil())

			// RRule schedule
			Expect(deploymentConfig.Schedules[2].RRule).To(Equal(new("RRULE:FREQ=WEEKLY;BYDAY=MO,WE,FR")))
			Expect(deploymentConfig.Schedules[2].Interval).To(BeNil())
			Expect(deploymentConfig.Schedules[2].Cron).To(BeNil())
		})

		It("should support deployment with concurrency limits", func() {
			deploymentConfig := PrefectDeploymentConfiguration{
				Entrypoint:       "flows.py:my_flow",
				ConcurrencyLimit: new(5),
				GlobalConcurrencyLimit: &PrefectGlobalConcurrencyLimit{
					Active:             new(true),
					Name:               "global-etl-limit",
					Limit:              new(10),
					SlotDecayPerSecond: new("0.1"),
					CollisionStrategy:  new("CANCEL_NEW"),
				},
			}

			Expect(deploymentConfig.ConcurrencyLimit).To(Equal(new(5)))
			Expect(deploymentConfig.GlobalConcurrencyLimit).NotTo(BeNil())
			Expect(deploymentConfig.GlobalConcurrencyLimit.Name).To(Equal("global-etl-limit"))
			Expect(deploymentConfig.GlobalConcurrencyLimit.Limit).To(Equal(new(10)))
			Expect(deploymentConfig.GlobalConcurrencyLimit.CollisionStrategy).To(Equal(new("CANCEL_NEW")))
			Expect(deploymentConfig.GlobalConcurrencyLimit.SlotDecayPerSecond).To(Equal(new("0.1")))
		})

		It("should support deployment with version info", func() {
			deploymentConfig := PrefectDeploymentConfiguration{
				Entrypoint: "flows.py:my_flow",
				VersionInfo: &PrefectVersionInfo{
					Type:    new("git"),
					Version: new("v1.2.3"),
				},
			}

			Expect(deploymentConfig.VersionInfo).NotTo(BeNil())
			Expect(deploymentConfig.VersionInfo.Type).To(Equal(new("git")))
			Expect(deploymentConfig.VersionInfo.Version).To(Equal(new("v1.2.3")))
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
				EnforceParameterSchema: new(true),
			}

			Expect(deploymentConfig.Parameters).To(Equal(parameters))
			Expect(deploymentConfig.JobVariables).To(Equal(jobVariables))
			Expect(deploymentConfig.ParameterOpenApiSchema).To(Equal(parameterSchema))
			Expect(deploymentConfig.EnforceParameterSchema).To(Equal(new(true)))
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
				Id:                 new("deployment-123"),
				FlowId:             new("flow-456"),
				Ready:              true,
				SpecHash:           "abc123def456",
				ObservedGeneration: 2,
			}

			Expect(status.Id).To(Equal(new("deployment-123")))
			Expect(status.FlowId).To(Equal(new("flow-456")))
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
