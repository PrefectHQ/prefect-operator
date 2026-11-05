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

package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	prefectiov1 "github.com/PrefectHQ/prefect-operator/api/v1"
)

var _ = Describe("PrefectServer controller", func() {
	tolerations := []corev1.Toleration{
		{Key: "dedicated", Operator: corev1.TolerationOpEqual, Value: "prefect", Effect: corev1.TaintEffectNoSchedule},
	}

	var (
		ctx           context.Context
		namespace     *corev1.Namespace
		namespaceName string
		name          types.NamespacedName
		prefectserver *prefectiov1.PrefectServer
	)

	Describe("the PostgreSQL database wait image", func() {
		var (
			controllerReconciler *PrefectServerReconciler
			server               *prefectiov1.PrefectServer
		)

		BeforeEach(func() {
			controllerReconciler = &PrefectServerReconciler{}
			server = &prefectiov1.PrefectServer{
				Spec: prefectiov1.PrefectServerSpec{
					Postgres: &prefectiov1.PostgresConfiguration{},
				},
			}
		})

		It("uses the default image", func() {
			deployment := controllerReconciler.postgresDeploymentSpec(server)
			migrationJob := controllerReconciler.postgresMigrationJob(server)

			Expect(deployment.Template.Spec.InitContainers).To(HaveLen(1))
			Expect(deployment.Template.Spec.InitContainers[0].Image).To(Equal("postgres:16-alpine"))
			Expect(migrationJob.Spec.Template.Spec.InitContainers).To(HaveLen(1))
			Expect(migrationJob.Spec.Template.Spec.InitContainers[0].Image).To(Equal("postgres:16-alpine"))
		})

		It("uses a configured image", func() {
			server.Spec.Postgres.WaitForDatabaseImage = "registry.example.com/postgres:16-alpine"

			deployment := controllerReconciler.postgresDeploymentSpec(server)
			migrationJob := controllerReconciler.postgresMigrationJob(server)

			Expect(deployment.Template.Spec.InitContainers).To(HaveLen(1))
			Expect(deployment.Template.Spec.InitContainers[0].Image).To(Equal("registry.example.com/postgres:16-alpine"))
			Expect(migrationJob.Spec.Template.Spec.InitContainers).To(HaveLen(1))
			Expect(migrationJob.Spec.Template.Spec.InitContainers[0].Image).To(Equal("registry.example.com/postgres:16-alpine"))
		})
	})

	Context("for any server", func() {
		BeforeEach(func() {
			ctx = context.Background()
			namespaceName = fmt.Sprintf("any-ns-%d", time.Now().UnixNano())

			namespace = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: namespaceName},
			}
			Expect(k8sClient.Create(ctx, namespace)).To(Succeed())
		})

		It("should ignore removed PrefectServers", func() {
			serverList := &prefectiov1.PrefectServerList{}
			err := k8sClient.List(ctx, serverList, &client.ListOptions{Namespace: namespaceName})
			Expect(err).NotTo(HaveOccurred())
			Expect(serverList.Items).To(HaveLen(0))

			controllerReconciler := &PrefectServerReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: namespaceName,
					Name:      "nonexistant-prefect",
				},
			})
			Expect(err).NotTo(HaveOccurred())
		})

		It("should allow specifying a full image name", func() {
			prefectserver = &prefectiov1.PrefectServer{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespaceName,
					Name:      "prefect-on-anything",
				},
				Spec: prefectiov1.PrefectServerSpec{
					Image: new("prefecthq/prefect:custom-prefect-image"),
				},
			}
			Expect(k8sClient.Create(ctx, prefectserver)).To(Succeed())

			controllerReconciler := &PrefectServerReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: namespaceName,
					Name:      "prefect-on-anything",
				},
			})
			Expect(err).NotTo(HaveOccurred())

			deployment := &appsv1.Deployment{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{
					Namespace: namespaceName,
					Name:      "prefect-on-anything",
				}, deployment)
			}).Should(Succeed())

			Expect(deployment.Spec.Template.Spec.Containers).To(HaveLen(1))
			container := deployment.Spec.Template.Spec.Containers[0]
			Expect(container.Image).To(Equal("prefecthq/prefect:custom-prefect-image"))
		})

		It("should allow specifying a Prefect version", func() {
			version := "3.3.3.3.3.3.3.3"
			expectedImage := fmt.Sprintf("prefecthq/prefect:%s-python3.12", version)
			prefectserver = &prefectiov1.PrefectServer{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespaceName,
					Name:      "prefect-on-anything",
				},
				Spec: prefectiov1.PrefectServerSpec{
					Version: &version,
				},
			}
			Expect(k8sClient.Create(ctx, prefectserver)).To(Succeed())

			controllerReconciler := &PrefectServerReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: namespaceName,
					Name:      "prefect-on-anything",
				},
			})
			Expect(err).NotTo(HaveOccurred())

			deployment := &appsv1.Deployment{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{
					Namespace: namespaceName,
					Name:      "prefect-on-anything",
				}, deployment)
			}).Should(Succeed())

			Expect(deployment.Spec.Template.Spec.Containers).To(HaveLen(1))
			container := deployment.Spec.Template.Spec.Containers[0]
			Expect(container.Image).To(Equal(expectedImage))
		})

		Context("when creating any server", func() {
			var deployment *appsv1.Deployment
			var service *corev1.Service

			BeforeEach(func() {
				name = types.NamespacedName{
					Namespace: namespaceName,
					Name:      "prefect-on-anything",
				}

				prefectserver = &prefectiov1.PrefectServer{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: namespaceName,
						Name:      "prefect-on-anything",
					},
					Spec: prefectiov1.PrefectServerSpec{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("100m"),
								corev1.ResourceMemory: resource.MustParse("128Mi"),
							},
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("500m"),
								corev1.ResourceMemory: resource.MustParse("512Mi"),
							},
						},
						DeploymentLabels: map[string]string{
							"some":    "additional-label",
							"another": "extra-label",
						},
						ServiceLabels: map[string]string{
							"app":  "prefect-server",
							"foo":  "bar",
							"fuzz": "buzz",
						},
						NodeSelector: map[string]string{
							"some": "node-selector",
						},
						Tolerations: tolerations,
					},
				}
				Expect(k8sClient.Create(ctx, prefectserver)).To(Succeed())

				controllerReconciler := &PrefectServerReconciler{
					Client: k8sClient,
					Scheme: k8sClient.Scheme(),
				}

				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: name,
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(k8sClient.Get(ctx, name, prefectserver)).To(Succeed())

				deployment = &appsv1.Deployment{}
				Eventually(func() error {
					return k8sClient.Get(ctx, types.NamespacedName{
						Namespace: namespaceName,
						Name:      "prefect-on-anything",
					}, deployment)
				}).Should(Succeed())

				service = &corev1.Service{}
				Eventually(func() error {
					return k8sClient.Get(ctx, types.NamespacedName{
						Namespace: namespaceName,
						Name:      "prefect-on-anything",
					}, service)
				}).Should(Succeed())
			})

			Describe("the PrefectServer", func() {
				It("should have the DeploymentReconciled condition", func() {
					condition := meta.FindStatusCondition(prefectserver.Status.Conditions, "DeploymentReconciled")
					Expect(condition).NotTo(BeNil())
					Expect(condition.Status).To(Equal(metav1.ConditionTrue))
					Expect(condition.Reason).To(Equal("DeploymentCreated"))
					Expect(condition.Message).To(Equal("Deployment was created"))
				})

				It("should have the ServiceReconciled condition", func() {
					condition := meta.FindStatusCondition(prefectserver.Status.Conditions, "ServiceReconciled")
					Expect(condition).NotTo(BeNil())
					Expect(condition.Status).To(Equal(metav1.ConditionTrue))
					Expect(condition.Reason).To(Equal("ServiceCreated"))
					Expect(condition.Message).To(Equal("Service was created"))
				})

				It("should have the PersistentVolumeClaimReconciled condition", func() {
					condition := meta.FindStatusCondition(prefectserver.Status.Conditions, "PersistentVolumeClaimReconciled")
					Expect(condition).NotTo(BeNil())
					Expect(condition.Status).To(Equal(metav1.ConditionTrue))
					Expect(condition.Reason).To(Equal("PersistentVolumeClaimNotRequired"))
					Expect(condition.Message).To(Equal("PersistentVolumeClaim is not required"))
				})

				It("should have the MigrationJobReconciled condition", func() {
					condition := meta.FindStatusCondition(prefectserver.Status.Conditions, "MigrationJobReconciled")
					Expect(condition).NotTo(BeNil())
					Expect(condition.Status).To(Equal(metav1.ConditionTrue))
					Expect(condition.Reason).To(Equal("MigrationJobNotRequired"))
					Expect(condition.Message).To(Equal("MigrationJob is not required"))
				})
			})

			Describe("the Deployment", func() {
				It("should be owned by the PrefectServer", func() {
					Expect(deployment.OwnerReferences).To(ContainElement(
						metav1.OwnerReference{
							APIVersion:         "prefect.io/v1",
							Kind:               "PrefectServer",
							Name:               "prefect-on-anything",
							UID:                prefectserver.UID,
							Controller:         new(true),
							BlockOwnerDeletion: new(true),
						},
					))
				})

				It("should have appropriate labels", func() {
					Expect(deployment.Spec.Selector.MatchLabels).To(Equal(map[string]string{
						"prefect.io/server": "prefect-on-anything",
						"app":               "prefect-server",
						"some":              "additional-label",
						"another":           "extra-label",
					}))
					Expect(deployment.Spec.Template.Labels).To(Equal(map[string]string{
						"prefect.io/server": "prefect-on-anything",
						"app":               "prefect-server",
						"some":              "additional-label",
						"another":           "extra-label",
					}))
				})

				It("should have a server container with the right image and entrypoint arguments", func() {
					Expect(deployment.Spec.Template.Spec.Containers).To(HaveLen(1))
					container := deployment.Spec.Template.Spec.Containers[0]

					expectedImage := fmt.Sprintf("prefecthq/prefect:%s-python3.12", prefectiov1.DEFAULT_PREFECT_VERSION)
					Expect(container.Name).To(Equal("prefect-server"))
					Expect(container.Image).To(Equal(expectedImage))
					Expect(container.Command).To(BeNil())
					Expect(container.Args).To(Equal([]string{"prefect", "server", "start", "--host", "0.0.0.0"}))
				})

				It("should have an environment with PREFECT_HOME set", func() {
					Expect(deployment.Spec.Template.Spec.Containers).To(HaveLen(1))
					container := deployment.Spec.Template.Spec.Containers[0]

					Expect(container.Env).To(ContainElements([]corev1.EnvVar{
						{Name: "PREFECT_HOME", Value: "/var/lib/prefect/"},
					}))
				})

				It("should expose the Prefect server on port 4200", func() {
					Expect(deployment.Spec.Template.Spec.Containers).To(HaveLen(1))
					container := deployment.Spec.Template.Spec.Containers[0]

					Expect(container.Ports).To(ConsistOf([]corev1.ContainerPort{
						{Name: "api", ContainerPort: 4200, Protocol: corev1.ProtocolTCP},
					}))
				})

				It("should have the specified resource requirements", func() {
					Expect(deployment.Spec.Template.Spec.Containers).To(HaveLen(1))
					container := deployment.Spec.Template.Spec.Containers[0]

					Expect(container.Resources.Requests).To(Equal(corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("100m"),
						corev1.ResourceMemory: resource.MustParse("128Mi"),
					}))
					Expect(container.Resources.Limits).To(Equal(corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("512Mi"),
					}))
				})

				It("should have the correct startup, readiness, and liveness probes", func() {
					Expect(deployment.Spec.Template.Spec.Containers).To(HaveLen(1))
					container := deployment.Spec.Template.Spec.Containers[0]

					Expect(container.StartupProbe).To(Equal(&corev1.Probe{
						ProbeHandler: corev1.ProbeHandler{
							HTTPGet: &corev1.HTTPGetAction{
								Path:   "/api/health",
								Port:   intstr.FromInt(4200),
								Scheme: corev1.URISchemeHTTP,
							},
						},
						InitialDelaySeconds: 10,
						PeriodSeconds:       5,
						TimeoutSeconds:      5,
						SuccessThreshold:    1,
						FailureThreshold:    30,
					}))

					Expect(container.ReadinessProbe).To(Equal(&corev1.Probe{
						ProbeHandler: corev1.ProbeHandler{
							HTTPGet: &corev1.HTTPGetAction{
								Path:   "/api/health",
								Port:   intstr.FromInt(4200),
								Scheme: corev1.URISchemeHTTP,
							},
						},
						InitialDelaySeconds: 10,
						PeriodSeconds:       5,
						TimeoutSeconds:      5,
						SuccessThreshold:    1,
						FailureThreshold:    30,
					}))

					Expect(container.LivenessProbe).To(Equal(&corev1.Probe{
						ProbeHandler: corev1.ProbeHandler{
							HTTPGet: &corev1.HTTPGetAction{
								Path:   "/api/health",
								Port:   intstr.FromInt(4200),
								Scheme: corev1.URISchemeHTTP,
							},
						},
						InitialDelaySeconds: 120,
						PeriodSeconds:       10,
						TimeoutSeconds:      5,
						SuccessThreshold:    1,
						FailureThreshold:    2,
					}))
				})

				It("should have the correct nodeSelector value", func() {
					Expect(deployment.Spec.Template.Spec.NodeSelector).To(HaveKeyWithValue("some", "node-selector"))
				})

				It("should have the correct tolerations", func() {
					Expect(deployment.Spec.Template.Spec.Tolerations).To(Equal(tolerations))
				})
			})

			Describe("the Service", func() {
				It("should be owned by the PrefectServer", func() {
					Expect(service.OwnerReferences).To(ContainElement(
						metav1.OwnerReference{
							APIVersion:         "prefect.io/v1",
							Kind:               "PrefectServer",
							Name:               "prefect-on-anything",
							UID:                prefectserver.UID,
							Controller:         new(true),
							BlockOwnerDeletion: new(true),
						},
					))
				})

				It("should have matching labels", func() {
					Expect(service.Labels).To(Equal(map[string]string{
						"prefect.io/server": "prefect-on-anything",
						"app":               "prefect-server",
						"foo":               "bar",
						"fuzz":              "buzz",
					}))
					Expect(service.Spec.Selector).To(Equal(map[string]string{
						"prefect.io/server": "prefect-on-anything",
						"some":              "additional-label",
						"another":           "extra-label",
						"app":               "prefect-server",
					}))
				})

				It("should expose the API port", func() {
					Expect(service.Spec.Ports).To(ConsistOf(corev1.ServicePort{
						Name:       "api",
						Protocol:   corev1.ProtocolTCP,
						Port:       4200,
						TargetPort: intstr.FromString("api"),
					}))
				})
			})
		})

		Context("When updating any server", func() {
			BeforeEach(func() {
				name = types.NamespacedName{
					Namespace: namespaceName,
					Name:      "prefect-on-anything",
				}

				prefectserver = &prefectiov1.PrefectServer{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: namespaceName,
						Name:      "prefect-on-anything",
					},
					Spec: prefectiov1.PrefectServerSpec{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("100m"),
								corev1.ResourceMemory: resource.MustParse("128Mi"),
							},
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("500m"),
								corev1.ResourceMemory: resource.MustParse("512Mi"),
							},
						},
					},
				}
				Expect(k8sClient.Create(ctx, prefectserver)).To(Succeed())

				controllerReconciler := &PrefectServerReconciler{
					Client: k8sClient,
					Scheme: k8sClient.Scheme(),
				}

				// Reconcile once to create the server
				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: name,
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(k8sClient.Get(ctx, name, prefectserver)).To(Succeed())

				prefectserver.Spec.Settings = []corev1.EnvVar{
					{Name: "PREFECT_SOME_SETTING", Value: "some-value"},
				}
				Expect(k8sClient.Update(ctx, prefectserver)).To(Succeed())

				// Reconcile again to update the server
				_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: name,
				})
				Expect(err).NotTo(HaveOccurred())
			})

			It("should update the Deployment with the new setting", func() {
				deployment := &appsv1.Deployment{}
				Eventually(func() error {
					return k8sClient.Get(ctx, types.NamespacedName{
						Namespace: namespaceName,
						Name:      "prefect-on-anything",
					}, deployment)
				}).Should(Succeed())

				Expect(deployment.Spec.Template.Spec.Containers).To(HaveLen(1))
				container := deployment.Spec.Template.Spec.Containers[0]
				Expect(container.Env).To(ContainElement(corev1.EnvVar{
					Name:  "PREFECT_SOME_SETTING",
					Value: "some-value",
				}))
			})

			It("should remove scheduling constraints cleared from the server", func() {
				controllerReconciler := &PrefectServerReconciler{
					Client: k8sClient,
					Scheme: k8sClient.Scheme(),
				}

				Expect(k8sClient.Get(ctx, name, prefectserver)).To(Succeed())
				prefectserver.Spec.Tolerations = tolerations
				prefectserver.Spec.NodeSelector = map[string]string{"keep": "this", "remove": "this"}
				prefectserver.Spec.Affinity = &corev1.Affinity{
					PodAntiAffinity: &corev1.PodAntiAffinity{
						PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{{
							Weight: 1,
							PodAffinityTerm: corev1.PodAffinityTerm{
								TopologyKey:   "kubernetes.io/hostname",
								LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "prefect-server"}},
							},
						}},
					},
				}
				Expect(k8sClient.Update(ctx, prefectserver)).To(Succeed())
				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: name})
				Expect(err).NotTo(HaveOccurred())

				deployment := &appsv1.Deployment{}
				Expect(k8sClient.Get(ctx, name, deployment)).To(Succeed())
				Expect(deployment.Spec.Template.Spec.Tolerations).To(Equal(tolerations))
				Expect(deployment.Spec.Template.Spec.NodeSelector).To(HaveKey("remove"))
				Expect(deployment.Spec.Template.Spec.Affinity).NotTo(BeNil())

				Expect(k8sClient.Get(ctx, name, prefectserver)).To(Succeed())
				prefectserver.Spec.Tolerations = nil
				prefectserver.Spec.NodeSelector = map[string]string{"keep": "this"}
				prefectserver.Spec.Affinity = nil
				Expect(k8sClient.Update(ctx, prefectserver)).To(Succeed())
				_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: name})
				Expect(err).NotTo(HaveOccurred())

				Expect(k8sClient.Get(ctx, name, deployment)).To(Succeed())
				Expect(deployment.Spec.Template.Spec.Tolerations).To(BeEmpty())
				Expect(deployment.Spec.Template.Spec.NodeSelector).To(Equal(map[string]string{"keep": "this"}))
				Expect(deployment.Spec.Template.Spec.Affinity).To(BeNil())
			})

			It("should update the Deployment with new resource requirements", func() {
				// Update the PrefectServer with new resource requirements
				Expect(k8sClient.Get(ctx, name, prefectserver)).To(Succeed())
				prefectserver.Spec.Resources = corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("200m"),
						corev1.ResourceMemory: resource.MustParse("256Mi"),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("1"),
						corev1.ResourceMemory: resource.MustParse("1Gi"),
					},
				}
				Expect(k8sClient.Update(ctx, prefectserver)).To(Succeed())

				// Reconcile to apply the changes
				controllerReconciler := &PrefectServerReconciler{
					Client: k8sClient,
					Scheme: k8sClient.Scheme(),
				}
				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: name,
				})
				Expect(err).NotTo(HaveOccurred())

				// Check if the Deployment was updated with new resource requirements
				deployment := &appsv1.Deployment{}
				Eventually(func() error {
					return k8sClient.Get(ctx, types.NamespacedName{
						Namespace: namespaceName,
						Name:      "prefect-on-anything",
					}, deployment)
				}).Should(Succeed())

				Expect(deployment.Spec.Template.Spec.Containers).To(HaveLen(1))
				container := deployment.Spec.Template.Spec.Containers[0]
				Expect(container.Resources.Requests).To(Equal(corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("200m"),
					corev1.ResourceMemory: resource.MustParse("256Mi"),
				}))
				Expect(container.Resources.Limits).To(Equal(corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("1"),
					corev1.ResourceMemory: resource.MustParse("1Gi"),
				}))
			})

			It("should update the Deployment with the extra container", func() {
				// Update the PrefectServer with an extra container
				Expect(k8sClient.Get(ctx, name, prefectserver)).To(Succeed())
				prefectserver.Spec.ExtraContainers = []corev1.Container{
					{
						Name:  "extra-container",
						Image: "extra-image",
					},
				}
				Expect(k8sClient.Update(ctx, prefectserver)).To(Succeed())

				// Reconcile to apply the changes
				controllerReconciler := &PrefectServerReconciler{
					Client: k8sClient,
					Scheme: k8sClient.Scheme(),
				}
				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: name,
				})
				Expect(err).NotTo(HaveOccurred())

				// Check if the Deployment was updated with the extra container
				deployment := &appsv1.Deployment{}
				Eventually(func() error {
					return k8sClient.Get(ctx, types.NamespacedName{
						Namespace: namespaceName,
						Name:      "prefect-on-anything",
					}, deployment)
				}).Should(Succeed())

				Expect(deployment.Spec.Template.Spec.Containers).To(HaveLen(2))
				container := deployment.Spec.Template.Spec.Containers[1]
				Expect(container.Name).To(Equal("extra-container"))
			})

			It("should update the Service with the extra port", func() {
				// Update the PrefectServer with an extra port
				Expect(k8sClient.Get(ctx, name, prefectserver)).To(Succeed())
				prefectserver.Spec.ExtraServicePorts = []corev1.ServicePort{
					{
						Name:       "extra-port",
						Port:       4300,
						TargetPort: intstr.FromString("extra-port"),
						Protocol:   corev1.ProtocolTCP,
					},
				}

				Expect(k8sClient.Update(ctx, prefectserver)).To(Succeed())
				// Reconcile to apply the changes
				controllerReconciler := &PrefectServerReconciler{
					Client: k8sClient,
					Scheme: k8sClient.Scheme(),
				}
				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: name,
				})
				Expect(err).NotTo(HaveOccurred())

				// Check if the Service was updated with the extra port
				service := &corev1.Service{}
				Eventually(func() error {
					return k8sClient.Get(ctx, types.NamespacedName{
						Namespace: namespaceName,
						Name:      prefectserver.Name,
					}, service)
				}).Should(Succeed())

				Expect(service.Spec.Ports).To(HaveLen(2))
				port := service.Spec.Ports[1]
				Expect(port.Name).To(Equal("extra-port"))
			})

			It("should update the entrypoint arguments with the extra args", func() {
				// Update the PrefectServer with an extra args
				Expect(k8sClient.Get(ctx, name, prefectserver)).To(Succeed())
				prefectserver.Spec.ExtraArgs = []string{"--some-arg", "some-value"}

				Expect(k8sClient.Update(ctx, prefectserver)).To(Succeed())
				// Reconcile to apply the changes
				controllerReconciler := &PrefectServerReconciler{
					Client: k8sClient,
					Scheme: k8sClient.Scheme(),
				}
				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: name,
				})
				Expect(err).NotTo(HaveOccurred())

				// Check if the Deployment was updated with the extra args
				deployment := &appsv1.Deployment{}
				Eventually(func() error {
					return k8sClient.Get(ctx, types.NamespacedName{
						Namespace: namespaceName,
						Name:      "prefect-on-anything",
					}, deployment)
				}).Should(Succeed())

				container := deployment.Spec.Template.Spec.Containers[0]
				Expect(container.Command).To(BeNil())
				Expect(container.Args).To(Equal([]string{"prefect", "server", "start", "--host", "0.0.0.0", "--some-arg", "some-value"}))
			})

			It("should create a Deployment with IPv6/dual-stack host (empty string)", func() {
				name := types.NamespacedName{
					Namespace: namespaceName,
					Name:      "prefect-ipv6-server",
				}

				prefectserver := &prefectiov1.PrefectServer{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: namespaceName,
						Name:      "prefect-ipv6-server",
					},
					Spec: prefectiov1.PrefectServerSpec{
						Host: new(""),
					},
				}
				Expect(k8sClient.Create(ctx, prefectserver)).To(Succeed())

				controllerReconciler := &PrefectServerReconciler{
					Client: k8sClient,
					Scheme: k8sClient.Scheme(),
				}
				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: name,
				})
				Expect(err).NotTo(HaveOccurred())

				deployment := &appsv1.Deployment{}
				Eventually(func() error {
					return k8sClient.Get(ctx, name, deployment)
				}).Should(Succeed())

				container := deployment.Spec.Template.Spec.Containers[0]
				Expect(container.Args).To(Equal([]string{"prefect", "server", "start", "--host", ""}))
			})
		})

		Context("When evaluating changes with any server", func() {
			BeforeEach(func() {
				name = types.NamespacedName{
					Namespace: namespaceName,
					Name:      "prefect-on-anything-no-changes",
				}

				prefectserver = &prefectiov1.PrefectServer{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: namespaceName,
						Name:      "prefect-on-anything-no-changes",
					},
				}
				Expect(k8sClient.Create(ctx, prefectserver)).To(Succeed())

				controllerReconciler := &PrefectServerReconciler{
					Client: k8sClient,
					Scheme: k8sClient.Scheme(),
				}

				// Reconcile once to create the server
				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: name,
				})
				Expect(err).NotTo(HaveOccurred())
			})

			It("should not change a deployment if nothing has changed", func() {
				before := &appsv1.Deployment{}
				Expect(k8sClient.Get(ctx, types.NamespacedName{
					Namespace: namespaceName,
					Name:      "prefect-on-anything-no-changes",
				}, before)).To(Succeed())

				controllerReconciler := &PrefectServerReconciler{
					Client: k8sClient,
					Scheme: k8sClient.Scheme(),
				}

				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: name,
				})
				Expect(err).NotTo(HaveOccurred())

				after := &appsv1.Deployment{}
				Expect(k8sClient.Get(ctx, types.NamespacedName{
					Namespace: namespaceName,
					Name:      "prefect-on-anything-no-changes",
				}, after)).To(Succeed())

				Expect(after.Generation).To(Equal(before.Generation))
				Expect(after).To(Equal(before))
			})
		})
	})

	Context("for ephemeral servers", func() {
		BeforeEach(func() {
			ctx = context.Background()
			namespaceName = fmt.Sprintf("ephemeral-ns-%d", time.Now().UnixNano())

			namespace = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: namespaceName},
			}
			Expect(k8sClient.Create(ctx, namespace)).To(Succeed())

			name = types.NamespacedName{
				Namespace: namespaceName,
				Name:      "prefect-on-ephemeral",
			}
		})

		Context("when creating an ephemeral server", func() {
			var deployment *appsv1.Deployment
			var service *corev1.Service

			BeforeEach(func() {
				prefectserver = &prefectiov1.PrefectServer{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: namespaceName,
						Name:      "prefect-on-ephemeral",
					},
				}
				Expect(k8sClient.Create(ctx, prefectserver)).To(Succeed())

				controllerReconciler := &PrefectServerReconciler{
					Client: k8sClient,
					Scheme: k8sClient.Scheme(),
				}

				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: name,
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(k8sClient.Get(ctx, name, prefectserver)).To(Succeed())

				deployment = &appsv1.Deployment{}
				Eventually(func() error {
					return k8sClient.Get(ctx, types.NamespacedName{
						Namespace: namespaceName,
						Name:      "prefect-on-ephemeral",
					}, deployment)
				}).Should(Succeed())

				service = &corev1.Service{}
				Eventually(func() error {
					return k8sClient.Get(ctx, types.NamespacedName{
						Namespace: namespaceName,
						Name:      "prefect-on-ephemeral",
					}, service)
				}).Should(Succeed())
			})

			Describe("the PrefectServer", func() {
				It("should have the DeploymentReconciled condition", func() {
					condition := meta.FindStatusCondition(prefectserver.Status.Conditions, "DeploymentReconciled")
					Expect(condition).NotTo(BeNil())
					Expect(condition.Status).To(Equal(metav1.ConditionTrue))
					Expect(condition.Reason).To(Equal("DeploymentCreated"))
					Expect(condition.Message).To(Equal("Deployment was created"))
				})

				It("should have the ServiceReconciled condition", func() {
					condition := meta.FindStatusCondition(prefectserver.Status.Conditions, "ServiceReconciled")
					Expect(condition).NotTo(BeNil())
					Expect(condition.Status).To(Equal(metav1.ConditionTrue))
					Expect(condition.Reason).To(Equal("ServiceCreated"))
					Expect(condition.Message).To(Equal("Service was created"))
				})

				It("should have the PersistentVolumeClaimReconciled condition", func() {
					condition := meta.FindStatusCondition(prefectserver.Status.Conditions, "PersistentVolumeClaimReconciled")
					Expect(condition).NotTo(BeNil())
					Expect(condition.Status).To(Equal(metav1.ConditionTrue))
					Expect(condition.Reason).To(Equal("PersistentVolumeClaimNotRequired"))
					Expect(condition.Message).To(Equal("PersistentVolumeClaim is not required"))
				})

				It("should have the MigrationJobReconciled condition", func() {
					condition := meta.FindStatusCondition(prefectserver.Status.Conditions, "MigrationJobReconciled")
					Expect(condition).NotTo(BeNil())
					Expect(condition.Status).To(Equal(metav1.ConditionTrue))
					Expect(condition.Reason).To(Equal("MigrationJobNotRequired"))
					Expect(condition.Message).To(Equal("MigrationJob is not required"))
				})
			})

			Describe("the Deployment", func() {
				It("should use ephemeral storage", func() {
					Expect(deployment.Spec.Template.Spec.Volumes).To(ContainElement(
						corev1.Volume{
							Name: "prefect-data",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					))

					Expect(deployment.Spec.Template.Spec.Containers).To(HaveLen(1))
					container := deployment.Spec.Template.Spec.Containers[0]
					Expect(container.VolumeMounts).To(ContainElement(
						corev1.VolumeMount{
							Name:      "prefect-data",
							MountPath: "/var/lib/prefect/",
						},
					))
				})

				It("should have an environment pointing to the ephemeral database", func() {
					Expect(deployment.Spec.Template.Spec.Containers).To(HaveLen(1))
					container := deployment.Spec.Template.Spec.Containers[0]

					Expect(container.Env).To(ContainElements([]corev1.EnvVar{
						{Name: "PREFECT_API_DATABASE_DRIVER", Value: "sqlite+aiosqlite"},
						{Name: "PREFECT_API_DATABASE_NAME", Value: "/var/lib/prefect/prefect.db"},
						{Name: "PREFECT_API_DATABASE_MIGRATE_ON_START", Value: "True"},
					}))
				})
			})
		})

		Context("When updating an ephemeral server", func() {
			BeforeEach(func() {
				prefectserver = &prefectiov1.PrefectServer{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: namespaceName,
						Name:      "prefect-on-ephemeral",
					},
				}
				Expect(k8sClient.Create(ctx, prefectserver)).To(Succeed())

				controllerReconciler := &PrefectServerReconciler{
					Client: k8sClient,
					Scheme: k8sClient.Scheme(),
				}

				// Reconcile once to create the server
				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: name,
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(k8sClient.Get(ctx, name, prefectserver)).To(Succeed())

				prefectserver.Spec.Settings = []corev1.EnvVar{
					{Name: "PREFECT_SOME_SETTING", Value: "some-value"},
				}
				Expect(k8sClient.Update(ctx, prefectserver)).To(Succeed())

				// Reconcile again to update the server
				_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: name,
				})
				Expect(err).NotTo(HaveOccurred())
			})

			It("should update the Deployment with the new setting", func() {
				deployment := &appsv1.Deployment{}
				Eventually(func() error {
					return k8sClient.Get(ctx, types.NamespacedName{
						Namespace: namespaceName,
						Name:      "prefect-on-ephemeral",
					}, deployment)
				}).Should(Succeed())

				Expect(deployment.Spec.Template.Spec.Containers).To(HaveLen(1))
				container := deployment.Spec.Template.Spec.Containers[0]
				Expect(container.Env).To(ContainElement(corev1.EnvVar{
					Name:  "PREFECT_SOME_SETTING",
					Value: "some-value",
				}))
			})
		})

		Context("When evaluating changes with an ephemeral server", func() {
			BeforeEach(func() {
				name = types.NamespacedName{
					Namespace: namespaceName,
					Name:      "prefect-on-ephemeral-no-changes",
				}

				prefectserver = &prefectiov1.PrefectServer{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: namespaceName,
						Name:      "prefect-on-ephemeral-no-changes",
					},
				}
				Expect(k8sClient.Create(ctx, prefectserver)).To(Succeed())

				controllerReconciler := &PrefectServerReconciler{
					Client: k8sClient,
					Scheme: k8sClient.Scheme(),
				}

				// Reconcile once to create the server
				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: name,
				})
				Expect(err).NotTo(HaveOccurred())
			})

			It("should not change a deployment if nothing has changed", func() {
				before := &appsv1.Deployment{}
				Expect(k8sClient.Get(ctx, types.NamespacedName{
					Namespace: namespaceName,
					Name:      "prefect-on-ephemeral-no-changes",
				}, before)).To(Succeed())

				controllerReconciler := &PrefectServerReconciler{
					Client: k8sClient,
					Scheme: k8sClient.Scheme(),
				}

				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: name,
				})
				Expect(err).NotTo(HaveOccurred())

				after := &appsv1.Deployment{}
				Expect(k8sClient.Get(ctx, types.NamespacedName{
					Namespace: namespaceName,
					Name:      "prefect-on-ephemeral-no-changes",
				}, after)).To(Succeed())

				Expect(after.Generation).To(Equal(before.Generation))
				Expect(after).To(Equal(before))
			})
		})
	})

	Context("for SQLite servers", func() {
		BeforeEach(func() {
			ctx = context.Background()
			namespaceName = fmt.Sprintf("sqlite-ns-%d", time.Now().UnixNano())

			namespace = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: namespaceName},
			}
			Expect(k8sClient.Create(ctx, namespace)).To(Succeed())

			name = types.NamespacedName{
				Namespace: namespaceName,
				Name:      "prefect-on-sqlite",
			}
		})

		Context("When creating a server backed by SQLite", func() {
			var persistentVolumeClaim *corev1.PersistentVolumeClaim
			var deployment *appsv1.Deployment
			var service *corev1.Service

			BeforeEach(func() {
				prefectserver = &prefectiov1.PrefectServer{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: namespaceName,
						Name:      "prefect-on-sqlite",
					},
					Spec: prefectiov1.PrefectServerSpec{
						Tolerations: tolerations,
						SQLite: &prefectiov1.SQLiteConfiguration{
							StorageClassName: "standard",
							Size:             resource.MustParse("512Mi"),
						},
					},
				}
				Expect(k8sClient.Create(ctx, prefectserver)).To(Succeed())

				controllerReconciler := &PrefectServerReconciler{
					Client: k8sClient,
					Scheme: k8sClient.Scheme(),
				}

				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: name,
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(k8sClient.Get(ctx, name, prefectserver)).To(Succeed())

				persistentVolumeClaim = &corev1.PersistentVolumeClaim{}
				Eventually(func() error {
					return k8sClient.Get(ctx, types.NamespacedName{
						Namespace: namespaceName,
						Name:      "prefect-on-sqlite-data",
					}, persistentVolumeClaim)
				}).Should(Succeed())

				deployment = &appsv1.Deployment{}
				Eventually(func() error {
					return k8sClient.Get(ctx, types.NamespacedName{
						Namespace: namespaceName,
						Name:      "prefect-on-sqlite",
					}, deployment)
				}).Should(Succeed())

				service = &corev1.Service{}
				Eventually(func() error {
					return k8sClient.Get(ctx, types.NamespacedName{
						Namespace: namespaceName,
						Name:      "prefect-on-sqlite",
					}, service)
				}).Should(Succeed())
			})

			Describe("the PrefectServer", func() {
				It("should have the DeploymentReconciled condition", func() {
					condition := meta.FindStatusCondition(prefectserver.Status.Conditions, "DeploymentReconciled")
					Expect(condition).NotTo(BeNil())
					Expect(condition.Status).To(Equal(metav1.ConditionTrue))
					Expect(condition.Reason).To(Equal("DeploymentCreated"))
					Expect(condition.Message).To(Equal("Deployment was created"))
				})

				It("should have the ServiceReconciled condition", func() {
					condition := meta.FindStatusCondition(prefectserver.Status.Conditions, "ServiceReconciled")
					Expect(condition).NotTo(BeNil())
					Expect(condition.Status).To(Equal(metav1.ConditionTrue))
					Expect(condition.Reason).To(Equal("ServiceCreated"))
					Expect(condition.Message).To(Equal("Service was created"))
				})

				It("should have the PersistentVolumeClaimReconciled condition", func() {
					condition := meta.FindStatusCondition(prefectserver.Status.Conditions, "PersistentVolumeClaimReconciled")
					Expect(condition).NotTo(BeNil())
					Expect(condition.Status).To(Equal(metav1.ConditionTrue))
					Expect(condition.Reason).To(Equal("PersistentVolumeClaimCreated"))
					Expect(condition.Message).To(Equal("PersistentVolumeClaim was created"))
				})

				It("should have the MigrationJobReconciled condition", func() {
					condition := meta.FindStatusCondition(prefectserver.Status.Conditions, "MigrationJobReconciled")
					Expect(condition).NotTo(BeNil())
					Expect(condition.Status).To(Equal(metav1.ConditionTrue))
					Expect(condition.Reason).To(Equal("MigrationJobNotRequired"))
					Expect(condition.Message).To(Equal("MigrationJob is not required"))
				})
			})

			Describe("the Deployment", func() {
				It("should have the correct tolerations", func() {
					Expect(deployment.Spec.Template.Spec.Tolerations).To(Equal(tolerations))
				})

				It("should use persistent storage", func() {
					Expect(deployment.Spec.Template.Spec.Volumes).To(ContainElement(
						corev1.Volume{
							Name: "prefect-data",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: "prefect-on-sqlite-data",
								},
							},
						},
					))

					Expect(deployment.Spec.Template.Spec.Containers).To(HaveLen(1))
					container := deployment.Spec.Template.Spec.Containers[0]
					Expect(container.VolumeMounts).To(ContainElement(
						corev1.VolumeMount{
							Name:      "prefect-data",
							MountPath: "/var/lib/prefect/",
						},
					))
				})

				It("should have an environment pointing to the persistent database", func() {
					Expect(deployment.Spec.Template.Spec.Containers).To(HaveLen(1))
					container := deployment.Spec.Template.Spec.Containers[0]

					Expect(container.Env).To(ConsistOf([]corev1.EnvVar{
						{Name: "PREFECT_HOME", Value: "/var/lib/prefect/"},
						{Name: "PREFECT_API_DATABASE_DRIVER", Value: "sqlite+aiosqlite"},
						{Name: "PREFECT_API_DATABASE_NAME", Value: "/var/lib/prefect/prefect.db"},
						{Name: "PREFECT_API_DATABASE_MIGRATE_ON_START", Value: "True"},
					}))
				})
			})
		})

		Context("When updating a server backed by SQLite", func() {
			BeforeEach(func() {
				prefectserver = &prefectiov1.PrefectServer{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: namespaceName,
						Name:      "prefect-on-sqlite",
					},
					Spec: prefectiov1.PrefectServerSpec{
						SQLite: &prefectiov1.SQLiteConfiguration{
							StorageClassName: "standard",
							Size:             resource.MustParse("512Mi"),
						},
					},
				}
				Expect(k8sClient.Create(ctx, prefectserver)).To(Succeed())

				controllerReconciler := &PrefectServerReconciler{
					Client: k8sClient,
					Scheme: k8sClient.Scheme(),
				}

				// Reconcile once to create the server
				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: name,
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(k8sClient.Get(ctx, name, prefectserver)).To(Succeed())

				prefectserver.Spec.Settings = []corev1.EnvVar{
					{Name: "PREFECT_SOME_SETTING", Value: "some-value"},
				}
				Expect(k8sClient.Update(ctx, prefectserver)).To(Succeed())

				// Reconcile again to update the server
				_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: name,
				})
				Expect(err).NotTo(HaveOccurred())
			})

			It("should update the Deployment with the new setting", func() {
				deployment := &appsv1.Deployment{}
				Eventually(func() error {
					return k8sClient.Get(ctx, types.NamespacedName{
						Namespace: namespaceName,
						Name:      "prefect-on-sqlite",
					}, deployment)
				}).Should(Succeed())

				Expect(deployment.Spec.Template.Spec.Containers).To(HaveLen(1))
				container := deployment.Spec.Template.Spec.Containers[0]
				Expect(container.Env).To(ContainElement(corev1.EnvVar{
					Name:  "PREFECT_SOME_SETTING",
					Value: "some-value",
				}))
			})
		})

		Context("When evaluating changes with a SQLite server", func() {
			BeforeEach(func() {
				name = types.NamespacedName{
					Namespace: namespaceName,
					Name:      "prefect-on-sqlite-no-changes",
				}

				prefectserver = &prefectiov1.PrefectServer{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: namespaceName,
						Name:      "prefect-on-sqlite-no-changes",
					},
					Spec: prefectiov1.PrefectServerSpec{
						SQLite: &prefectiov1.SQLiteConfiguration{
							StorageClassName: "standard",
							Size:             resource.MustParse("512Mi"),
						},
					},
				}
				Expect(k8sClient.Create(ctx, prefectserver)).To(Succeed())

				controllerReconciler := &PrefectServerReconciler{
					Client: k8sClient,
					Scheme: k8sClient.Scheme(),
				}

				// Reconcile once to create the server
				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: name,
				})
				Expect(err).NotTo(HaveOccurred())
			})

			It("should not change a deployment if nothing has changed", func() {
				before := &appsv1.Deployment{}
				Expect(k8sClient.Get(ctx, types.NamespacedName{
					Namespace: namespaceName,
					Name:      "prefect-on-sqlite-no-changes",
				}, before)).To(Succeed())

				controllerReconciler := &PrefectServerReconciler{
					Client: k8sClient,
					Scheme: k8sClient.Scheme(),
				}

				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: name,
				})
				Expect(err).NotTo(HaveOccurred())

				after := &appsv1.Deployment{}
				Expect(k8sClient.Get(ctx, types.NamespacedName{
					Namespace: namespaceName,
					Name:      "prefect-on-sqlite-no-changes",
				}, after)).To(Succeed())

				Expect(after.Generation).To(Equal(before.Generation))
				Expect(after).To(Equal(before))
			})
		})
	})

	Context("for PostgreSQL servers", func() {
		BeforeEach(func() {
			ctx = context.Background()
			namespaceName = fmt.Sprintf("postgres-ns-%d", time.Now().UnixNano())

			namespace = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: namespaceName},
			}
			Expect(k8sClient.Create(ctx, namespace)).To(Succeed())

			name = types.NamespacedName{
				Namespace: namespaceName,
				Name:      "prefect-on-postgres",
			}
		})

		Context("When creating a server backed by PostgreSQL", func() {
			var deployment *appsv1.Deployment
			var migrateJob *batchv1.Job
			var service *corev1.Service

			BeforeEach(func() {
				prefectserver = &prefectiov1.PrefectServer{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: namespaceName,
						Name:      "prefect-on-postgres",
					},
					Spec: prefectiov1.PrefectServerSpec{
						Postgres: &prefectiov1.PostgresConfiguration{
							Host:     new("some-postgres-server"),
							Port:     new(15432),
							User:     new("a-prefect-user"),
							Password: new("this-is-a-bad-idea"),
							Database: new("some-prefect"),
						},
						DeploymentLabels: map[string]string{
							"some":    "additional-label",
							"another": "extra-label",
						},
						MigrationJobLabels: map[string]string{
							"some":    "additional-label-for-migrations",
							"another": "extra-label-for-migrations",
						},
						NodeSelector: map[string]string{
							"some": "node-selector",
						},
						Tolerations: tolerations,
						Affinity: &corev1.Affinity{
							NodeAffinity: &corev1.NodeAffinity{
								RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
									NodeSelectorTerms: []corev1.NodeSelectorTerm{{
										MatchExpressions: []corev1.NodeSelectorRequirement{
											{Key: "dedicated", Operator: corev1.NodeSelectorOpIn, Values: []string{"prefect"}},
										},
									}},
								},
							},
							PodAffinity: &corev1.PodAffinity{
								RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{{
									TopologyKey:   "kubernetes.io/hostname",
									LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "prefect-server"}},
								}},
							},
						},
					},
				}
				Expect(k8sClient.Create(ctx, prefectserver)).To(Succeed())

				controllerReconciler := &PrefectServerReconciler{
					Client: k8sClient,
					Scheme: k8sClient.Scheme(),
				}

				result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: name,
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(k8sClient.Get(ctx, name, prefectserver)).To(Succeed())

				By("waiting for the migration Job before creating the Deployment")
				Expect(result.RequeueAfter).To(Equal(migrationJobPollInterval))
				err = k8sClient.Get(ctx, types.NamespacedName{Namespace: namespaceName, Name: "prefect-on-postgres"}, &appsv1.Deployment{})
				Expect(apierrors.IsNotFound(err)).To(BeTrue(), "the Deployment should not exist yet, got %v", err)

				migrateAndReconcile(ctx, prefectserver)
				Expect(k8sClient.Get(ctx, name, prefectserver)).To(Succeed())

				_, _, desiredMigrationJob, _ := controllerReconciler.prefectServerDeployment(prefectserver)
				migrateJob = &batchv1.Job{}
				Eventually(func() error {
					return k8sClient.Get(ctx, types.NamespacedName{
						Namespace: namespaceName,
						Name:      desiredMigrationJob.Name,
					}, migrateJob)
				}).Should(Succeed())

				deployment = &appsv1.Deployment{}
				Eventually(func() error {
					return k8sClient.Get(ctx, types.NamespacedName{
						Namespace: namespaceName,
						Name:      "prefect-on-postgres",
					}, deployment)
				}).Should(Succeed())

				service = &corev1.Service{}
				Eventually(func() error {
					return k8sClient.Get(ctx, types.NamespacedName{
						Namespace: namespaceName,
						Name:      "prefect-on-postgres",
					}, service)
				}).Should(Succeed())
			})

			Describe("the PrefectServer", func() {
				It("should have the DeploymentReconciled condition", func() {
					condition := meta.FindStatusCondition(prefectserver.Status.Conditions, "DeploymentReconciled")
					Expect(condition).NotTo(BeNil())
					Expect(condition.Status).To(Equal(metav1.ConditionTrue))
					Expect(condition.Reason).To(Equal("DeploymentCreated"))
					Expect(condition.Message).To(Equal("Deployment was created"))
				})

				It("should have the ServiceReconciled condition", func() {
					condition := meta.FindStatusCondition(prefectserver.Status.Conditions, "ServiceReconciled")
					Expect(condition).NotTo(BeNil())
					Expect(condition.Status).To(Equal(metav1.ConditionTrue))
					Expect(condition.Reason).To(Equal("ServiceCreated"))
					Expect(condition.Message).To(Equal("Service was created"))
				})

				It("should have the PersistentVolumeClaimReconciled condition", func() {
					condition := meta.FindStatusCondition(prefectserver.Status.Conditions, "PersistentVolumeClaimReconciled")
					Expect(condition).NotTo(BeNil())
					Expect(condition.Status).To(Equal(metav1.ConditionTrue))
					Expect(condition.Reason).To(Equal("PersistentVolumeClaimNotRequired"))
					Expect(condition.Message).To(Equal("PersistentVolumeClaim is not required"))
				})

				It("should have the MigrationJobReconciled condition", func() {
					condition := meta.FindStatusCondition(prefectserver.Status.Conditions, "MigrationJobReconciled")
					Expect(condition).NotTo(BeNil())
					Expect(condition.Status).To(Equal(metav1.ConditionTrue))
					Expect(condition.Reason).To(Equal("MigrationJobCreated"))
					Expect(condition.Message).To(Equal("MigrationJob was created"))
				})

				DescribeTable("on a later reconcile, should go on only once the migration Job completes",
					func(status batchv1.JobStatus, done bool, reason string) {
						setMigrationJobStatus(ctx, prefectserver, status)

						reconciler := &PrefectServerReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
						result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: name})
						Expect(err).NotTo(HaveOccurred())

						Expect(k8sClient.Get(ctx, name, prefectserver)).To(Succeed())
						condition := meta.FindStatusCondition(prefectserver.Status.Conditions, "MigrationJobReconciled")
						Expect(condition).NotTo(BeNil())
						Expect(condition.Reason).To(Equal(reason))
						if done {
							Expect(result.RequeueAfter).To(BeZero())
							Expect(condition.Status).To(Equal(metav1.ConditionTrue))
						} else {
							Expect(result.RequeueAfter).To(Equal(migrationJobPollInterval))
							Expect(condition.Status).To(Equal(metav1.ConditionFalse))
						}
					},
					Entry("waits while it runs", migrationRunning, false, "MigrationJobAlreadyExists"),
					Entry("waits while it retries a failed pod", migrationRetrying, false, "MigrationJobAlreadyExists"),
					Entry("goes on once it completes", migrationComplete, true, "MigrationJobCreated"),
					Entry("stops once it fails", migrationFailed, false, "MigrationJobFailed"),
				)
			})

			Describe("the Deployment", func() {
				It("should have the correct tolerations", func() {
					Expect(deployment.Spec.Template.Spec.Tolerations).To(Equal(tolerations))
				})

				It("should not use persistent storage", func() {
					Expect(deployment.Spec.Template.Spec.Volumes).To(BeEmpty())

					Expect(deployment.Spec.Template.Spec.Containers).To(HaveLen(1))
					container := deployment.Spec.Template.Spec.Containers[0]
					Expect(container.VolumeMounts).To(BeEmpty())
				})

				It("should have an environment pointing to the PostgreSQL database", func() {
					Expect(deployment.Spec.Template.Spec.Containers).To(HaveLen(1))
					container := deployment.Spec.Template.Spec.Containers[0]

					Expect(container.Env).To(ConsistOf([]corev1.EnvVar{
						{Name: "PREFECT_HOME", Value: "/var/lib/prefect/"},
						{Name: "PREFECT_API_DATABASE_DRIVER", Value: "postgresql+asyncpg"},
						{Name: "PREFECT_API_DATABASE_HOST", Value: "some-postgres-server"},
						{Name: "PREFECT_API_DATABASE_PORT", Value: "15432"},
						{Name: "PREFECT_API_DATABASE_USER", Value: "a-prefect-user"},
						{Name: "PREFECT_API_DATABASE_PASSWORD", Value: "this-is-a-bad-idea"},
						{Name: "PREFECT_API_DATABASE_NAME", Value: "some-prefect"},
						{Name: "PREFECT_API_DATABASE_MIGRATE_ON_START", Value: "False"},
					}))
				})
			})

			Describe("the migration Job", func() {
				It("should be owned by the PrefectServer", func() {
					Expect(migrateJob.OwnerReferences).To(ContainElement(
						metav1.OwnerReference{
							APIVersion:         "prefect.io/v1",
							Kind:               "PrefectServer",
							Name:               "prefect-on-postgres",
							UID:                prefectserver.UID,
							Controller:         new(true),
							BlockOwnerDeletion: new(true),
						},
					))
				})

				It("should have the correct labels", func() {
					Expect(migrateJob.Labels).To(HaveKeyWithValue("prefect.io/server", "prefect-on-postgres"))
					Expect(migrateJob.Labels).To(HaveKeyWithValue("some", "additional-label-for-migrations"))
					Expect(migrateJob.Labels).To(HaveKeyWithValue("another", "extra-label-for-migrations"))
				})

				It("should have an environment pointing to the PostgreSQL database", func() {
					Expect(migrateJob.Spec.Template.Spec.Containers).To(HaveLen(1))
					container := migrateJob.Spec.Template.Spec.Containers[0]

					Expect(container.Env).To(ConsistOf([]corev1.EnvVar{
						{Name: "PREFECT_HOME", Value: "/var/lib/prefect/"},
						{Name: "PREFECT_API_DATABASE_DRIVER", Value: "postgresql+asyncpg"},
						{Name: "PREFECT_API_DATABASE_HOST", Value: "some-postgres-server"},
						{Name: "PREFECT_API_DATABASE_PORT", Value: "15432"},
						{Name: "PREFECT_API_DATABASE_USER", Value: "a-prefect-user"},
						{Name: "PREFECT_API_DATABASE_PASSWORD", Value: "this-is-a-bad-idea"},
						{Name: "PREFECT_API_DATABASE_NAME", Value: "some-prefect"},
						{Name: "PREFECT_API_DATABASE_MIGRATE_ON_START", Value: "False"},
					}))
				})

				It("should have the correct nodeSelector value", func() {
					Expect(migrateJob.Spec.Template.Spec.NodeSelector).To(HaveKeyWithValue("some", "node-selector"))
				})

				It("should have the correct tolerations", func() {
					Expect(migrateJob.Spec.Template.Spec.Tolerations).To(Equal(tolerations))
				})

				It("should only have the node affinity", func() {
					Expect(migrateJob.Spec.Template.Spec.Affinity).To(Equal(&corev1.Affinity{
						NodeAffinity: prefectserver.Spec.Affinity.NodeAffinity,
					}))
				})
			})
		})

		Context("When creating a server backed by PostgreSQL ConfigMaps and Secrets", func() {
			var deployment *appsv1.Deployment
			var migrateJob *batchv1.Job
			var service *corev1.Service

			BeforeEach(func() {
				prefectserver = &prefectiov1.PrefectServer{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: namespaceName,
						Name:      "prefect-on-postgres",
					},
					Spec: prefectiov1.PrefectServerSpec{
						Postgres: &prefectiov1.PostgresConfiguration{
							HostFrom: &corev1.EnvVarSource{
								ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{Name: "postgres-config"},
									Key:                  "host",
								},
							},
							PortFrom: &corev1.EnvVarSource{
								ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{Name: "postgres-config"},
									Key:                  "host",
								},
							},
							UserFrom: &corev1.EnvVarSource{
								SecretKeyRef: &corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{Name: "postgres-secret"},
									Key:                  "user",
								},
							},
							PasswordFrom: &corev1.EnvVarSource{
								SecretKeyRef: &corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{Name: "postgres-secret"},
									Key:                  "password",
								},
							},
							DatabaseFrom: &corev1.EnvVarSource{
								FieldRef: &corev1.ObjectFieldSelector{
									APIVersion: "v1",
									FieldPath:  "metadata.name",
								},
							},
						},
					},
				}
				Expect(k8sClient.Create(ctx, prefectserver)).To(Succeed())

				controllerReconciler := &PrefectServerReconciler{
					Client: k8sClient,
					Scheme: k8sClient.Scheme(),
				}

				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: name,
				})
				Expect(err).NotTo(HaveOccurred())
				migrateAndReconcile(ctx, prefectserver)

				_, _, desiredMigrationJob, _ := controllerReconciler.prefectServerDeployment(prefectserver)
				migrateJob = &batchv1.Job{}
				Eventually(func() error {
					return k8sClient.Get(ctx, types.NamespacedName{
						Namespace: namespaceName,
						Name:      desiredMigrationJob.Name,
					}, migrateJob)
				}).Should(Succeed())

				deployment = &appsv1.Deployment{}
				Eventually(func() error {
					return k8sClient.Get(ctx, types.NamespacedName{
						Namespace: namespaceName,
						Name:      "prefect-on-postgres",
					}, deployment)
				}).Should(Succeed())

				service = &corev1.Service{}
				Eventually(func() error {
					return k8sClient.Get(ctx, types.NamespacedName{
						Namespace: namespaceName,
						Name:      "prefect-on-postgres",
					}, service)
				}).Should(Succeed())
			})

			Describe("the Deployment", func() {
				It("should not use persistent storage", func() {
					Expect(deployment.Spec.Template.Spec.Volumes).To(BeEmpty())

					Expect(deployment.Spec.Template.Spec.Containers).To(HaveLen(1))
					container := deployment.Spec.Template.Spec.Containers[0]
					Expect(container.VolumeMounts).To(BeEmpty())
				})

				It("should have an environment pointing to the PostgreSQL database", func() {
					Expect(deployment.Spec.Template.Spec.Containers).To(HaveLen(1))
					container := deployment.Spec.Template.Spec.Containers[0]

					Expect(container.Env).To(ConsistOf([]corev1.EnvVar{
						{Name: "PREFECT_HOME", Value: "/var/lib/prefect/"},
						{Name: "PREFECT_API_DATABASE_DRIVER", Value: "postgresql+asyncpg"},
						{Name: "PREFECT_API_DATABASE_HOST", ValueFrom: &corev1.EnvVarSource{
							ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{Name: "postgres-config"},
								Key:                  "host",
							},
						}},
						{Name: "PREFECT_API_DATABASE_PORT", ValueFrom: &corev1.EnvVarSource{
							ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{Name: "postgres-config"},
								Key:                  "host",
							},
						}},
						{Name: "PREFECT_API_DATABASE_USER", ValueFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{Name: "postgres-secret"},
								Key:                  "user",
							},
						}},
						{Name: "PREFECT_API_DATABASE_PASSWORD", ValueFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{Name: "postgres-secret"},
								Key:                  "password",
							},
						}},
						{Name: "PREFECT_API_DATABASE_NAME", ValueFrom: &corev1.EnvVarSource{
							FieldRef: &corev1.ObjectFieldSelector{
								APIVersion: "v1",
								FieldPath:  "metadata.name",
							},
						}},
						{Name: "PREFECT_API_DATABASE_MIGRATE_ON_START", Value: "False"},
					}))
				})
			})
		})

		Context("When updating a server backed by PostgreSQL", func() {
			var controllerReconciler *PrefectServerReconciler
			newSetting := corev1.EnvVar{Name: "PREFECT_SOME_SETTING", Value: "some-value"}
			BeforeEach(func() {
				prefectserver = &prefectiov1.PrefectServer{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: namespaceName,
						Name:      "prefect-on-postgres",
					},
					Spec: prefectiov1.PrefectServerSpec{
						Postgres: &prefectiov1.PostgresConfiguration{
							Host:     new("some-postgres-server"),
							Port:     new(15432),
							User:     new("a-prefect-user"),
							Password: new("this-is-a-bad-idea"),
							Database: new("some-prefect"),
						},
					},
				}
				Expect(k8sClient.Create(ctx, prefectserver)).To(Succeed())

				controllerReconciler = &PrefectServerReconciler{
					Client: k8sClient,
					Scheme: k8sClient.Scheme(),
				}

				// Create the server, then change a setting. The new setting
				// needs a new migration Job before the Deployment can change.
				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: name})
				Expect(err).NotTo(HaveOccurred())
				Expect(k8sClient.Get(ctx, name, prefectserver)).To(Succeed())
				migrateAndReconcile(ctx, prefectserver)
				Expect(k8sClient.Get(ctx, name, prefectserver)).To(Succeed())

				prefectserver.Spec.Settings = []corev1.EnvVar{newSetting}
				Expect(k8sClient.Update(ctx, prefectserver)).To(Succeed())

				result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: name})
				Expect(err).NotTo(HaveOccurred())
				Expect(result.RequeueAfter).To(Equal(migrationJobPollInterval))
			})

			deploymentEnv := func() []corev1.EnvVar {
				GinkgoHelper()
				deployment := &appsv1.Deployment{}
				Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: namespaceName, Name: "prefect-on-postgres"}, deployment)).To(Succeed())
				Expect(deployment.Spec.Template.Spec.Containers).To(HaveLen(1))
				return deployment.Spec.Template.Spec.Containers[0].Env
			}

			It("should create new migration Job with the new setting", func() {
				_, _, desiredMigrationJob, _ := controllerReconciler.prefectServerDeployment(prefectserver)
				job := &batchv1.Job{}
				Eventually(func() error {
					return k8sClient.Get(ctx, types.NamespacedName{
						Namespace: namespaceName,
						Name:      desiredMigrationJob.Name,
					}, job)
				}).Should(Succeed())
				Expect(job.Spec.Template.Spec.Containers).To(HaveLen(1))
				container := job.Spec.Template.Spec.Containers[0]
				Expect(container.Env).To(ContainElement(newSetting))
			})

			It("should keep the Deployment as it was while the new migration Job runs", func() {
				Expect(deploymentEnv()).NotTo(ContainElement(newSetting))
			})

			It("should update the Deployment once the new migration Job completes", func() {
				migrateAndReconcile(ctx, prefectserver)
				Expect(deploymentEnv()).To(ContainElement(newSetting))
			})

			It("should keep the Deployment as it was when the new migration Job fails", func() {
				setMigrationJobStatus(ctx, prefectserver, migrationFailed)
				result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: name})
				Expect(err).NotTo(HaveOccurred())
				Expect(result.RequeueAfter).To(Equal(migrationJobPollInterval))
				Expect(deploymentEnv()).NotTo(ContainElement(newSetting))

				Expect(k8sClient.Get(ctx, name, prefectserver)).To(Succeed())
				condition := meta.FindStatusCondition(prefectserver.Status.Conditions, "MigrationJobReconciled")
				Expect(condition).NotTo(BeNil())
				Expect(condition.Status).To(Equal(metav1.ConditionFalse))
				Expect(condition.Reason).To(Equal("MigrationJobFailed"))
				Expect(condition.Message).To(ContainSubstring("Job has reached the specified backoff limit"))
			})

			It("should do nothing if an active migration Job already exists", func() {
				migrateJob := setMigrationJobStatus(ctx, prefectserver, migrationRunning)
				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: name})
				Expect(err).NotTo(HaveOccurred())

				after := &batchv1.Job{}
				Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(migrateJob), after)).To(Succeed())
				Expect(after.Generation).To(Equal(migrateJob.Generation))
			})
		})

		Context("When evaluating changes with a PostgreSQL server", func() {
			BeforeEach(func() {
				name = types.NamespacedName{
					Namespace: namespaceName,
					Name:      "prefect-on-postgres-no-changes",
				}

				prefectserver = &prefectiov1.PrefectServer{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: namespaceName,
						Name:      "prefect-on-postgres-no-changes",
					},
					Spec: prefectiov1.PrefectServerSpec{
						Postgres: &prefectiov1.PostgresConfiguration{
							Host:     new("some-postgres-server"),
							Port:     new(15432),
							User:     new("a-prefect-user"),
							Password: new("this-is-a-bad-idea"),
							Database: new("some-prefect"),
						},
					},
				}
				Expect(k8sClient.Create(ctx, prefectserver)).To(Succeed())

				controllerReconciler := &PrefectServerReconciler{
					Client: k8sClient,
					Scheme: k8sClient.Scheme(),
				}

				// Reconcile once to create the server
				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: name,
				})
				Expect(err).NotTo(HaveOccurred())
				migrateAndReconcile(ctx, prefectserver)
			})

			It("should not change a deployment if nothing has changed", func() {
				before := &appsv1.Deployment{}
				Expect(k8sClient.Get(ctx, types.NamespacedName{
					Namespace: namespaceName,
					Name:      "prefect-on-postgres-no-changes",
				}, before)).To(Succeed())

				controllerReconciler := &PrefectServerReconciler{
					Client: k8sClient,
					Scheme: k8sClient.Scheme(),
				}

				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: name,
				})
				Expect(err).NotTo(HaveOccurred())

				after := &appsv1.Deployment{}
				Expect(k8sClient.Get(ctx, types.NamespacedName{
					Namespace: namespaceName,
					Name:      "prefect-on-postgres-no-changes",
				}, after)).To(Succeed())

				Expect(after.Generation).To(Equal(before.Generation))
				Expect(after).To(Equal(before))
			})
		})
	})

	Context("PrefectServer Status Updates", func() {
		var (
			prefectServer *prefectiov1.PrefectServer
			deployment    *appsv1.Deployment
			reconciler    *PrefectServerReconciler
			name          types.NamespacedName
		)

		BeforeEach(func() {
			name = types.NamespacedName{
				Namespace: "test-" + uuid.New().String(),
				Name:      "test-prefectserver",
			}
			ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name.Namespace}}
			Expect(k8sClient.Create(ctx, ns)).To(Succeed())

			prefectServer = &prefectiov1.PrefectServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name.Name,
					Namespace: name.Namespace,
				},
				Spec: prefectiov1.PrefectServerSpec{},
			}
			Expect(k8sClient.Create(ctx, prefectServer)).To(Succeed())

			reconciler = &PrefectServerReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: name,
			})
			Expect(err).NotTo(HaveOccurred())

			deployment = &appsv1.Deployment{}
			Expect(k8sClient.Get(ctx, name, deployment)).To(Succeed())

			// Update the replicas to 1, which the Deployment controller would do
			deployment.Status.Replicas = 1
			Expect(k8sClient.Status().Update(ctx, deployment)).To(Succeed())
		})

		It("should have default values initially", func() {
			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: name})
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{}))

			updatedPrefectServer := &prefectiov1.PrefectServer{}
			Expect(k8sClient.Get(ctx, name, updatedPrefectServer)).To(Succeed())
			Expect(updatedPrefectServer.Status.Ready).To(Equal(false))
		})

		It("should update status when becoming ready", func() {
			deployment.Status.ReadyReplicas = 1
			Expect(k8sClient.Status().Update(ctx, deployment)).To(Succeed())

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: name})
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{}))

			updatedPrefectServer := &prefectiov1.PrefectServer{}
			Expect(k8sClient.Get(ctx, name, updatedPrefectServer)).To(Succeed())
			Expect(updatedPrefectServer.Status.Ready).To(Equal(true))
		})

		It("should update status when becoming unready", func() {
			deployment.Status.ReadyReplicas = 0
			Expect(k8sClient.Status().Update(ctx, deployment)).To(Succeed())

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: name})
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{}))

			updatedPrefectServer := &prefectiov1.PrefectServer{}
			Expect(k8sClient.Get(ctx, name, updatedPrefectServer)).To(Succeed())
			Expect(updatedPrefectServer.Status.Ready).To(Equal(false))
		})

		It("should toggle status correctly", func() {
			// First, make it ready
			deployment.Status.ReadyReplicas = 1
			Expect(k8sClient.Status().Update(ctx, deployment)).To(Succeed())
			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: name})
			Expect(err).NotTo(HaveOccurred())

			updatedPrefectServer := &prefectiov1.PrefectServer{}
			Expect(k8sClient.Get(ctx, name, updatedPrefectServer)).To(Succeed())
			Expect(updatedPrefectServer.Status.Ready).To(Equal(true))

			// Then, make it unready
			deployment.Status.ReadyReplicas = 0
			Expect(k8sClient.Status().Update(ctx, deployment)).To(Succeed())
			_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: name})
			Expect(err).NotTo(HaveOccurred())

			Expect(k8sClient.Get(ctx, name, updatedPrefectServer)).To(Succeed())
			Expect(updatedPrefectServer.Status.Ready).To(Equal(false))

			// Finally, make it ready again
			deployment.Status.ReadyReplicas = 1
			Expect(k8sClient.Status().Update(ctx, deployment)).To(Succeed())
			_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: name})
			Expect(err).NotTo(HaveOccurred())

			Expect(k8sClient.Get(ctx, name, updatedPrefectServer)).To(Succeed())
			Expect(updatedPrefectServer.Status.Ready).To(Equal(true))
		})
	})

	Context("error handling scenarios", func() {
		var (
			ctx           context.Context
			namespace     *corev1.Namespace
			namespaceName string
			name          types.NamespacedName
			prefectserver *prefectiov1.PrefectServer
		)

		BeforeEach(func() {
			ctx = context.Background()
			namespaceName = fmt.Sprintf("error-ns-%d", time.Now().UnixNano())

			namespace = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: namespaceName},
			}
			Expect(k8sClient.Create(ctx, namespace)).To(Succeed())

			name = types.NamespacedName{
				Namespace: namespaceName,
				Name:      "prefect-server-test",
			}

			prefectserver = &prefectiov1.PrefectServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name.Name,
					Namespace: namespaceName,
				},
				Spec: prefectiov1.PrefectServerSpec{
					Image: new("prefecthq/prefect:2.11.0-python3.11"),
				},
			}
		})

		AfterEach(func() {
			Expect(k8sClient.Delete(ctx, namespace)).To(Succeed())
		})

		It("should handle SetControllerReference errors when creating deployment specs", func() {
			// Create a server without a proper scheme (simulates SetControllerReference error)
			badReconciler := &PrefectServerReconciler{
				Client: k8sClient,
				Scheme: nil, // This will cause SetControllerReference to fail
			}

			// This test verifies that when the helper functions are called with an invalid scheme,
			// errors are now properly handled and returned
			_, _, _, err := badReconciler.prefectServerDeployment(prefectserver)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("scheme is nil, cannot set controller reference"))

			// Test service creation with invalid scheme
			_, err = badReconciler.prefectServerService(prefectserver)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("scheme is nil, cannot set controller reference"))
		})

		It("should handle SetControllerReference errors in PostgreSQL configurations", func() {
			prefectserver.Spec.Postgres = &prefectiov1.PostgresConfiguration{
				Host:     new("localhost"),
				Port:     new(5432),
				User:     new("prefect"),
				Password: new("password"),
				Database: new("prefect"),
			}

			badReconciler := &PrefectServerReconciler{
				Client: k8sClient,
				Scheme: nil, // This will cause SetControllerReference to fail
			}

			_, _, _, err := badReconciler.prefectServerDeployment(prefectserver)

			// Verify that SetControllerReference error is now properly handled
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("scheme is nil, cannot set controller reference"))
		})

		It("should handle SetControllerReference errors in SQLite configurations", func() {
			prefectserver.Spec.SQLite = &prefectiov1.SQLiteConfiguration{
				StorageClassName: "standard",
				Size:             resource.MustParse("1Gi"),
			}

			badReconciler := &PrefectServerReconciler{
				Client: k8sClient,
				Scheme: nil, // This will cause SetControllerReference to fail
			}

			_, _, _, err := badReconciler.prefectServerDeployment(prefectserver)

			// Verify that SetControllerReference error is now properly handled
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("scheme is nil, cannot set controller reference"))
		})

		It("should demonstrate proper error propagation from helper functions", func() {
			// Create server and attempt reconciliation
			Expect(k8sClient.Create(ctx, prefectserver)).To(Succeed())

			// Create a reconciler with invalid scheme to trigger SetControllerReference errors
			badReconciler := &PrefectServerReconciler{
				Client: k8sClient,
				Scheme: nil,
			}

			// Now this reconciliation should fail early when the helper functions encounter
			// SetControllerReference errors, rather than panicking later
			_, err := badReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: name})

			// The error should be properly propagated from the helper functions
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("scheme is nil, cannot set controller reference"))
		})
	})
})

