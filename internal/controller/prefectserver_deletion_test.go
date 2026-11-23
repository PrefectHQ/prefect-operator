package controller

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	prefectiov1 "github.com/PrefectHQ/prefect-operator/api/v1"
)

var _ = Describe("PrefectServer deletion", func() {
	const otherFinalizer = "example.com/other"

	var (
		ctx        context.Context
		namespace  string
		recorder   *events.FakeRecorder
		reconciler *PrefectServerReconciler
		server     *prefectiov1.PrefectServer
	)

	newNamespace := func(prefix string) string {
		name := fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
		Expect(k8sClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}})).To(Succeed())
		return name
	}

	reconcileServer := func() reconcile.Result {
		result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(server)})
		Expect(err).NotTo(HaveOccurred())
		return result
	}

	serverGone := func() bool {
		err := k8sClient.Get(ctx, client.ObjectKeyFromObject(server), &prefectiov1.PrefectServer{})
		if apierrors.IsNotFound(err) {
			return true
		}
		Expect(err).NotTo(HaveOccurred())
		return false
	}

	// workPool creates a PrefectWorkPool in namespace whose spec.server is
	// ref, with the finalizer its controller adds.
	workPool := func(namespace, name string, ref prefectiov1.PrefectServerReference) *prefectiov1.PrefectWorkPool {
		pool := &prefectiov1.PrefectWorkPool{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace, Finalizers: []string{PrefectWorkPoolFinalizer}},
			Spec:       prefectiov1.PrefectWorkPoolSpec{Type: "kubernetes", Server: ref},
		}
		Expect(k8sClient.Create(ctx, pool)).To(Succeed())
		return pool
	}

	deployment := func(namespace, name string, ref prefectiov1.PrefectServerReference) *prefectiov1.PrefectDeployment {
		d := &prefectiov1.PrefectDeployment{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace, Finalizers: []string{PrefectDeploymentFinalizer}},
			Spec: prefectiov1.PrefectDeploymentSpec{
				Server:     ref,
				WorkPool:   prefectiov1.PrefectWorkPoolReference{Name: "pool"},
				Deployment: prefectiov1.PrefectDeploymentConfiguration{Entrypoint: "flows.py:flow"},
			},
		}
		Expect(k8sClient.Create(ctx, d)).To(Succeed())
		return d
	}

	// finishDeleting deletes obj and removes its finalizer, the way its own
	// controller does once it has deleted the Prefect object.
	finishDeleting := func(obj client.Object, finalizer string) {
		Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, obj))).To(Succeed())
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(obj), obj)).To(Succeed())
		controllerutil.RemoveFinalizer(obj, finalizer)
		Expect(k8sClient.Update(ctx, obj)).To(Succeed())
	}

	deletionBlocked := func() *metav1.Condition {
		current := &prefectiov1.PrefectServer{}
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(server), current)).To(Succeed())
		return meta.FindStatusCondition(current.Status.Conditions, "DeletionBlocked")
	}

	BeforeEach(func() {
		ctx = context.Background()
		namespace = newNamespace("server-deletion")
		recorder = events.NewFakeRecorder(10)
		reconciler = &PrefectServerReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(), Recorder: recorder}
		server = &prefectiov1.PrefectServer{
			ObjectMeta: metav1.ObjectMeta{Name: "prefect", Namespace: namespace},
		}
		Expect(k8sClient.Create(ctx, server)).To(Succeed())
	})

	It("adds the finalizer and still reconciles the server's Deployment", func() {
		reconcileServer()

		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(server), server)).To(Succeed())
		Expect(server.Finalizers).To(ConsistOf(PrefectServerFinalizer))
		Expect(meta.IsStatusConditionTrue(server.Status.Conditions, "DeploymentReconciled")).To(BeTrue())
	})

	It("removes the finalizer of a server with no dependents", func() {
		reconcileServer()
		Expect(k8sClient.Delete(ctx, server)).To(Succeed())

		Expect(reconcileServer()).To(Equal(reconcile.Result{}))
		Expect(serverGone()).To(BeTrue())
	})

	It("keeps the server until the dependents deleted with it are gone", func() {
		reconcileServer()
		ref := prefectiov1.PrefectServerReference{Name: server.Name}
		pool := workPool(namespace, "pool", ref)
		d := deployment(namespace, "flow", ref)

		Expect(k8sClient.Delete(ctx, server)).To(Succeed())
		Expect(k8sClient.Delete(ctx, pool)).To(Succeed())
		Expect(k8sClient.Delete(ctx, d)).To(Succeed())

		Expect(reconcileServer().RequeueAfter).To(Equal(deleteRetryInterval))
		Expect(serverGone()).To(BeFalse())
		condition := deletionBlocked()
		Expect(condition).NotTo(BeNil())
		Expect(condition.Message).To(Equal(fmt.Sprintf(
			"2 resources use this server and must be deleted first: PrefectDeployment %s/flow, PrefectWorkPool %s/pool",
			namespace, namespace)))
		Expect(recorder.Events).To(Receive(HavePrefix("Warning DependentsExist 2 resources use this server")))

		finishDeleting(pool, PrefectWorkPoolFinalizer)
		reconcileServer()
		Expect(serverGone()).To(BeFalse())
		// The message changed, so there's another event.
		Expect(recorder.Events).To(Receive(HavePrefix("Warning DependentsExist 1 resource uses this server")))

		finishDeleting(d, PrefectDeploymentFinalizer)
		Expect(reconcileServer()).To(Equal(reconcile.Result{}))
		Expect(serverGone()).To(BeTrue())
	})

	It("waits for a dependent that isn't being deleted", func() {
		reconcileServer()
		pool := workPool(namespace, "pool", prefectiov1.PrefectServerReference{Name: server.Name})

		Expect(k8sClient.Delete(ctx, server)).To(Succeed())
		reconcileServer()
		Expect(recorder.Events).To(Receive())

		// A second reconcile with the same dependents doesn't repeat the event.
		reconcileServer()
		Expect(recorder.Events).NotTo(Receive())
		Expect(serverGone()).To(BeFalse())

		finishDeleting(pool, PrefectWorkPoolFinalizer)
		reconcileServer()
		Expect(serverGone()).To(BeTrue())
	})

	It("waits for a dependent in another namespace, and only one that names the server's namespace", func() {
		reconcileServer()
		other := newNamespace("server-deletion-other")
		// This one names a server called "prefect" in its own namespace.
		workPool(other, "local", prefectiov1.PrefectServerReference{Name: server.Name})
		remote := workPool(other, "remote", prefectiov1.PrefectServerReference{Name: server.Name, Namespace: namespace})

		Expect(k8sClient.Delete(ctx, server)).To(Succeed())
		reconcileServer()
		Expect(deletionBlocked().Message).To(Equal(fmt.Sprintf(
			"1 resource uses this server and must be deleted first: PrefectWorkPool %s/remote", other)))

		finishDeleting(remote, PrefectWorkPoolFinalizer)
		reconcileServer()
		Expect(serverGone()).To(BeTrue())
	})

	It("doesn't wait for a resource that also sets remoteApiUrl", func() {
		reconcileServer()
		pool := workPool(namespace, "remote", prefectiov1.PrefectServerReference{
			Name:         server.Name,
			RemoteAPIURL: new("https://prefect.example.com/api"),
		})

		Expect(k8sClient.Delete(ctx, server)).To(Succeed())
		reconcileServer()
		Expect(serverGone()).To(BeTrue())

		finishDeleting(pool, PrefectWorkPoolFinalizer)
	})

	It("names at most five dependents in the condition", func() {
		reconcileServer()
		for i := range 7 {
			workPool(namespace, fmt.Sprintf("pool-%d", i), prefectiov1.PrefectServerReference{Name: server.Name})
		}

		Expect(k8sClient.Delete(ctx, server)).To(Succeed())
		reconcileServer()
		Expect(deletionBlocked().Message).To(HaveSuffix(fmt.Sprintf("PrefectWorkPool %s/pool-4, and 2 more", namespace)))

		for i := range 7 {
			pool := &prefectiov1.PrefectWorkPool{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: namespace, Name: fmt.Sprintf("pool-%d", i)}, pool)).To(Succeed())
			finishDeleting(pool, PrefectWorkPoolFinalizer)
		}
		reconcileServer()
		Expect(serverGone()).To(BeTrue())
	})

	It("leaves alone a server deleted before it had the finalizer", func() {
		server.Finalizers = []string{otherFinalizer}
		Expect(k8sClient.Update(ctx, server)).To(Succeed())
		Expect(k8sClient.Delete(ctx, server)).To(Succeed())

		reconcileServer()

		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(server), server)).To(Succeed())
		Expect(server.Finalizers).To(ConsistOf(otherFinalizer))
		finishDeleting(server, otherFinalizer)
	})

	Describe("serverOfDeletedDependent", func() {
		It("requests the server a deleted dependent used, only while the server is being deleted", func() {
			reconcileServer()
			pool := &prefectiov1.PrefectWorkPool{
				ObjectMeta: metav1.ObjectMeta{Name: "pool", Namespace: namespace},
				Spec:       prefectiov1.PrefectWorkPoolSpec{Server: prefectiov1.PrefectServerReference{Name: server.Name}},
			}

			Expect(reconciler.serverOfDeletedDependent(ctx, pool)).To(BeEmpty())

			Expect(k8sClient.Delete(ctx, server)).To(Succeed())
			Expect(reconciler.serverOfDeletedDependent(ctx, pool)).To(ConsistOf(
				reconcile.Request{NamespacedName: client.ObjectKeyFromObject(server)}))

			reconcileServer()
			Expect(serverGone()).To(BeTrue())
		})

		It("ignores a dependent that uses a remote server or a server that doesn't exist", func() {
			remote := &prefectiov1.PrefectWorkPool{
				ObjectMeta: metav1.ObjectMeta{Name: "pool", Namespace: namespace},
				Spec: prefectiov1.PrefectWorkPoolSpec{Server: prefectiov1.PrefectServerReference{
					RemoteAPIURL: new("https://prefect.example.com/api"),
				}},
			}
			missing := &prefectiov1.PrefectWorkPool{
				ObjectMeta: metav1.ObjectMeta{Name: "pool", Namespace: namespace},
				Spec:       prefectiov1.PrefectWorkPoolSpec{Server: prefectiov1.PrefectServerReference{Name: "missing"}},
			}

			Expect(reconciler.serverOfDeletedDependent(ctx, remote)).To(BeEmpty())
			Expect(reconciler.serverOfDeletedDependent(ctx, missing)).To(BeEmpty())
		})
	})
})
