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

package prefect

import (
	"context"
	"encoding/json"

	prefectiov1 "github.com/PrefectHQ/prefect-operator/api/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
)

var _ = Describe("ConvertToDeploymentSpec", func() {
	var (
		k8sDeployment *prefectiov1.PrefectDeployment
		flowID        string
	)

	BeforeEach(func() {
		flowID = "test-flow-123"
		k8sDeployment = &prefectiov1.PrefectDeployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-deployment",
				Namespace: "test-namespace",
			},
			Spec: prefectiov1.PrefectDeploymentSpec{
				WorkPool: prefectiov1.PrefectWorkPoolReference{
					Name: "test-workpool",
				},
				Deployment: prefectiov1.PrefectDeploymentConfiguration{
					Entrypoint: "flows.py:main_flow",
				},
			},
		}
	})

	Context("Basic conversion", func() {
		It("Should convert minimal deployment successfully", func() {
			spec, err := ConvertToDeploymentSpec(k8sDeployment, flowID)

			Expect(err).NotTo(HaveOccurred())
			Expect(spec).NotTo(BeNil())
			Expect(spec.Name).To(Equal("test-deployment"))
			Expect(spec.FlowID).To(Equal(flowID))
			Expect(spec.Entrypoint).To(Equal(ptr.To("flows.py:main_flow")))
			Expect(spec.WorkPoolName).To(Equal(ptr.To("test-workpool")))
		})
	})

	Context("Version info handling", func() {
		It("Should handle nil version info", func() {
			k8sDeployment.Spec.Deployment.VersionInfo = nil

			spec, err := ConvertToDeploymentSpec(k8sDeployment, flowID)

			Expect(err).NotTo(HaveOccurred())
			Expect(spec.Version).To(BeNil())
		})

		It("Should handle valid version info", func() {
			k8sDeployment.Spec.Deployment.VersionInfo = &prefectiov1.PrefectVersionInfo{
				Version: ptr.To("v1.0.0"),
			}

			spec, err := ConvertToDeploymentSpec(k8sDeployment, flowID)

			Expect(err).NotTo(HaveOccurred())
			Expect(spec.Version).To(Equal(ptr.To("v1.0.0")))
		})
	})

	Context("WorkQueue handling", func() {
		It("Should handle nil work queue", func() {
			k8sDeployment.Spec.WorkPool.WorkQueue = nil

			spec, err := ConvertToDeploymentSpec(k8sDeployment, flowID)

			Expect(err).NotTo(HaveOccurred())
			Expect(spec.WorkQueueName).To(BeNil())
		})

		It("Should handle valid work queue", func() {
			k8sDeployment.Spec.WorkPool.WorkQueue = ptr.To("test-queue")

			spec, err := ConvertToDeploymentSpec(k8sDeployment, flowID)

			Expect(err).NotTo(HaveOccurred())
			Expect(spec.WorkQueueName).To(Equal(ptr.To("test-queue")))
		})
	})

	Context("Parameters handling", func() {
		It("Should handle nil parameters", func() {
			k8sDeployment.Spec.Deployment.Parameters = nil

			spec, err := ConvertToDeploymentSpec(k8sDeployment, flowID)

			Expect(err).NotTo(HaveOccurred())
			Expect(spec.Parameters).To(BeNil())
		})

		It("Should handle valid parameters", func() {
			params := map[string]interface{}{
				"key1": "value1",
				"key2": 42,
			}
			paramsJSON, _ := json.Marshal(params)
			k8sDeployment.Spec.Deployment.Parameters = &runtime.RawExtension{
				Raw: paramsJSON,
			}

			spec, err := ConvertToDeploymentSpec(k8sDeployment, flowID)

			Expect(err).NotTo(HaveOccurred())
			Expect(spec.Parameters).To(HaveKey("key1"))
			Expect(spec.Parameters["key1"]).To(Equal("value1"))
			Expect(spec.Parameters).To(HaveKey("key2"))
			Expect(spec.Parameters["key2"]).To(BeNumerically("==", 42))
		})

		It("Should return error for invalid parameters JSON", func() {
			k8sDeployment.Spec.Deployment.Parameters = &runtime.RawExtension{
				Raw: []byte(`{invalid json`),
			}

			spec, err := ConvertToDeploymentSpec(k8sDeployment, flowID)

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to unmarshal parameters"))
			Expect(spec).To(BeNil())
		})
	})

	Context("Job variables handling", func() {
		It("Should handle nil job variables", func() {
			k8sDeployment.Spec.Deployment.JobVariables = nil

			spec, err := ConvertToDeploymentSpec(k8sDeployment, flowID)

			Expect(err).NotTo(HaveOccurred())
			Expect(spec.JobVariables).To(BeNil())
		})

		It("Should handle valid job variables", func() {
			jobVars := map[string]interface{}{
				"cpu":    "100m",
				"memory": "128Mi",
			}
			jobVarsJSON, _ := json.Marshal(jobVars)
			k8sDeployment.Spec.Deployment.JobVariables = &runtime.RawExtension{
				Raw: jobVarsJSON,
			}

			spec, err := ConvertToDeploymentSpec(k8sDeployment, flowID)

			Expect(err).NotTo(HaveOccurred())
			Expect(spec.JobVariables).To(Equal(jobVars))
		})

		It("Should return error for invalid job variables JSON", func() {
			k8sDeployment.Spec.Deployment.JobVariables = &runtime.RawExtension{
				Raw: []byte(`{invalid json`),
			}

			spec, err := ConvertToDeploymentSpec(k8sDeployment, flowID)

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to unmarshal job variables"))
			Expect(spec).To(BeNil())
		})
	})

	Context("Parameter schema handling", func() {
		It("Should handle nil parameter schema", func() {
			k8sDeployment.Spec.Deployment.ParameterOpenApiSchema = nil

			spec, err := ConvertToDeploymentSpec(k8sDeployment, flowID)

			Expect(err).NotTo(HaveOccurred())
			Expect(spec.ParameterOpenAPISchema).To(BeNil())
		})

		It("Should handle valid parameter schema", func() {
			schema := map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{
						"type": "string",
					},
				},
			}
			schemaJSON, _ := json.Marshal(schema)
			k8sDeployment.Spec.Deployment.ParameterOpenApiSchema = &runtime.RawExtension{
				Raw: schemaJSON,
			}

			spec, err := ConvertToDeploymentSpec(k8sDeployment, flowID)

			Expect(err).NotTo(HaveOccurred())
			Expect(spec.ParameterOpenAPISchema).To(Equal(schema))
		})

		It("Should return error for invalid parameter schema JSON", func() {
			k8sDeployment.Spec.Deployment.ParameterOpenApiSchema = &runtime.RawExtension{
				Raw: []byte(`{invalid json`),
			}

			spec, err := ConvertToDeploymentSpec(k8sDeployment, flowID)

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to unmarshal parameter schema"))
			Expect(spec).To(BeNil())
		})
	})

	Context("Pull steps handling", func() {
		It("Should handle nil pull steps", func() {
			k8sDeployment.Spec.Deployment.PullSteps = nil

			spec, err := ConvertToDeploymentSpec(k8sDeployment, flowID)

			Expect(err).NotTo(HaveOccurred())
			Expect(spec.PullSteps).To(BeNil())
		})

		It("Should handle valid pull steps", func() {
			pullStep1 := map[string]interface{}{
				"prefect.deployments.steps.git_clone": map[string]interface{}{
					"repository": "https://github.com/org/repo.git",
				},
			}
			pullStep2 := map[string]interface{}{
				"prefect.deployments.steps.pip_install_requirements": map[string]interface{}{
					"requirements_file": "requirements.txt",
				},
			}

			pullStep1JSON, _ := json.Marshal(pullStep1)
			pullStep2JSON, _ := json.Marshal(pullStep2)
			k8sDeployment.Spec.Deployment.PullSteps = []runtime.RawExtension{
				{Raw: pullStep1JSON},
				{Raw: pullStep2JSON},
			}

			spec, err := ConvertToDeploymentSpec(k8sDeployment, flowID)

			Expect(err).NotTo(HaveOccurred())
			Expect(spec.PullSteps).To(HaveLen(2))
			Expect(spec.PullSteps[0]).To(Equal(pullStep1))
			Expect(spec.PullSteps[1]).To(Equal(pullStep2))
		})

		It("Should return error for invalid pull step JSON", func() {
			k8sDeployment.Spec.Deployment.PullSteps = []runtime.RawExtension{
				{Raw: []byte(`{invalid json`)},
			}

			spec, err := ConvertToDeploymentSpec(k8sDeployment, flowID)

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to unmarshal pull step 0"))
			Expect(spec).To(BeNil())
		})

		It("Should return error for invalid pull step JSON in second step", func() {
			validStep := map[string]interface{}{"valid": "step"}
			validStepJSON, _ := json.Marshal(validStep)
			k8sDeployment.Spec.Deployment.PullSteps = []runtime.RawExtension{
				{Raw: validStepJSON},
				{Raw: []byte(`{invalid json`)},
			}

			spec, err := ConvertToDeploymentSpec(k8sDeployment, flowID)

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to unmarshal pull step 1"))
			Expect(spec).To(BeNil())
		})
	})

	Context("Schedule handling", func() {
		It("Should handle nil schedules", func() {
			k8sDeployment.Spec.Deployment.Schedules = nil

			spec, err := ConvertToDeploymentSpec(k8sDeployment, flowID)

			Expect(err).NotTo(HaveOccurred())
			Expect(spec.Schedules).To(BeNil())
		})

		Context("Interval schedules", func() {
			It("Should handle interval schedule without anchor date", func() {
				k8sDeployment.Spec.Deployment.Schedules = []prefectiov1.PrefectSchedule{
					{
						Slug:     "daily-interval",
						Interval: ptr.To(86400), // 1 day in seconds
						Timezone: ptr.To("UTC"),
						Active:   ptr.To(true),
					},
				}

				spec, err := ConvertToDeploymentSpec(k8sDeployment, flowID)

				Expect(err).NotTo(HaveOccurred())
				Expect(spec.Schedules).To(HaveLen(1))
				Expect(spec.Schedules[0].Schedule.Interval).To(Equal(ptr.To(float64(86400))))
				Expect(spec.Schedules[0].Schedule.Timezone).To(Equal(ptr.To("UTC")))
				Expect(spec.Schedules[0].Active).To(Equal(ptr.To(true)))
				Expect(spec.Schedules[0].Schedule.AnchorDate).To(BeNil())
				// Ensure other schedule types are nil
				Expect(spec.Schedules[0].Schedule.Cron).To(BeNil())
				Expect(spec.Schedules[0].Schedule.RRule).To(BeNil())
			})

			It("Should handle interval schedule with anchor date", func() {
				k8sDeployment.Spec.Deployment.Schedules = []prefectiov1.PrefectSchedule{
					{
						Slug:             "daily-interval",
						Interval:         ptr.To(86400),
						AnchorDate:       ptr.To("2024-01-01T00:00:00Z"),
						Timezone:         ptr.To("UTC"),
						MaxScheduledRuns: ptr.To(10),
					},
				}

				spec, err := ConvertToDeploymentSpec(k8sDeployment, flowID)

				Expect(err).NotTo(HaveOccurred())
				Expect(spec.Schedules).To(HaveLen(1))
				Expect(spec.Schedules[0].Schedule.Interval).To(Equal(ptr.To(float64(86400))))
				Expect(spec.Schedules[0].Schedule.AnchorDate).NotTo(BeNil())
				Expect(spec.Schedules[0].Schedule.AnchorDate.Year()).To(Equal(2024))
				Expect(spec.Schedules[0].MaxScheduledRuns).To(Equal(ptr.To(10)))
			})

			It("Should return error for invalid anchor date format", func() {
				k8sDeployment.Spec.Deployment.Schedules = []prefectiov1.PrefectSchedule{
					{
						Slug:       "daily-interval",
						Interval:   ptr.To(86400),
						AnchorDate: ptr.To("invalid-date-format"),
					},
				}

				spec, err := ConvertToDeploymentSpec(k8sDeployment, flowID)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to parse anchor_date for interval schedule 0"))
				Expect(spec).To(BeNil())
			})
		})

		Context("Cron schedules", func() {
			It("Should handle cron schedule", func() {
				k8sDeployment.Spec.Deployment.Schedules = []prefectiov1.PrefectSchedule{
					{
						Slug:     "daily-9am",
						Cron:     ptr.To("0 9 * * *"),
						DayOr:    ptr.To(true),
						Timezone: ptr.To("America/New_York"),
						Active:   ptr.To(true),
					},
				}

				spec, err := ConvertToDeploymentSpec(k8sDeployment, flowID)

				Expect(err).NotTo(HaveOccurred())
				Expect(spec.Schedules).To(HaveLen(1))
				Expect(spec.Schedules[0].Schedule.Cron).To(Equal(ptr.To("0 9 * * *")))
				Expect(spec.Schedules[0].Schedule.DayOr).To(Equal(ptr.To(true)))
				Expect(spec.Schedules[0].Schedule.Timezone).To(Equal(ptr.To("America/New_York")))
				Expect(spec.Schedules[0].Active).To(Equal(ptr.To(true)))
				// Ensure other schedule types are nil
				Expect(spec.Schedules[0].Schedule.Interval).To(BeNil())
				Expect(spec.Schedules[0].Schedule.AnchorDate).To(BeNil())
				Expect(spec.Schedules[0].Schedule.RRule).To(BeNil())
			})

			It("Should handle cron schedule without day_or field", func() {
				k8sDeployment.Spec.Deployment.Schedules = []prefectiov1.PrefectSchedule{
					{
						Slug:             "every-5-minutes",
						Cron:             ptr.To("*/5 * * * *"),
						Timezone:         ptr.To("UTC"),
						Active:           ptr.To(true),
						MaxScheduledRuns: ptr.To(100),
					},
				}

				spec, err := ConvertToDeploymentSpec(k8sDeployment, flowID)

				Expect(err).NotTo(HaveOccurred())
				Expect(spec.Schedules).To(HaveLen(1))
				Expect(spec.Schedules[0].Schedule.Cron).To(Equal(ptr.To("*/5 * * * *")))
				Expect(spec.Schedules[0].Schedule.DayOr).To(BeNil()) // Should be nil when not specified
				Expect(spec.Schedules[0].MaxScheduledRuns).To(Equal(ptr.To(100)))
			})
		})

		Context("RRule schedules", func() {
			It("Should handle rrule schedule", func() {
				k8sDeployment.Spec.Deployment.Schedules = []prefectiov1.PrefectSchedule{
					{
						Slug:     "weekly-monday",
						RRule:    ptr.To("RRULE:FREQ=WEEKLY;BYDAY=MO"),
						Timezone: ptr.To("UTC"),
						Active:   ptr.To(true),
					},
				}

				spec, err := ConvertToDeploymentSpec(k8sDeployment, flowID)

				Expect(err).NotTo(HaveOccurred())
				Expect(spec.Schedules).To(HaveLen(1))
				Expect(spec.Schedules[0].Schedule.RRule).To(Equal(ptr.To("RRULE:FREQ=WEEKLY;BYDAY=MO")))
				Expect(spec.Schedules[0].Schedule.Timezone).To(Equal(ptr.To("UTC")))
				Expect(spec.Schedules[0].Active).To(Equal(ptr.To(true)))
				// Ensure other schedule types are nil
				Expect(spec.Schedules[0].Schedule.Interval).To(BeNil())
				Expect(spec.Schedules[0].Schedule.AnchorDate).To(BeNil())
				Expect(spec.Schedules[0].Schedule.Cron).To(BeNil())
				Expect(spec.Schedules[0].Schedule.DayOr).To(BeNil())
			})

			It("Should handle complex rrule schedule", func() {
				k8sDeployment.Spec.Deployment.Schedules = []prefectiov1.PrefectSchedule{
					{
						Slug:             "monthly-first-friday",
						RRule:            ptr.To("RRULE:FREQ=MONTHLY;BYDAY=1FR"),
						Timezone:         ptr.To("America/Los_Angeles"),
						Active:           ptr.To(true),
						MaxScheduledRuns: ptr.To(12),
					},
				}

				spec, err := ConvertToDeploymentSpec(k8sDeployment, flowID)

				Expect(err).NotTo(HaveOccurred())
				Expect(spec.Schedules).To(HaveLen(1))
				Expect(spec.Schedules[0].Schedule.RRule).To(Equal(ptr.To("RRULE:FREQ=MONTHLY;BYDAY=1FR")))
				Expect(spec.Schedules[0].Schedule.Timezone).To(Equal(ptr.To("America/Los_Angeles")))
				Expect(spec.Schedules[0].MaxScheduledRuns).To(Equal(ptr.To(12)))
			})
		})

		Context("Mixed schedule types", func() {
			It("Should handle multiple schedules of different types", func() {
				k8sDeployment.Spec.Deployment.Schedules = []prefectiov1.PrefectSchedule{
					{
						Slug:       "hourly-interval",
						Interval:   ptr.To(3600),
						AnchorDate: ptr.To("2024-01-01T00:00:00Z"),
						Timezone:   ptr.To("UTC"),
						Active:     ptr.To(true),
					},
					{
						Slug:     "daily-cron",
						Cron:     ptr.To("0 9 * * *"),
						DayOr:    ptr.To(false),
						Timezone: ptr.To("America/New_York"),
						Active:   ptr.To(true),
					},
					{
						Slug:     "weekly-rrule",
						RRule:    ptr.To("RRULE:FREQ=WEEKLY;BYDAY=MO,WE,FR"),
						Timezone: ptr.To("Europe/London"),
						Active:   ptr.To(true),
					},
				}

				spec, err := ConvertToDeploymentSpec(k8sDeployment, flowID)

				Expect(err).NotTo(HaveOccurred())
				Expect(spec.Schedules).To(HaveLen(3))

				// Interval schedule
				Expect(spec.Schedules[0].Schedule.Interval).To(Equal(ptr.To(float64(3600))))
				Expect(spec.Schedules[0].Schedule.AnchorDate).NotTo(BeNil())
				Expect(spec.Schedules[0].Schedule.Cron).To(BeNil())
				Expect(spec.Schedules[0].Schedule.RRule).To(BeNil())

				// Cron schedule
				Expect(spec.Schedules[1].Schedule.Cron).To(Equal(ptr.To("0 9 * * *")))
				Expect(spec.Schedules[1].Schedule.DayOr).To(Equal(ptr.To(false)))
				Expect(spec.Schedules[1].Schedule.Interval).To(BeNil())
				Expect(spec.Schedules[1].Schedule.RRule).To(BeNil())

				// RRule schedule
				Expect(spec.Schedules[2].Schedule.RRule).To(Equal(ptr.To("RRULE:FREQ=WEEKLY;BYDAY=MO,WE,FR")))
				Expect(spec.Schedules[2].Schedule.Interval).To(BeNil())
				Expect(spec.Schedules[2].Schedule.Cron).To(BeNil())
			})
		})

		Context("Schedule validation", func() {
			It("Should return error when no schedule type is specified", func() {
				k8sDeployment.Spec.Deployment.Schedules = []prefectiov1.PrefectSchedule{
					{
						Slug:     "empty-schedule",
						Timezone: ptr.To("UTC"),
						Active:   ptr.To(true),
						// No interval, cron, or rrule specified
					},
				}

				spec, err := ConvertToDeploymentSpec(k8sDeployment, flowID)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("schedule 0 (empty-schedule): exactly one of interval, cron, or rrule must be specified"))
				Expect(spec).To(BeNil())
			})

			It("Should return error when multiple schedule types are specified", func() {
				k8sDeployment.Spec.Deployment.Schedules = []prefectiov1.PrefectSchedule{
					{
						Slug:     "invalid-schedule",
						Interval: ptr.To(3600),        // interval specified
						Cron:     ptr.To("0 9 * * *"), // cron also specified - invalid!
						Timezone: ptr.To("UTC"),
						Active:   ptr.To(true),
					},
				}

				spec, err := ConvertToDeploymentSpec(k8sDeployment, flowID)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("schedule 0 (invalid-schedule): exactly one of interval, cron, or rrule must be specified"))
				Expect(spec).To(BeNil())
			})

			It("Should return error when all three schedule types are specified", func() {
				k8sDeployment.Spec.Deployment.Schedules = []prefectiov1.PrefectSchedule{
					{
						Slug:     "invalid-schedule",
						Interval: ptr.To(3600),                         // interval specified
						Cron:     ptr.To("0 9 * * *"),                  // cron specified
						RRule:    ptr.To("RRULE:FREQ=WEEKLY;BYDAY=MO"), // rrule specified - all three invalid!
						Timezone: ptr.To("UTC"),
						Active:   ptr.To(true),
					},
				}

				spec, err := ConvertToDeploymentSpec(k8sDeployment, flowID)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("schedule 0 (invalid-schedule): exactly one of interval, cron, or rrule must be specified"))
				Expect(spec).To(BeNil())
			})
		})
	})

	Context("Global concurrency limit handling", func() {
		It("Should handle nil global concurrency limit", func() {
			k8sDeployment.Spec.Deployment.GlobalConcurrencyLimit = nil

			spec, err := ConvertToDeploymentSpec(k8sDeployment, flowID)

			Expect(err).NotTo(HaveOccurred())
			Expect(spec.GlobalConcurrencyLimits).To(BeNil())
		})

		It("Should handle valid global concurrency limit", func() {
			k8sDeployment.Spec.Deployment.GlobalConcurrencyLimit = &prefectiov1.PrefectGlobalConcurrencyLimit{
				Name: "global-limit-1",
			}

			spec, err := ConvertToDeploymentSpec(k8sDeployment, flowID)

			Expect(err).NotTo(HaveOccurred())
			Expect(spec.GlobalConcurrencyLimits).To(Equal([]string{"global-limit-1"}))
		})
	})

	Context("Complete deployment with all fields", func() {
		It("Should handle deployment with all optional fields populated", func() {
			// Set up comprehensive deployment with all fields
			params := map[string]interface{}{"param1": "value1"}
			jobVars := map[string]interface{}{"cpu": "100m"}
			schema := map[string]interface{}{"type": "object"}
			pullStep := map[string]interface{}{"step": "git_clone"}

			paramsJSON, _ := json.Marshal(params)
			jobVarsJSON, _ := json.Marshal(jobVars)
			schemaJSON, _ := json.Marshal(schema)
			pullStepJSON, _ := json.Marshal(pullStep)

			k8sDeployment.Spec.Deployment = prefectiov1.PrefectDeploymentConfiguration{
				Description: ptr.To("Test deployment"),
				Tags:        []string{"test", "example"},
				VersionInfo: &prefectiov1.PrefectVersionInfo{
					Version: ptr.To("v1.0.0"),
				},
				Entrypoint:             "flows.py:main_flow",
				Path:                   ptr.To("/opt/flows"),
				Parameters:             &runtime.RawExtension{Raw: paramsJSON},
				JobVariables:           &runtime.RawExtension{Raw: jobVarsJSON},
				ParameterOpenApiSchema: &runtime.RawExtension{Raw: schemaJSON},
				EnforceParameterSchema: ptr.To(true),
				PullSteps:              []runtime.RawExtension{{Raw: pullStepJSON}},
				Paused:                 ptr.To(false),
				ConcurrencyLimit:       ptr.To(5),
				GlobalConcurrencyLimit: &prefectiov1.PrefectGlobalConcurrencyLimit{
					Name: "global-limit",
				},
				Schedules: []prefectiov1.PrefectSchedule{
					{
						Slug:             "test-schedule",
						Interval:         ptr.To(3600),
						AnchorDate:       ptr.To("2024-01-01T00:00:00Z"),
						Timezone:         ptr.To("UTC"),
						Active:           ptr.To(true),
						MaxScheduledRuns: ptr.To(10),
					},
				},
			}
			k8sDeployment.Spec.WorkPool.WorkQueue = ptr.To("test-queue")

			spec, err := ConvertToDeploymentSpec(k8sDeployment, flowID)

			Expect(err).NotTo(HaveOccurred())
			Expect(spec).NotTo(BeNil())

			// Verify all fields are properly converted
			Expect(spec.Description).To(Equal(ptr.To("Test deployment")))
			Expect(spec.Tags).To(Equal([]string{"test", "example"}))
			Expect(spec.Version).To(Equal(ptr.To("v1.0.0")))
			Expect(spec.Path).To(Equal(ptr.To("/opt/flows")))
			Expect(spec.Parameters).To(Equal(params))
			Expect(spec.JobVariables).To(Equal(jobVars))
			Expect(spec.ParameterOpenAPISchema).To(Equal(schema))
			Expect(spec.EnforceParameterSchema).To(Equal(ptr.To(true)))
			Expect(spec.PullSteps).To(HaveLen(1))
			Expect(spec.PullSteps[0]).To(Equal(pullStep))
			Expect(spec.Paused).To(Equal(ptr.To(false)))
			Expect(spec.ConcurrencyLimit).To(Equal(ptr.To(5)))
			Expect(spec.GlobalConcurrencyLimits).To(Equal([]string{"global-limit"}))
			Expect(spec.WorkQueueName).To(Equal(ptr.To("test-queue")))
			Expect(spec.Schedules).To(HaveLen(1))
			Expect(spec.Schedules[0].Schedule.Interval).To(Equal(ptr.To(float64(3600))))
		})
	})
})

