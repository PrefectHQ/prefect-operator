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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	prefectiov1 "github.com/PrefectHQ/prefect-operator/api/v1"
)

var _ = Describe("PrefectWorkPool Controller", func() {
	var (
		ctx             context.Context
		namespace       *corev1.Namespace
		namespaceName   string
		name            types.NamespacedName
		prefectworkpool *prefectiov1.PrefectWorkPool
	)

	BeforeEach(func() {
		ctx = context.Background()
		namespaceName = fmt.Sprintf("any-ns-%d", time.Now().UnixNano())

		namespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: namespaceName},
		}
		Expect(k8sClient.Create(ctx, namespace)).To(Succeed())
	})

	It("should ignore removed PrefectWorkPools", func() {
		serverList := &prefectiov1.PrefectWorkPoolList{}
		err := k8sClient.List(ctx, serverList, &client.ListOptions{Namespace: namespaceName})
		Expect(err).NotTo(HaveOccurred())
		Expect(serverList.Items).To(HaveLen(0))

		controllerReconciler := &PrefectWorkPoolReconciler{
			Client: k8sClient,
			Scheme: k8sClient.Scheme(),
		}

		_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: namespaceName,
				Name:      "nonexistant-work-pool",
			},
		})
		Expect(err).NotTo(HaveOccurred())
	})

	It("should allow specifying a full image name", func() {
		prefectworkpool = &prefectiov1.PrefectWorkPool{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespaceName,
				Name:      "example-work-pool",
			},
			Spec: prefectiov1.PrefectWorkPoolSpec{
				Image: ptr.To("prefecthq/prefect:custom-prefect-image"),
			},
		}
		Expect(k8sClient.Create(ctx, prefectworkpool)).To(Succeed())

		controllerReconciler := &PrefectWorkPoolReconciler{
			Client: k8sClient,
			Scheme: k8sClient.Scheme(),
		}

		_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: namespaceName,
				Name:      "example-work-pool",
			},
		})
		Expect(err).NotTo(HaveOccurred())

		deployment := &appsv1.Deployment{}
		Eventually(func() error {
			return k8sClient.Get(ctx, types.NamespacedName{
				Namespace: namespaceName,
				Name:      "example-work-pool",
			}, deployment)
		}).Should(Succeed())

		Expect(deployment.Spec.Template.Spec.Containers).To(HaveLen(1))
		container := deployment.Spec.Template.Spec.Containers[0]
		Expect(container.Image).To(Equal("prefecthq/prefect:custom-prefect-image"))
	})

	It("should allow specifying a Prefect version", func() {
		prefectworkpool = &prefectiov1.PrefectWorkPool{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespaceName,
				Name:      "example-work-pool",
			},
			Spec: prefectiov1.PrefectWorkPoolSpec{
				Version: ptr.To("3.3.3.3.3.3.3.3"),
			},
		}
		Expect(k8sClient.Create(ctx, prefectworkpool)).To(Succeed())

		controllerReconciler := &PrefectWorkPoolReconciler{
			Client: k8sClient,
			Scheme: k8sClient.Scheme(),
		}

		_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: namespaceName,
				Name:      "example-work-pool",
			},
		})
		Expect(err).NotTo(HaveOccurred())

		deployment := &appsv1.Deployment{}
		Eventually(func() error {
			return k8sClient.Get(ctx, types.NamespacedName{
				Namespace: namespaceName,
				Name:      "example-work-pool",
			}, deployment)
		}).Should(Succeed())

		Expect(deployment.Spec.Template.Spec.Containers).To(HaveLen(1))
		container := deployment.Spec.Template.Spec.Containers[0]
		Expect(container.Image).To(Equal("prefecthq/prefect:3.3.3.3.3.3.3.3-python3.12"))
	})

	Context("when creating a work pool", func() {
		var deployment *appsv1.Deployment

		BeforeEach(func() {
			name = types.NamespacedName{
				Namespace: namespaceName,
				Name:      "example-work-pool",
			}

			prefectworkpool = &prefectiov1.PrefectWorkPool{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespaceName,
					Name:      "example-work-pool",
				},
				Spec: prefectiov1.PrefectWorkPoolSpec{
					Version: ptr.To("3.0.0rc15"),
					Type:    "kubernetes",
				},
			}
			Expect(k8sClient.Create(ctx, prefectworkpool)).To(Succeed())

			controllerReconciler := &PrefectWorkPoolReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: name,
			})
			Expect(err).NotTo(HaveOccurred())

			deployment = &appsv1.Deployment{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{
					Namespace: namespaceName,
					Name:      "example-work-pool",
				}, deployment)
			}).Should(Succeed())
		})

		Describe("the Deployment", func() {
			It("should be owned by the PrefectWorkPool", func() {
				Expect(deployment.OwnerReferences).To(ContainElement(
					metav1.OwnerReference{
						APIVersion:         "prefect.io/v1",
						Kind:               "PrefectWorkPool",
						Name:               "example-work-pool",
						UID:                prefectworkpool.UID,
						Controller:         ptr.To(true),
						BlockOwnerDeletion: ptr.To(true),
					},
				))
			})

			It("should have appropriate labels", func() {
				Expect(deployment.Spec.Selector.MatchLabels).To(Equal(map[string]string{
					"prefect.io/worker": "example-work-pool",
				}))
			})

			It("should have a worker container with the right image and command", func() {
				Expect(deployment.Spec.Template.Spec.Containers).To(HaveLen(1))
				container := deployment.Spec.Template.Spec.Containers[0]

				Expect(container.Name).To(Equal("prefect-worker"))
				Expect(container.Image).To(Equal("prefecthq/prefect:3.0.0rc15-python3.12-kubernetes"))
				Expect(container.Command).To(Equal([]string{"prefect", "worker", "start", "--pool", "example-work-pool", "--type", "kubernetes"}))
			})

			It("should have an environment with PREFECT_HOME set", func() {
				Expect(deployment.Spec.Template.Spec.Containers).To(HaveLen(1))
				container := deployment.Spec.Template.Spec.Containers[0]

				Expect(container.Env).To(ContainElements([]corev1.EnvVar{
					{Name: "PREFECT_HOME", Value: "/var/lib/prefect/"},
				}))
			})

			It("should not expose any ports", func() {
				Expect(deployment.Spec.Template.Spec.Containers).To(HaveLen(1))
				container := deployment.Spec.Template.Spec.Containers[0]

				Expect(container.Ports).To(BeEmpty())
			})
		})
	})

	Context("When updating a work pool", func() {
		BeforeEach(func() {
			name = types.NamespacedName{
				Namespace: namespaceName,
				Name:      "example-work-pool",
			}

			prefectworkpool = &prefectiov1.PrefectWorkPool{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespaceName,
					Name:      "example-work-pool",
				},
			}
			Expect(k8sClient.Create(ctx, prefectworkpool)).To(Succeed())

			controllerReconciler := &PrefectWorkPoolReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			// Reconcile once to create the work pool
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: name,
			})
			Expect(err).NotTo(HaveOccurred())

			prefectworkpool.Spec.Settings = []corev1.EnvVar{
				{Name: "PREFECT_SOME_SETTING", Value: "some-value"},
			}
			Expect(k8sClient.Update(ctx, prefectworkpool)).To(Succeed())

			// Reconcile again to update the work pool
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
					Name:      "example-work-pool",
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
				Name:      "example-work-pool",
			}, deployment)).To(Succeed())

			deployment.OwnerReferences = nil
			Expect(k8sClient.Update(ctx, deployment)).To(Succeed())

			controllerReconciler := &PrefectWorkPoolReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: name,
			})
			Expect(err).To(MatchError("Deployment already exists and is not controlled by PrefectWorkPool example-work-pool"))
		})
	})

	Context("When evaluating changes with a work pool", func() {
		BeforeEach(func() {
			name = types.NamespacedName{
				Namespace: namespaceName,
				Name:      "example-work-pool-no-changes",
			}

			prefectworkpool = &prefectiov1.PrefectWorkPool{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespaceName,
					Name:      "example-work-pool-no-changes",
				},
			}
			Expect(k8sClient.Create(ctx, prefectworkpool)).To(Succeed())

			controllerReconciler := &PrefectWorkPoolReconciler{
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
				Name:      "example-work-pool-no-changes",
			}, before)).To(Succeed())

			controllerReconciler := &PrefectWorkPoolReconciler{
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
				Name:      "example-work-pool-no-changes",
			}, after)).To(Succeed())

			Expect(after.Generation).To(Equal(before.Generation))
			Expect(after).To(Equal(before))
		})
	})
})
