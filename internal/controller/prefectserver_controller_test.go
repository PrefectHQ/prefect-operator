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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	prefectiov1 "github.com/PrefectHQ/prefect-operator/api/v1"
)

var _ = Describe("PrefectServer controller", func() {
	var (
		ctx           context.Context
		namespace     *corev1.Namespace
		namespaceName string
		name          types.NamespacedName
		prefectserver *prefectiov1.PrefectServer
	)

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
					Image: ptr.To("prefecthq/prefect:custom-prefect-image"),
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
			prefectserver = &prefectiov1.PrefectServer{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespaceName,
					Name:      "prefect-on-anything",
				},
				Spec: prefectiov1.PrefectServerSpec{
					Version: ptr.To("3.3.3.3.3.3.3.3"),
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
			Expect(container.Image).To(Equal("prefecthq/prefect:3.3.3.3.3.3.3.3-python3.12"))
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
							Controller:         ptr.To(true),
							BlockOwnerDeletion: ptr.To(true),
						},
					))
				})

				It("should have appropriate labels", func() {
					Expect(deployment.Spec.Selector.MatchLabels).To(Equal(map[string]string{
						"prefect.io/server": "prefect-on-anything",
					}))
				})

				It("should have a server container with the right image and command", func() {
					Expect(deployment.Spec.Template.Spec.Containers).To(HaveLen(1))
					container := deployment.Spec.Template.Spec.Containers[0]

					Expect(container.Name).To(Equal("prefect-server"))
					Expect(container.Image).To(Equal("prefecthq/prefect:3.0.0rc18-python3.12"))
					Expect(container.Command).To(Equal([]string{"prefect", "server", "start", "--host", "0.0.0.0"}))
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
			})

			Describe("the Service", func() {
				It("should be owned by the PrefectServer", func() {
					Expect(service.OwnerReferences).To(ContainElement(
						metav1.OwnerReference{
							APIVersion:         "prefect.io/v1",
							Kind:               "PrefectServer",
							Name:               "prefect-on-anything",
							UID:                prefectserver.UID,
							Controller:         ptr.To(true),
							BlockOwnerDeletion: ptr.To(true),
						},
					))
				})

				It("should have matching labels", func() {
					Expect(service.Spec.Selector).To(Equal(map[string]string{
						"prefect.io/server": "prefect-on-anything",
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

			It("should not attempt to update a Deployment that it does not own", func() {
				deployment := &appsv1.Deployment{}
				Expect(k8sClient.Get(ctx, types.NamespacedName{
					Namespace: namespaceName,
					Name:      "prefect-on-anything",
				}, deployment)).To(Succeed())

				deployment.OwnerReferences = nil
				Expect(k8sClient.Update(ctx, deployment)).To(Succeed())

				controllerReconciler := &PrefectServerReconciler{
					Client: k8sClient,
					Scheme: k8sClient.Scheme(),
				}

				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: name,
				})
				Expect(err).To(MatchError("Deployment prefect-on-anything already exists and is not controlled by PrefectServer prefect-on-anything"))
			})

			It("should not attempt to update a service that it does not own", func() {
				service := &corev1.Service{}
				Expect(k8sClient.Get(ctx, types.NamespacedName{
					Namespace: namespaceName,
					Name:      "prefect-on-anything",
				}, service)).To(Succeed())

				service.OwnerReferences = nil
				Expect(k8sClient.Update(ctx, service)).To(Succeed())

				controllerReconciler := &PrefectServerReconciler{
					Client: k8sClient,
					Scheme: k8sClient.Scheme(),
				}

				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: name,
				})
				Expect(err).To(MatchError("Service prefect-on-anything already exists and is not controlled by PrefectServer prefect-on-anything"))
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

			It("should not attempt to update a Deployment that it does not own", func() {
				deployment := &appsv1.Deployment{}
				Expect(k8sClient.Get(ctx, types.NamespacedName{
					Namespace: namespaceName,
					Name:      "prefect-on-ephemeral",
				}, deployment)).To(Succeed())

				deployment.OwnerReferences = nil
				Expect(k8sClient.Update(ctx, deployment)).To(Succeed())

				controllerReconciler := &PrefectServerReconciler{
					Client: k8sClient,
					Scheme: k8sClient.Scheme(),
				}

				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: name,
				})
				Expect(err).To(MatchError("Deployment prefect-on-ephemeral already exists and is not controlled by PrefectServer prefect-on-ephemeral"))
			})

			It("should not attempt to update a service that it does not own", func() {
				service := &corev1.Service{}
				Expect(k8sClient.Get(ctx, types.NamespacedName{
					Namespace: namespaceName,
					Name:      "prefect-on-ephemeral",
				}, service)).To(Succeed())

				service.OwnerReferences = nil
				Expect(k8sClient.Update(ctx, service)).To(Succeed())

				controllerReconciler := &PrefectServerReconciler{
					Client: k8sClient,
					Scheme: k8sClient.Scheme(),
				}

				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: name,
				})
				Expect(err).To(MatchError("Service prefect-on-ephemeral already exists and is not controlled by PrefectServer prefect-on-ephemeral"))
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

			It("should not attempt to update a PersistentVolumeClaim that it does not own", func() {
				pvc := &corev1.PersistentVolumeClaim{}
				Expect(k8sClient.Get(ctx, types.NamespacedName{
					Namespace: namespaceName,
					Name:      "prefect-on-sqlite-data",
				}, pvc)).To(Succeed())

				pvc.OwnerReferences = nil
				Expect(k8sClient.Update(ctx, pvc)).To(Succeed())

				controllerReconciler := &PrefectServerReconciler{
					Client: k8sClient,
					Scheme: k8sClient.Scheme(),
				}

				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: name,
				})
				Expect(err).To(MatchError("PersistentVolumeClaim prefect-on-sqlite-data already exists and is not controlled by PrefectServer prefect-on-sqlite"))
			})

			It("should not attempt to update a Deployment that it does not own", func() {
				deployment := &appsv1.Deployment{}
				Expect(k8sClient.Get(ctx, types.NamespacedName{
					Namespace: namespaceName,
					Name:      "prefect-on-sqlite",
				}, deployment)).To(Succeed())

				deployment.OwnerReferences = nil
				Expect(k8sClient.Update(ctx, deployment)).To(Succeed())

				controllerReconciler := &PrefectServerReconciler{
					Client: k8sClient,
					Scheme: k8sClient.Scheme(),
				}

				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: name,
				})
				Expect(err).To(MatchError("Deployment prefect-on-sqlite already exists and is not controlled by PrefectServer prefect-on-sqlite"))
			})

			It("should not attempt to update a service that it does not own", func() {
				service := &corev1.Service{}
				Expect(k8sClient.Get(ctx, types.NamespacedName{
					Namespace: namespaceName,
					Name:      "prefect-on-sqlite",
				}, service)).To(Succeed())

				service.OwnerReferences = nil
				Expect(k8sClient.Update(ctx, service)).To(Succeed())

				controllerReconciler := &PrefectServerReconciler{
					Client: k8sClient,
					Scheme: k8sClient.Scheme(),
				}

				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: name,
				})
				Expect(err).To(MatchError("Service prefect-on-sqlite already exists and is not controlled by PrefectServer prefect-on-sqlite"))
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
							Host:     ptr.To("some-postgres-server"),
							Port:     ptr.To(15432),
							User:     ptr.To("a-prefect-user"),
							Password: ptr.To("this-is-a-bad-idea"),
							Database: ptr.To("some-prefect"),
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

				migrateJob = &batchv1.Job{}
				Eventually(func() error {
					return k8sClient.Get(ctx, types.NamespacedName{
						Namespace: namespaceName,
						Name:      "prefect-on-postgres-migration",
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
							Controller:         ptr.To(true),
							BlockOwnerDeletion: ptr.To(true),
						},
					))
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

				migrateJob = &batchv1.Job{}
				Eventually(func() error {
					return k8sClient.Get(ctx, types.NamespacedName{
						Namespace: namespaceName,
						Name:      "prefect-on-postgres-migration",
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
			BeforeEach(func() {
				prefectserver = &prefectiov1.PrefectServer{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: namespaceName,
						Name:      "prefect-on-postgres",
					},
					Spec: prefectiov1.PrefectServerSpec{
						Postgres: &prefectiov1.PostgresConfiguration{
							Host:     ptr.To("some-postgres-server"),
							Port:     ptr.To(15432),
							User:     ptr.To("a-prefect-user"),
							Password: ptr.To("this-is-a-bad-idea"),
							Database: ptr.To("some-prefect"),
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
						Name:      "prefect-on-postgres",
					}, deployment)
				}).Should(Succeed())

				Expect(deployment.Spec.Template.Spec.Containers).To(HaveLen(1))
				container := deployment.Spec.Template.Spec.Containers[0]
				Expect(container.Env).To(ContainElement(corev1.EnvVar{
					Name:  "PREFECT_SOME_SETTING",
					Value: "some-value",
				}))
			})

			It("should not attempt to update a migration Job that it does not own", func() {
				job := &batchv1.Job{}
				Expect(k8sClient.Get(ctx, types.NamespacedName{
					Namespace: namespaceName,
					Name:      "prefect-on-postgres-migration",
				}, job)).To(Succeed())

				job.OwnerReferences = nil
				Expect(k8sClient.Update(ctx, job)).To(Succeed())

				controllerReconciler := &PrefectServerReconciler{
					Client: k8sClient,
					Scheme: k8sClient.Scheme(),
				}

				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: name,
				})
				Expect(err).To(MatchError("Job prefect-on-postgres-migration already exists and is not controlled by PrefectServer prefect-on-postgres"))
			})

			It("should not attempt to update a Deployment that it does not own", func() {
				deployment := &appsv1.Deployment{}
				Expect(k8sClient.Get(ctx, types.NamespacedName{
					Namespace: namespaceName,
					Name:      "prefect-on-postgres",
				}, deployment)).To(Succeed())

				deployment.OwnerReferences = nil
				Expect(k8sClient.Update(ctx, deployment)).To(Succeed())

				controllerReconciler := &PrefectServerReconciler{
					Client: k8sClient,
					Scheme: k8sClient.Scheme(),
				}

				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: name,
				})
				Expect(err).To(MatchError("Deployment prefect-on-postgres already exists and is not controlled by PrefectServer prefect-on-postgres"))
			})

			It("should not attempt to update a service that it does not own", func() {
				service := &corev1.Service{}
				Expect(k8sClient.Get(ctx, types.NamespacedName{
					Namespace: namespaceName,
					Name:      "prefect-on-postgres",
				}, service)).To(Succeed())

				service.OwnerReferences = nil
				Expect(k8sClient.Update(ctx, service)).To(Succeed())

				controllerReconciler := &PrefectServerReconciler{
					Client: k8sClient,
					Scheme: k8sClient.Scheme(),
				}

				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: name,
				})
				Expect(err).To(MatchError("Service prefect-on-postgres already exists and is not controlled by PrefectServer prefect-on-postgres"))
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
							Host:     ptr.To("some-postgres-server"),
							Port:     ptr.To(15432),
							User:     ptr.To("a-prefect-user"),
							Password: ptr.To("this-is-a-bad-idea"),
							Database: ptr.To("some-prefect"),
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
})