var _ = Describe("UpdateDeploymentStatus", func() {
	var (
		k8sDeployment     *prefectiov1.PrefectDeployment
		prefectDeployment *Deployment
	)

	BeforeEach(func() {
		k8sDeployment = &prefectiov1.PrefectDeployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-deployment",
				Namespace: "test-namespace",
			},
		}

		prefectDeployment = &Deployment{
			ID:     "deployment-123",
			FlowID: "flow-456",
			Status: "READY",
		}
	})

	It("Should update status correctly", func() {
		UpdateDeploymentStatus(k8sDeployment, prefectDeployment)

		Expect(k8sDeployment.Status.Id).To(Equal(ptr.To("deployment-123")))
		Expect(k8sDeployment.Status.FlowId).To(Equal(ptr.To("flow-456")))
		Expect(k8sDeployment.Status.Ready).To(BeTrue())
	})

	It("Should handle non-ready status", func() {
		prefectDeployment.Status = "PENDING"

		UpdateDeploymentStatus(k8sDeployment, prefectDeployment)

		Expect(k8sDeployment.Status.Ready).To(BeFalse())
	})
})

var _ = Describe("GetFlowIDFromDeployment", func() {
	var (
		ctx           context.Context
		mockClient    *MockClient
		k8sDeployment *prefectiov1.PrefectDeployment
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockClient = NewMockClient()
		k8sDeployment = &prefectiov1.PrefectDeployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-deployment",
				Namespace: "test-namespace",
			},
			Spec: prefectiov1.PrefectDeploymentSpec{
				Deployment: prefectiov1.PrefectDeploymentConfiguration{
					Tags:   []string{"test", "deployment"},
					Labels: map[string]string{"env": "test"},
				},
			},
		}
	})

	It("Should get flow ID successfully", func() {
		flowID, err := GetFlowIDFromDeployment(ctx, mockClient, k8sDeployment)

		Expect(err).NotTo(HaveOccurred())
		Expect(flowID).NotTo(BeEmpty())
	})

	It("Should handle flow creation error", func() {
		mockClient.ShouldFailFlowCreate = true
		mockClient.FailureMessage = "mock flow error"

		flowID, err := GetFlowIDFromDeployment(ctx, mockClient, k8sDeployment)

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("failed to create or get flow"))
		Expect(err.Error()).To(ContainSubstring("mock flow error"))
		Expect(flowID).To(BeEmpty())
	})
})
