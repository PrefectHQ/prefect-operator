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
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	prefectiov1 "github.com/PrefectHQ/prefect-operator/api/v1"
	"github.com/PrefectHQ/prefect-operator/internal/prefect"
)

var _ = Describe("PrefectWorkQueue controller", func() {
	var (
		ctx           context.Context
		namespace     *corev1.Namespace
		namespaceName string
		name          types.NamespacedName
		workQueue     *prefectiov1.PrefectWorkQueue
		reconciler    *PrefectWorkQueueReconciler
		mockClient    *prefect.MockClient
	)

	// syncedWorkQueue drives the reconciler through finalizer + first sync and
	// returns the freshly-read object.
	syncedWorkQueue := func() *prefectiov1.PrefectWorkQueue {
		Expect(k8sClient.Create(ctx, workQueue)).To(Succeed())

		_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: name})
		Expect(err).NotTo(HaveOccurred())

		_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: name})
		Expect(err).NotTo(HaveOccurred())

		fresh := &prefectiov1.PrefectWorkQueue{}
		Expect(k8sClient.Get(ctx, name, fresh)).To(Succeed())
		return fresh
	}

	BeforeEach(func() {
		ctx = context.Background()
		namespaceName = fmt.Sprintf("workqueue-ns-%d", time.Now().UnixNano())
		name = types.NamespacedName{Namespace: namespaceName, Name: "test-workqueue"}

		namespace = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespaceName}}
		Expect(k8sClient.Create(ctx, namespace)).To(Succeed())

		apiKeySecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "prefect-api-key", Namespace: namespaceName},
			Data:       map[string][]byte{"api-key": []byte("test-api-key-value")},
		}
		Expect(k8sClient.Create(ctx, apiKeySecret)).To(Succeed())

		workQueue = &prefectiov1.PrefectWorkQueue{
			ObjectMeta: metav1.ObjectMeta{Name: name.Name, Namespace: name.Namespace},
			Spec: prefectiov1.PrefectWorkQueueSpec{
				Server: prefectiov1.PrefectServerReference{
					RemoteAPIURL: new("https://api.prefect.cloud/api/accounts/abc/workspaces/def"),
					AccountID:    new("abc-123"),
					WorkspaceID:  new("def-456"),
					APIKey: &prefectiov1.APIKeySpec{
						ValueFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{Name: "prefect-api-key"},
								Key:                  "api-key",
							},
						},
					},
				},
				Name:             "ingest",
				WorkPoolName:     "example-pool",
				ConcurrencyLimit: new(int32(1)),
			},
		}

		mockClient = prefect.NewMockClient()
		// The pool-scoped queue routes 404 without the pool, so seed it.
		_, err := mockClient.CreateWorkPool(ctx, &prefect.WorkPoolSpec{Name: "example-pool", Type: "kubernetes"})
		Expect(err).NotTo(HaveOccurred())

		reconciler = &PrefectWorkQueueReconciler{
			Client:                k8sClient,
			Scheme:                k8sClient.Scheme(),
			PrefectClient:         mockClient,
			DefaultResyncInterval: testResyncInterval,
		}
	})

	AfterEach(func() {
		Expect(k8sClient.Delete(ctx, namespace)).To(Succeed())
	})

	It("should ignore removed PrefectWorkQueues", func() {
		result, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: types.NamespacedName{Namespace: namespaceName, Name: "nonexistent"},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(result.RequeueAfter).To(Equal(time.Duration(0)))
	})

	Context("When reconciling a new PrefectWorkQueue", func() {
		It("Should create the work queue successfully", func() {
			Expect(k8sClient.Create(ctx, workQueue)).To(Succeed())

			By("First reconciliation - adding finalizer")
			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: name})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(Equal(time.Second))

			Expect(k8sClient.Get(ctx, name, workQueue)).To(Succeed())
			Expect(workQueue.Finalizers).To(ContainElement(PrefectWorkQueueFinalizer))

			By("Second reconciliation - syncing with Prefect")
			result, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: name})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(BeJitteredResync(testResyncInterval))

			Expect(k8sClient.Get(ctx, name, workQueue)).To(Succeed())
			Expect(workQueue.Status.Id).NotTo(BeNil())
			_, parseErr := uuid.Parse(*workQueue.Status.Id)
			Expect(parseErr).NotTo(HaveOccurred())
			Expect(workQueue.Status.Ready).To(BeTrue())
			Expect(workQueue.Status.Adopted).To(HaveValue(BeFalse()))
			Expect(workQueue.Status.AppliedFields).To(ConsistOf("concurrencyLimit"))
			Expect(workQueue.Status.SpecHash).NotTo(BeEmpty())
			Expect(workQueue.Status.ObservedGeneration).To(Equal(workQueue.Generation))

			readyCondition := meta.FindStatusCondition(workQueue.Status.Conditions, PrefectWorkQueueConditionReady)
			Expect(readyCondition).NotTo(BeNil())
			Expect(readyCondition.Status).To(Equal(metav1.ConditionTrue))
			syncedCondition := meta.FindStatusCondition(workQueue.Status.Conditions, PrefectWorkQueueConditionSynced)
			Expect(syncedCondition).NotTo(BeNil())
			Expect(syncedCondition.Status).To(Equal(metav1.ConditionTrue))

			By("The queue exists in Prefect with the declared fields")
			remote, err := mockClient.GetWorkQueue(ctx, "example-pool", "ingest")
			Expect(err).NotTo(HaveOccurred())
			Expect(remote).NotTo(BeNil())
			Expect(remote.ConcurrencyLimit).To(HaveValue(Equal(int32(1))))
		})
	})

	Context("When the queue already exists in Prefect", func() {
		It("Should adopt it and patch only the drifting fields", func() {
			By("Seeding an implicitly-created queue (no concurrency limit)")
			existing, err := mockClient.CreateWorkQueue(ctx, "example-pool", &prefect.WorkQueueSpec{
				Name:        "ingest",
				Description: new("created by a deployment"),
			})
			Expect(err).NotTo(HaveOccurred())

			fresh := syncedWorkQueue()

			By("The CR adopted the existing queue rather than failing")
			Expect(fresh.Status.Id).To(HaveValue(Equal(existing.ID)))
			Expect(fresh.Status.Ready).To(BeTrue())
			Expect(fresh.Status.Adopted).To(HaveValue(BeTrue()))

			By("The declared concurrency limit was applied")
			Expect(mockClient.UpdateWorkQueueCalls).To(Equal(1))
			remote, err := mockClient.GetWorkQueue(ctx, "example-pool", "ingest")
			Expect(err).NotTo(HaveOccurred())
			Expect(remote.ConcurrencyLimit).To(HaveValue(Equal(int32(1))))

			By("The undeclared description was left alone")
			Expect(remote.Description).To(HaveValue(Equal("created by a deployment")))
		})
	})

	Context("When the work pool does not exist yet", func() {
		It("Should requeue with WorkPoolNotFound until the pool appears", func() {
			workQueue.Spec.WorkPoolName = "missing-pool"
			Expect(k8sClient.Create(ctx, workQueue)).To(Succeed())

			By("Finalizer pass")
			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: name})
			Expect(err).NotTo(HaveOccurred())

			By("Pool not found yet - requeue")
			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: name})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(Equal(5 * time.Second))

			Expect(k8sClient.Get(ctx, name, workQueue)).To(Succeed())
			Expect(workQueue.Status.Ready).To(BeFalse())
			cond := meta.FindStatusCondition(workQueue.Status.Conditions, PrefectWorkQueueConditionSynced)
			Expect(cond).NotTo(BeNil())
			Expect(cond.Reason).To(Equal("WorkPoolNotFound"))

			By("Seeding the pool into Prefect")
			_, err = mockClient.CreateWorkPool(ctx, &prefect.WorkPoolSpec{Name: "missing-pool", Type: "kubernetes"})
			Expect(err).NotTo(HaveOccurred())

			By("Now it syncs")
			result, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: name})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(BeJitteredResync(testResyncInterval))

			Expect(k8sClient.Get(ctx, name, workQueue)).To(Succeed())
			Expect(workQueue.Status.Ready).To(BeTrue())
		})
	})

	Context("When the remote queue already matches", func() {
		It("Should not PATCH on drift-detection resyncs", func() {
			fresh := syncedWorkQueue()
			Expect(mockClient.UpdateWorkQueueCalls).To(Equal(0))

			By("Aging LastSyncTime past the resync interval")
			aged := metav1.NewTime(time.Now().Add(-2 * testResyncInterval))
			fresh.Status.LastSyncTime = &aged
			Expect(k8sClient.Status().Update(ctx, fresh)).To(Succeed())

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: name})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(BeJitteredResync(testResyncInterval))

			By("The resync re-checked Prefect but did not PATCH")
			Expect(mockClient.UpdateWorkQueueCalls).To(Equal(0))
		})
	})

	Context("When the spec changes", func() {
		It("Should detect the change and re-sync, keeping the same ID", func() {
			fresh := syncedWorkQueue()
			initialID := *fresh.Status.Id

			fresh.Spec.ConcurrencyLimit = new(int32(4))
			Expect(k8sClient.Update(ctx, fresh)).To(Succeed())

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: name})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(BeJitteredResync(testResyncInterval))

			Expect(k8sClient.Get(ctx, name, fresh)).To(Succeed())
			Expect(fresh.Status.Ready).To(BeTrue())
			Expect(*fresh.Status.Id).To(Equal(initialID))

			remote, err := mockClient.GetWorkQueue(ctx, "example-pool", "ingest")
			Expect(err).NotTo(HaveOccurred())
			Expect(remote.ConcurrencyLimit).To(HaveValue(Equal(int32(4))))
		})
	})

	Context("When a declared field is removed from the spec", func() {
		It("Should reset it to its create-time default in Prefect", func() {
			workQueue.Spec.IsPaused = new(true)
			fresh := syncedWorkQueue()
			Expect(fresh.Status.AppliedFields).To(ConsistOf("concurrencyLimit", "isPaused"))

			remote, err := mockClient.GetWorkQueue(ctx, "example-pool", "ingest")
			Expect(err).NotTo(HaveOccurred())
			Expect(remote.ConcurrencyLimit).To(HaveValue(Equal(int32(1))))
			Expect(remote.IsPaused).To(HaveValue(BeTrue()))

			By("Removing concurrencyLimit and isPaused from the spec")
			fresh.Spec.ConcurrencyLimit = nil
			fresh.Spec.IsPaused = nil
			Expect(k8sClient.Update(ctx, fresh)).To(Succeed())

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: name})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(BeJitteredResync(testResyncInterval))

			By("The limit is cleared and the queue unpaused in Prefect")
			remote, err = mockClient.GetWorkQueue(ctx, "example-pool", "ingest")
			Expect(err).NotTo(HaveOccurred())
			Expect(remote.ConcurrencyLimit).To(BeNil())
			Expect(remote.IsPaused).To(HaveValue(BeFalse()))

			Expect(k8sClient.Get(ctx, name, fresh)).To(Succeed())
			Expect(fresh.Status.AppliedFields).To(BeEmpty())
		})
	})

	Context("When the work queue is deleted out-of-band in Prefect", func() {
		It("Should recreate it on the next sync", func() {
			fresh := syncedWorkQueue()
			originalID := *fresh.Status.Id

			By("Deleting the queue directly in Prefect")
			Expect(mockClient.DeleteWorkQueue(ctx, "example-pool", "ingest")).To(Succeed())

			By("Changing the spec to force a sync")
			fresh.Spec.ConcurrencyLimit = new(int32(2))
			Expect(k8sClient.Update(ctx, fresh)).To(Succeed())

			By("Reconcile finds nothing under the name and recreates")
			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: name})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(BeJitteredResync(testResyncInterval))

			Expect(k8sClient.Get(ctx, name, fresh)).To(Succeed())
			Expect(fresh.Status.Id).NotTo(BeNil())
			Expect(*fresh.Status.Id).NotTo(Equal(originalID))
			Expect(fresh.Status.Ready).To(BeTrue())

			remote, err := mockClient.GetWorkQueue(ctx, "example-pool", "ingest")
			Expect(err).NotTo(HaveOccurred())
			Expect(remote.ConcurrencyLimit).To(HaveValue(Equal(int32(2))))
		})
	})

	Context("When spec.name changes", func() {
		It("Should create the new queue and leave the old one untouched", func() {
			fresh := syncedWorkQueue()
			originalID := *fresh.Status.Id

			fresh.Spec.Name = "ingest-v2"
			Expect(k8sClient.Update(ctx, fresh)).To(Succeed())

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: name})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(BeJitteredResync(testResyncInterval))

			By("A queue exists under the new name")
			renamed, err := mockClient.GetWorkQueue(ctx, "example-pool", "ingest-v2")
			Expect(err).NotTo(HaveOccurred())
			Expect(renamed).NotTo(BeNil())
			Expect(renamed.ConcurrencyLimit).To(HaveValue(Equal(int32(1))))

			By("The old queue was not renamed, deleted, or modified")
			old, err := mockClient.GetWorkQueue(ctx, "example-pool", "ingest")
			Expect(err).NotTo(HaveOccurred())
			Expect(old).NotTo(BeNil())
			Expect(old.ID).To(Equal(originalID))

			Expect(k8sClient.Get(ctx, name, fresh)).To(Succeed())
			Expect(fresh.Status.Id).To(HaveValue(Equal(renamed.ID)))
			Expect(*fresh.Status.Id).NotTo(Equal(originalID))
		})
	})

	Context("When Prefect returns a sync error", func() {
		It("Should set SyncError and requeue on the error interval", func() {
			mockClient.ShouldFailCreate = true
			mockClient.FailureMessage = "simulated Prefect API error"

			Expect(k8sClient.Create(ctx, workQueue)).To(Succeed())

			By("Finalizer pass")
			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: name})
			Expect(err).NotTo(HaveOccurred())

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: name})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(Equal(RequeueIntervalError))

			Expect(k8sClient.Get(ctx, name, workQueue)).To(Succeed())
			Expect(workQueue.Status.Ready).To(BeFalse())
			cond := meta.FindStatusCondition(workQueue.Status.Conditions, PrefectWorkQueueConditionSynced)
			Expect(cond).NotTo(BeNil())
			Expect(cond.Reason).To(Equal("SyncError"))
		})
	})

	Context("When a PrefectWorkQueue that created its queue is deleted", func() {
		It("Should delete the queue in Prefect and remove the finalizer", func() {
			fresh := syncedWorkQueue()
			Expect(fresh.Status.Adopted).To(HaveValue(BeFalse()))

			Expect(k8sClient.Delete(ctx, fresh)).To(Succeed())

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: name})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(Equal(time.Duration(0)))

			By("The CR is gone")
			err = k8sClient.Get(ctx, name, &prefectiov1.PrefectWorkQueue{})
			Expect(err).To(HaveOccurred())

			By("The queue is gone from Prefect")
			remote, err := mockClient.GetWorkQueue(ctx, "example-pool", "ingest")
			Expect(err).NotTo(HaveOccurred())
			Expect(remote).To(BeNil())
		})
	})

	Context("When a PrefectWorkQueue that adopted its queue is deleted", func() {
		It("Should leave the queue in place in Prefect", func() {
			By("Seeding an existing queue so the CR adopts it")
			_, err := mockClient.CreateWorkQueue(ctx, "example-pool", &prefect.WorkQueueSpec{Name: "ingest"})
			Expect(err).NotTo(HaveOccurred())

			fresh := syncedWorkQueue()
			Expect(fresh.Status.Adopted).To(HaveValue(BeTrue()))

			Expect(k8sClient.Delete(ctx, fresh)).To(Succeed())

			_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: name})
			Expect(err).NotTo(HaveOccurred())

			By("The CR is gone")
			err = k8sClient.Get(ctx, name, &prefectiov1.PrefectWorkQueue{})
			Expect(err).To(HaveOccurred())

			By("The adopted queue survives in Prefect, limit intact")
			remote, err := mockClient.GetWorkQueue(ctx, "example-pool", "ingest")
			Expect(err).NotTo(HaveOccurred())
			Expect(remote).NotTo(BeNil())
			Expect(remote.ConcurrencyLimit).To(HaveValue(Equal(int32(1))))
		})
	})

	Context("When workPoolName is changed", func() {
		It("Should be rejected as immutable", func() {
			fresh := syncedWorkQueue()

			fresh.Spec.WorkPoolName = "another-pool"
			err := k8sClient.Update(ctx, fresh)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("workPoolName is immutable"))
		})
	})
})