// Migration Job statuses as the Job controller sets them. A failed pod alone
// leaves the Job running; only the Complete or Failed condition ends it.
var (
	migrationRunning  = batchv1.JobStatus{Active: 1}
	migrationRetrying = batchv1.JobStatus{Active: 1, Failed: 1}
	migrationComplete = batchv1.JobStatus{
		Succeeded:  1,
		Conditions: []batchv1.JobCondition{{Type: batchv1.JobComplete, Status: corev1.ConditionTrue}},
	}
	migrationFailed = batchv1.JobStatus{
		Failed: 7,
		Conditions: []batchv1.JobCondition{{
			Type:    batchv1.JobFailed,
			Status:  corev1.ConditionTrue,
			Reason:  "BackoffLimitExceeded",
			Message: "Job has reached the specified backoff limit",
		}},
	}
)

// setMigrationJobStatus sets the status of the server's current migration Job
// and returns the Job. Envtest runs no Job controller to do it.
func setMigrationJobStatus(ctx context.Context, server *prefectiov1.PrefectServer, status batchv1.JobStatus) *batchv1.Job {
	GinkgoHelper()
	job := &batchv1.Job{}
	key := types.NamespacedName{
		Namespace: server.Namespace,
		Name:      (&PrefectServerReconciler{}).postgresMigrationJob(server).Name,
	}
	Expect(k8sClient.Get(ctx, key, job)).To(Succeed())
	job.Status = *status.DeepCopy()
	Expect(k8sClient.Status().Update(ctx, job)).To(Succeed())
	return job
}

// migrateAndReconcile completes the server's current migration Job and
// reconciles, which rolls out the server.
func migrateAndReconcile(ctx context.Context, server *prefectiov1.PrefectServer) {
	GinkgoHelper()
	setMigrationJobStatus(ctx, server, migrationComplete)
	reconciler := &PrefectServerReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
	result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(server)})
	Expect(err).NotTo(HaveOccurred())
	Expect(result.RequeueAfter).To(BeZero(), "the server should not be waiting on its migration")
}
