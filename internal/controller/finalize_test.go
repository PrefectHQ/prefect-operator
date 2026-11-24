package controller

import (
	"context"
	"errors"
	"fmt"
	"time"

	prefectiov1 "github.com/PrefectHQ/prefect-operator/api/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var _ = Describe("finalize", func() {
	const (
		finalizer      = "prefect.io/test-cleanup"
		otherFinalizer = "example.com/other"
	)

	var (
		ctx      context.Context
		name     types.NamespacedName
		calls    int
		recorder *events.FakeRecorder
	)

	// deleting creates a ConfigMap with finalizers, deletes it so that it has
	// a deletion timestamp, and returns it as read back.
	deleting := func(finalizers ...string) *corev1.ConfigMap {
		obj := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
			Name:       name.Name,
			Namespace:  name.Namespace,
			Finalizers: finalizers,
		}}
		Expect(k8sClient.Create(ctx, obj)).To(Succeed())
		Expect(k8sClient.Delete(ctx, obj)).To(Succeed())
		Expect(k8sClient.Get(ctx, name, obj)).To(Succeed())
		Expect(obj.DeletionTimestamp).NotTo(BeNil())
		return obj
	}

	expectGone := func() {
		err := k8sClient.Get(ctx, name, &corev1.ConfigMap{})
		Expect(apierrors.IsNotFound(err)).To(BeTrue(), "expected the ConfigMap to be deleted, got %v", err)
	}

	BeforeEach(func() {
		ctx = context.Background()
		calls = 0
		recorder = events.NewFakeRecorder(10)
		namespaceName := fmt.Sprintf("finalize-ns-%d", time.Now().UnixNano())
		Expect(k8sClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespaceName}})).To(Succeed())
		name = types.NamespacedName{Namespace: namespaceName, Name: "finalize-test"}
	})

	It("deletes from Prefect and then removes the finalizer", func() {
		obj := deleting(finalizer)

		result, err := finalize(ctx, k8sClient, recorder, obj, finalizer, func(context.Context) error {
			calls++
			return nil
		})

		Expect(err).NotTo(HaveOccurred())
		Expect(result.IsZero()).To(BeTrue())
		Expect(calls).To(Equal(1))
		Expect(recorder.Events).NotTo(Receive())
		expectGone()
	})

	It("keeps the finalizer and retries when the Prefect delete fails", func() {
		obj := deleting(finalizer)

		result, err := finalize(ctx, k8sClient, recorder, obj, finalizer, func(context.Context) error {
			calls++
			return errors.New("prefect is down")
		})

		Expect(err).NotTo(HaveOccurred())
		Expect(result.RequeueAfter).To(BeAFirstRetry())
		Expect(calls).To(Equal(1))
		Expect(recorder.Events).To(Receive(And(
			HavePrefix("Warning DeleteFailed Failed to delete from Prefect, retrying until "),
			HaveSuffix(": prefect is down"))))
		Expect(k8sClient.Get(ctx, name, obj)).To(Succeed())
		Expect(obj.Finalizers).To(ConsistOf(finalizer))

		controllerutil.RemoveFinalizer(obj, finalizer)
		Expect(k8sClient.Update(ctx, obj)).To(Succeed())
	})

	It("removes the finalizer when the Prefect delete still fails after the retry window", func() {
		DeferCleanup(func(window time.Duration) { deleteRetryWindow = window }, deleteRetryWindow)
		deleteRetryWindow = 0
		obj := deleting(finalizer)

		_, err := finalize(ctx, k8sClient, recorder, obj, finalizer, func(context.Context) error {
			calls++
			return errors.New("prefect is down")
		})

		Expect(err).NotTo(HaveOccurred())
		Expect(calls).To(Equal(1))
		Expect(recorder.Events).To(Receive(Equal(
			"Warning DeleteFailed Failed to delete from Prefect, so it may still exist there: prefect is down")))
		expectGone()
	})

	It("removes the finalizer when there's nothing to delete in Prefect", func() {
		obj := deleting(finalizer)

		_, err := finalize(ctx, k8sClient, recorder, obj, finalizer, nil)

		Expect(err).NotTo(HaveOccurred())
		expectGone()
	})

	It("does nothing when the object doesn't have the finalizer", func() {
		obj := deleting(otherFinalizer)

		_, err := finalize(ctx, k8sClient, recorder, obj, finalizer, func(context.Context) error {
			calls++
			return nil
		})

		Expect(err).NotTo(HaveOccurred())
		Expect(calls).To(BeZero())
		Expect(k8sClient.Get(ctx, name, obj)).To(Succeed())
		Expect(obj.Finalizers).To(ConsistOf(otherFinalizer))

		controllerutil.RemoveFinalizer(obj, otherFinalizer)
		Expect(k8sClient.Update(ctx, obj)).To(Succeed())
	})

	It("returns an error when it can't remove the finalizer", func() {
		obj := deleting(finalizer)
		stale := obj.DeepCopy()

		// Change the object so that an update from the stale copy conflicts.
		obj.Labels = map[string]string{"changed": "true"}
		Expect(k8sClient.Update(ctx, obj)).To(Succeed())

		_, err := finalize(ctx, k8sClient, recorder, stale, finalizer, nil)

		Expect(apierrors.IsConflict(err)).To(BeTrue(), "expected a conflict, got %v", err)
		Expect(k8sClient.Get(ctx, name, obj)).To(Succeed())
		Expect(obj.Finalizers).To(ConsistOf(finalizer))

		controllerutil.RemoveFinalizer(obj, finalizer)
		Expect(k8sClient.Update(ctx, obj)).To(Succeed())
	})

	Context("in a terminating namespace", func() {
		var namespace *corev1.Namespace

		// queueDeleting creates a PrefectWorkQueue with server and the
		// finalizer, deletes its namespace and then the queue, and returns
		// the queue as read back.
		queueDeleting := func(server prefectiov1.PrefectServerReference) *prefectiov1.PrefectWorkQueue {
			queue := &prefectiov1.PrefectWorkQueue{
				ObjectMeta: metav1.ObjectMeta{Name: name.Name, Namespace: name.Namespace, Finalizers: []string{finalizer}},
				Spec:       prefectiov1.PrefectWorkQueueSpec{Server: server, Name: "default", WorkPoolName: "pool"},
			}
			Expect(k8sClient.Create(ctx, queue)).To(Succeed())
			Expect(k8sClient.Delete(ctx, namespace)).To(Succeed())
			Expect(k8sClient.Delete(ctx, queue)).To(Succeed())
			Expect(k8sClient.Get(ctx, name, queue)).To(Succeed())
			return queue
		}

		failingDelete := func(context.Context) error {
			calls++
			return errors.New("no such host")
		}

		expectQueueGone := func() {
			err := k8sClient.Get(ctx, name, &prefectiov1.PrefectWorkQueue{})
			Expect(apierrors.IsNotFound(err)).To(BeTrue(), "expected the PrefectWorkQueue to be deleted, got %v", err)
		}

		secretRef := func(secretName string) *corev1.SecretKeySelector {
			return &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: secretName}, Key: "key"}
		}

		BeforeEach(func() {
			namespace = &corev1.Namespace{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name.Namespace}, namespace)).To(Succeed())
		})

		It("removes the finalizer after one failed delete when the server's Service is gone", func() {
			queue := queueDeleting(prefectiov1.PrefectServerReference{Name: "prefect"})

			result, err := finalize(ctx, k8sClient, recorder, queue, finalizer, failingDelete)

			Expect(err).NotTo(HaveOccurred())
			Expect(result.IsZero()).To(BeTrue())
			Expect(calls).To(Equal(1))
			Expect(recorder.Events).To(Receive(Equal(fmt.Sprintf(
				"Warning DeleteFailed Failed to delete from Prefect, and Service %s/prefect was deleted with its namespace, "+
					"so it may still exist there: no such host", name.Namespace))))
			expectQueueGone()
		})

		It("removes the finalizer after one failed delete when the API key Secret is gone", func() {
			queue := queueDeleting(prefectiov1.PrefectServerReference{
				RemoteAPIURL: new("https://prefect.example.com"),
				APIKey:       &prefectiov1.APIKeySpec{ValueFrom: &corev1.EnvVarSource{SecretKeyRef: secretRef("api-key")}},
			})

			_, err := finalize(ctx, k8sClient, recorder, queue, finalizer, failingDelete)

			Expect(err).NotTo(HaveOccurred())
			Expect(recorder.Events).To(Receive(ContainSubstring(
				fmt.Sprintf("and Secret %s/api-key was deleted with its namespace", name.Namespace))))
			expectQueueGone()
		})

		It("removes the finalizer after one failed delete when the auth Secret is gone", func() {
			queue := queueDeleting(prefectiov1.PrefectServerReference{
				RemoteAPIURL: new("https://prefect.example.com"),
				Auth:         &prefectiov1.ServerAuth{SecretKeyRef: *secretRef("auth")},
			})

			_, err := finalize(ctx, k8sClient, recorder, queue, finalizer, failingDelete)

			Expect(err).NotTo(HaveOccurred())
			Expect(recorder.Events).To(Receive(ContainSubstring(
				fmt.Sprintf("and Secret %s/auth was deleted with its namespace", name.Namespace))))
			expectQueueGone()
		})

		It("keeps retrying while the Secret and Service it needs exist", func() {
			Expect(k8sClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "auth", Namespace: name.Namespace}})).To(Succeed())
			Expect(k8sClient.Create(ctx, &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{Name: "prefect", Namespace: name.Namespace},
				Spec:       corev1.ServiceSpec{Ports: []corev1.ServicePort{{Port: 4200}}},
			})).To(Succeed())
			queue := queueDeleting(prefectiov1.PrefectServerReference{
				Name: "prefect",
				Auth: &prefectiov1.ServerAuth{SecretKeyRef: *secretRef("auth")},
			})

			result, err := finalize(ctx, k8sClient, recorder, queue, finalizer, failingDelete)

			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(BeAFirstRetry())
			Expect(k8sClient.Get(ctx, name, queue)).To(Succeed())
			Expect(queue.Finalizers).To(ConsistOf(finalizer))

			controllerutil.RemoveFinalizer(queue, finalizer)
			Expect(k8sClient.Update(ctx, queue)).To(Succeed())
		})

		It("keeps retrying when the server's namespace isn't terminating", func() {
			otherNamespace := fmt.Sprintf("finalize-server-ns-%d", time.Now().UnixNano())
			Expect(k8sClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: otherNamespace}})).To(Succeed())
			queue := queueDeleting(prefectiov1.PrefectServerReference{Name: "prefect", Namespace: otherNamespace})

			result, err := finalize(ctx, k8sClient, recorder, queue, finalizer, failingDelete)

			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(BeAFirstRetry())
			Expect(k8sClient.Get(ctx, name, queue)).To(Succeed())
			Expect(queue.Finalizers).To(ConsistOf(finalizer))

			controllerutil.RemoveFinalizer(queue, finalizer)
			Expect(k8sClient.Update(ctx, queue)).To(Succeed())
		})
	})

	It("keeps retrying when the namespace isn't terminating, even if the Secret is gone", func() {
		queue := &prefectiov1.PrefectWorkQueue{
			ObjectMeta: metav1.ObjectMeta{Name: name.Name, Namespace: name.Namespace, Finalizers: []string{finalizer}},
			Spec: prefectiov1.PrefectWorkQueueSpec{
				Server: prefectiov1.PrefectServerReference{
					RemoteAPIURL: new("https://prefect.example.com"),
					APIKey: &prefectiov1.APIKeySpec{ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "api-key"}, Key: "key"}}},
				},
				Name:         "default",
				WorkPoolName: "pool",
			},
		}
		Expect(k8sClient.Create(ctx, queue)).To(Succeed())
		Expect(k8sClient.Delete(ctx, queue)).To(Succeed())
		Expect(k8sClient.Get(ctx, name, queue)).To(Succeed())

		result, err := finalize(ctx, k8sClient, recorder, queue, finalizer, func(context.Context) error {
			return errors.New("secret not found")
		})

		Expect(err).NotTo(HaveOccurred())
		Expect(result.RequeueAfter).To(BeAFirstRetry())
		Expect(k8sClient.Get(ctx, name, queue)).To(Succeed())
		Expect(queue.Finalizers).To(ConsistOf(finalizer))

		controllerutil.RemoveFinalizer(queue, finalizer)
		Expect(k8sClient.Update(ctx, queue)).To(Succeed())
	})
})

// BeAFirstRetry matches the RequeueAfter of a delete that failed just after
// the object's deletion. Deletion timestamps have one-second precision, so the
// delay can be a little over deleteRetryMinInterval.
func BeAFirstRetry() OmegaMatcher {
	return And(BeNumerically(">=", deleteRetryMinInterval), BeNumerically("<", 3*time.Second))
}

var _ = DescribeTable("deleteRetryDelay",
	func(sinceDeletion, want time.Duration) {
		Expect(deleteRetryDelay(sinceDeletion)).To(Equal(want))
	},
	Entry("waits the minimum right after the deletion", time.Duration(0), deleteRetryMinInterval),
	Entry("waits as long as the deletion has been in progress", 4*time.Second, 4*time.Second),
	Entry("waits at most deleteRetryInterval", 10*time.Minute, deleteRetryInterval),
)
