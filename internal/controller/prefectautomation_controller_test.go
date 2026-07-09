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

var _ = Describe("PrefectAutomation controller", func() {
	var (
		ctx           context.Context
		namespace     *corev1.Namespace
		namespaceName string
		name          types.NamespacedName
		automation    *prefectiov1.PrefectAutomation
		reconciler    *PrefectAutomationReconciler
		mockClient    *prefect.MockClient
	)

	// syncedAutomation drives the reconciler through finalizer + first sync and
	// returns the freshly-read object.
	syncedAutomation := func() *prefectiov1.PrefectAutomation {
		Expect(k8sClient.Create(ctx, automation)).To(Succeed())

		_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: name})
		Expect(err).NotTo(HaveOccurred())

		_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: name})
		Expect(err).NotTo(HaveOccurred())

		fresh := &prefectiov1.PrefectAutomation{}
		Expect(k8sClient.Get(ctx, name, fresh)).To(Succeed())
		return fresh
	}

	BeforeEach(func() {
		ctx = context.Background()
		namespaceName = fmt.Sprintf("automation-ns-%d", time.Now().UnixNano())
		name = types.NamespacedName{Namespace: namespaceName, Name: "test-automation"}

		namespace = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespaceName}}
		Expect(k8sClient.Create(ctx, namespace)).To(Succeed())

		apiKeySecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "prefect-api-key", Namespace: namespaceName},
			Data:       map[string][]byte{"api-key": []byte("test-api-key-value")},
		}
		Expect(k8sClient.Create(ctx, apiKeySecret)).To(Succeed())

		automation = &prefectiov1.PrefectAutomation{
			ObjectMeta: metav1.ObjectMeta{Name: name.Name, Namespace: name.Namespace},
			Spec: prefectiov1.PrefectAutomationSpec{
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
				Name: "my-automation",
				Trigger: prefectiov1.PrefectAutomationTrigger{
					Event: &prefectiov1.PrefectEventTrigger{Posture: "Reactive"},
				},
			},
		}

		mockClient = prefect.NewMockClient()
		reconciler = &PrefectAutomationReconciler{
			Client:                k8sClient,
			Scheme:                k8sClient.Scheme(),
			PrefectClient:         mockClient,
			DefaultResyncInterval: testResyncInterval,
		}
	})

	AfterEach(func() {
		Expect(k8sClient.Delete(ctx, namespace)).To(Succeed())
	})

	It("should ignore removed PrefectAutomations", func() {
		result, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: types.NamespacedName{Namespace: namespaceName, Name: "nonexistent"},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(result.RequeueAfter).To(Equal(time.Duration(0)))
	})

	Context("When reconciling a new PrefectAutomation", func() {
		It("Should create the automation successfully", func() {
			Expect(k8sClient.Create(ctx, automation)).To(Succeed())

			By("First reconciliation - adding finalizer")
			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: name})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(Equal(time.Second))

			Expect(k8sClient.Get(ctx, name, automation)).To(Succeed())
			Expect(automation.Finalizers).To(ContainElement(PrefectAutomationFinalizer))

			By("Second reconciliation - syncing with Prefect")
			result, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: name})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(BeJitteredResync(testResyncInterval))

			Expect(k8sClient.Get(ctx, name, automation)).To(Succeed())
			Expect(automation.Status.Id).NotTo(BeNil())
			_, parseErr := uuid.Parse(*automation.Status.Id)
			Expect(parseErr).NotTo(HaveOccurred())
			Expect(automation.Status.Ready).To(BeTrue())
			Expect(automation.Status.SpecHash).NotTo(BeEmpty())
			Expect(automation.Status.ObservedGeneration).To(Equal(automation.Generation))

			readyCondition := meta.FindStatusCondition(automation.Status.Conditions, PrefectAutomationConditionReady)
			Expect(readyCondition).NotTo(BeNil())
			Expect(readyCondition.Status).To(Equal(metav1.ConditionTrue))
			syncedCondition := meta.FindStatusCondition(automation.Status.Conditions, PrefectAutomationConditionSynced)
			Expect(syncedCondition).NotTo(BeNil())
			Expect(syncedCondition.Status).To(Equal(metav1.ConditionTrue))
		})
	})

	Context("When the spec is unchanged", func() {
		It("Should not re-sync within the resync interval", func() {
			fresh := syncedAutomation()
			initialHash := fresh.Status.SpecHash

			By("Confirming the sync stamped LastSyncTime")
			Expect(fresh.Status.LastSyncTime).NotTo(BeNil())
			initialSyncTime := fresh.Status.LastSyncTime.Time

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: name})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(BeJitteredResync(testResyncInterval))

			Expect(k8sClient.Get(ctx, name, fresh)).To(Succeed())
			Expect(fresh.Status.SpecHash).To(Equal(initialHash))
			// No re-sync happened, so LastSyncTime is unchanged.
			Expect(fresh.Status.LastSyncTime.Time).To(BeTemporally("~", initialSyncTime, time.Second))
		})
	})

	Context("When spec.interval is set", func() {
		It("Should requeue on the per-resource interval, not the default", func() {
			automation.Spec.Interval = &metav1.Duration{Duration: 90 * time.Second}
			fresh := syncedAutomation()

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: name})
			Expect(err).NotTo(HaveOccurred())
			// Requeue honors spec.interval (90s), not DefaultResyncInterval.
			Expect(result.RequeueAfter).To(BeJitteredResync(90 * time.Second))
			Expect(fresh.Status.Ready).To(BeTrue())
		})
	})

	Context("When the spec changes", func() {
		It("Should detect the change and re-sync", func() {
			fresh := syncedAutomation()
			initialID := *fresh.Status.Id

			fresh.Spec.Description = new("updated")
			Expect(k8sClient.Update(ctx, fresh)).To(Succeed())

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: name})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(BeJitteredResync(testResyncInterval))

			Expect(k8sClient.Get(ctx, name, fresh)).To(Succeed())
			Expect(fresh.Status.Ready).To(BeTrue())
			// Update path keeps the same ID.
			Expect(*fresh.Status.Id).To(Equal(initialID))
		})
	})

	Context("When the spec is invalid", func() {
		It("Should set InvalidSpec when no trigger is set", func() {
			automation.Spec.Trigger = prefectiov1.PrefectAutomationTrigger{}
			Expect(k8sClient.Create(ctx, automation)).To(Succeed())

			// Finalizer pass.
			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: name})
			Expect(err).NotTo(HaveOccurred())

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: name})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(Equal(time.Duration(0)))

			Expect(k8sClient.Get(ctx, name, automation)).To(Succeed())
			Expect(automation.Status.Ready).To(BeFalse())
			cond := meta.FindStatusCondition(automation.Status.Conditions, PrefectAutomationConditionSynced)
			Expect(cond).NotTo(BeNil())
			Expect(cond.Reason).To(Equal("InvalidSpec"))
		})

		It("Should set InvalidSpec when an action sets both deploymentId and deploymentName", func() {
			automation.Spec.Actions = []prefectiov1.PrefectAutomationAction{
				{
					Type:           "run-deployment",
					Source:         new("selected"),
					DeploymentID:   new("dep-id"),
					DeploymentName: new("dep-name"),
				},
			}
			Expect(k8sClient.Create(ctx, automation)).To(Succeed())

			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: name})
			Expect(err).NotTo(HaveOccurred())
			_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: name})
			Expect(err).NotTo(HaveOccurred())

			Expect(k8sClient.Get(ctx, name, automation)).To(Succeed())
			cond := meta.FindStatusCondition(automation.Status.Conditions, PrefectAutomationConditionSynced)
			Expect(cond).NotTo(BeNil())
			Expect(cond.Reason).To(Equal("InvalidSpec"))
		})
	})

	Context("When an action references a deployment by name", func() {
		BeforeEach(func() {
			automation.Spec.Actions = []prefectiov1.PrefectAutomationAction{
				{Type: "run-deployment", Source: new("selected"), DeploymentName: new("daily-etl")},
			}
		})

		It("Should requeue until the deployment exists, then sync", func() {
			Expect(k8sClient.Create(ctx, automation)).To(Succeed())

			By("Finalizer pass")
			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: name})
			Expect(err).NotTo(HaveOccurred())

			By("Deployment not found yet - requeue")
			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: name})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(Equal(5 * time.Second))

			Expect(k8sClient.Get(ctx, name, automation)).To(Succeed())
			Expect(automation.Status.Ready).To(BeFalse())
			cond := meta.FindStatusCondition(automation.Status.Conditions, PrefectAutomationConditionSynced)
			Expect(cond).NotTo(BeNil())
			Expect(cond.Reason).To(Equal("DeploymentNotFound"))

			By("Seeding the deployment into Prefect")
			_, err = mockClient.CreateOrUpdateDeployment(ctx, &prefect.DeploymentSpec{Name: "daily-etl", FlowID: "flow-1"})
			Expect(err).NotTo(HaveOccurred())

			By("Now it resolves and syncs")
			result, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: name})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(BeJitteredResync(testResyncInterval))

			Expect(k8sClient.Get(ctx, name, automation)).To(Succeed())
			Expect(automation.Status.Id).NotTo(BeNil())
			Expect(automation.Status.Ready).To(BeTrue())
		})
	})

	Context("When the automation is deleted out-of-band in Prefect", func() {
		It("Should clear the stale ID and recreate rather than loop on SyncError", func() {
			fresh := syncedAutomation()
			originalID := *fresh.Status.Id

			By("Deleting the automation directly in Prefect")
			Expect(mockClient.DeleteAutomation(ctx, originalID)).To(Succeed())

			By("Changing the spec to force a sync")
			fresh.Spec.Description = new("changed")
			Expect(k8sClient.Update(ctx, fresh)).To(Succeed())

			By("Reconcile detects the 404 and clears the ID")
			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: name})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(Equal(time.Second))

			Expect(k8sClient.Get(ctx, name, fresh)).To(Succeed())
			Expect(fresh.Status.Id).To(BeNil())

			By("Next reconcile recreates the automation")
			result, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: name})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(BeJitteredResync(testResyncInterval))

			Expect(k8sClient.Get(ctx, name, fresh)).To(Succeed())
			Expect(fresh.Status.Id).NotTo(BeNil())
			Expect(*fresh.Status.Id).NotTo(Equal(originalID))
			Expect(fresh.Status.Ready).To(BeTrue())
		})
	})

	Context("When Prefect returns a sync error", func() {
		It("Should set SyncError and requeue on the error interval", func() {
			mockClient.ShouldFailCreate = true
			mockClient.FailureMessage = "simulated Prefect API error"

			Expect(k8sClient.Create(ctx, automation)).To(Succeed())

			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: name})
			Expect(err).NotTo(HaveOccurred())

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: name})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(Equal(RequeueIntervalError))

			Expect(k8sClient.Get(ctx, name, automation)).To(Succeed())
			cond := meta.FindStatusCondition(automation.Status.Conditions, PrefectAutomationConditionSynced)
			Expect(cond).NotTo(BeNil())
			Expect(cond.Reason).To(Equal("SyncError"))
		})
	})

	Context("When the automation is deleted", func() {
		It("Should clean up from Prefect and remove the finalizer", func() {
			fresh := syncedAutomation()
			Expect(fresh.Status.Id).NotTo(BeNil())

			Expect(k8sClient.Delete(ctx, fresh)).To(Succeed())

			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: name})
			Expect(err).NotTo(HaveOccurred())

			err = k8sClient.Get(ctx, name, fresh)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not found"))
		})
	})
})
