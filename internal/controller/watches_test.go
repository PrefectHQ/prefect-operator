package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/event"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	prefectiov1 "github.com/PrefectHQ/prefect-operator/api/v1"
	"github.com/PrefectHQ/prefect-operator/internal/prefect"
	"github.com/PrefectHQ/prefect-operator/internal/prefect/prefecttest"
)

var _ = Describe("workPoolGotPrefectID", func() {
	pool := func(id *string) *prefectiov1.PrefectWorkPool {
		return &prefectiov1.PrefectWorkPool{Status: prefectiov1.PrefectWorkPoolStatus{Id: id}}
	}

	DescribeTable("passes updates that set or change the Prefect ID",
		func(oldID, newID *string, passes bool) {
			Expect(workPoolGotPrefectID.Update(event.UpdateEvent{ObjectOld: pool(oldID), ObjectNew: pool(newID)})).To(Equal(passes))
		},
		Entry("set for the first time", nil, new("a"), true),
		Entry("set after being empty", new(""), new("a"), true),
		Entry("changed", new("a"), new("b"), true),
		Entry("unchanged", new("a"), new("a"), false),
		Entry("cleared", new("a"), nil, false),
		Entry("never set", nil, nil, false),
	)

	It("ignores creates, deletes, and generic events", func() {
		Expect(workPoolGotPrefectID.Create(event.CreateEvent{Object: pool(new("a"))})).To(BeFalse())
		Expect(workPoolGotPrefectID.Delete(event.DeleteEvent{Object: pool(new("a"))})).To(BeFalse())
		Expect(workPoolGotPrefectID.Generic(event.GenericEvent{Object: pool(new("a"))})).To(BeFalse())
	})
})

var _ = Describe("Mapping a watched resource to the resources that use it", func() {
	var (
		ctx    context.Context
		ns1    string
		ns2    string
		suffix string
	)

	BeforeEach(func() {
		ctx = context.Background()
		suffix = fmt.Sprint(time.Now().UnixNano())
		ns1, ns2 = "mapping-a-"+suffix, "mapping-b-"+suffix
		for _, ns := range []string{ns1, ns2} {
			Expect(k8sClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}})).To(Succeed())
		}
	})

	requests := func(keys ...types.NamespacedName) []reconcile.Request {
		out := make([]reconcile.Request, 0, len(keys))
		for _, key := range keys {
			out = append(out, reconcile.Request{NamespacedName: key})
		}
		return out
	}

	poolNamed := func(name string) *prefectiov1.PrefectWorkPool {
		return &prefectiov1.PrefectWorkPool{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns1}}
	}

	It("maps a work pool to the deployments that use it, in every namespace", func() {
		pool, other := "pool-"+suffix, "other-pool-"+suffix
		create := func(namespace, name, workPool string) types.NamespacedName {
			Expect(k8sClient.Create(ctx, &prefectiov1.PrefectDeployment{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
				Spec: prefectiov1.PrefectDeploymentSpec{
					WorkPool:   prefectiov1.PrefectWorkPoolReference{Name: workPool},
					Deployment: prefectiov1.PrefectDeploymentConfiguration{Entrypoint: "flows.py:flow"},
				},
			})).To(Succeed())
			return types.NamespacedName{Namespace: namespace, Name: name}
		}
		first := create(ns1, "first", pool)
		second := create(ns2, "second", pool)
		create(ns1, "elsewhere", other)

		r := &PrefectDeploymentReconciler{Client: k8sClient}
		Expect(r.deploymentsUsingWorkPool(ctx, poolNamed(pool))).To(ConsistOf(requests(first, second)))
	})

	It("maps a work pool to its work queues, in every namespace", func() {
		pool, other := "pool-"+suffix, "other-pool-"+suffix
		create := func(namespace, name, workPool string) types.NamespacedName {
			Expect(k8sClient.Create(ctx, &prefectiov1.PrefectWorkQueue{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
				Spec:       prefectiov1.PrefectWorkQueueSpec{Name: name, WorkPoolName: workPool},
			})).To(Succeed())
			return types.NamespacedName{Namespace: namespace, Name: name}
		}
		first := create(ns1, "first", pool)
		second := create(ns2, "second", pool)
		create(ns1, "elsewhere", other)

		r := &PrefectWorkQueueReconciler{Client: k8sClient}
		Expect(r.workQueuesInWorkPool(ctx, poolNamed(pool))).To(ConsistOf(requests(first, second)))
	})

	It("maps a deployment to the automations that reference it by name, in every namespace", func() {
		deployment, other := "deployment-"+suffix, "other-deployment-"+suffix
		create := func(namespace, name string, spec prefectiov1.PrefectAutomationSpec) types.NamespacedName {
			spec.Name = name
			spec.Trigger = prefectiov1.PrefectAutomationTrigger{Event: &prefectiov1.PrefectEventTrigger{Posture: "Reactive"}}
			Expect(k8sClient.Create(ctx, &prefectiov1.PrefectAutomation{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
				Spec:       spec,
			})).To(Succeed())
			return types.NamespacedName{Namespace: namespace, Name: name}
		}
		runs := func(name string) []prefectiov1.PrefectAutomationAction {
			return []prefectiov1.PrefectAutomationAction{{Type: "run-deployment", Source: new("selected"), DeploymentName: new(name)}}
		}
		inActions := create(ns1, "in-actions", prefectiov1.PrefectAutomationSpec{Actions: runs(deployment)})
		onResolve := create(ns2, "on-resolve", prefectiov1.PrefectAutomationSpec{ActionsOnResolve: runs(deployment)})
		create(ns1, "elsewhere", prefectiov1.PrefectAutomationSpec{Actions: runs(other)})

		r := &PrefectAutomationReconciler{Client: k8sClient}
		named := &prefectiov1.PrefectDeployment{ObjectMeta: metav1.ObjectMeta{Name: deployment, Namespace: ns1}}
		Expect(r.automationsUsingDeployment(ctx, named)).To(ConsistOf(requests(inActions, onResolve)))
	})
})

// These specs run the controllers in a manager, so that watches deliver the
// reconciles. Each waiting resource's fallback requeue is RequeueIntervalWaiting
// away, so a sync well before then shows that a watch triggered it.
var _ = Describe("Watches", Ordered, func() {
	const watchTimeout = 10 * time.Second

	var (
		ctx       context.Context
		namespace string
	)

	BeforeAll(func() {
		Expect(watchTimeout).To(BeNumerically("<", RequeueIntervalWaiting))

		ctx = context.Background()
		namespace = fmt.Sprintf("watches-ns-%d", time.Now().UnixNano())
		Expect(k8sClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}})).To(Succeed())

		prefectAPI := prefecttest.NewServer()
		DeferCleanup(prefectAPI.Close)
		prefectClient := prefect.NewClient(prefectAPI.APIURL, "", logr.Discard())
		_, err := prefectClient.CreateWorkPool(ctx, &prefect.WorkPoolSpec{Name: "existing-pool", Type: "kubernetes"})
		Expect(err).NotTo(HaveOccurred())

		mgr, err := ctrl.NewManager(cfg, ctrl.Options{
			Scheme:  k8sClient.Scheme(),
			Metrics: metricsserver.Options{BindAddress: "0"},
			// The other specs reconcile by hand in their own namespaces, and
			// this manager's controllers must not reconcile their objects.
			Cache: cache.Options{DefaultNamespaces: map[string]cache.Config{namespace: {}}},
			// Controller names are registered for the whole process, so a
			// second run of the suite in one process (ginkgo --repeat) would
			// otherwise fail to start.
			Controller: config.Controller{SkipNameValidation: new(true)},
		})
		Expect(err).NotTo(HaveOccurred())

		recorder := mgr.GetEventRecorder("prefect-operator")
		for _, r := range []interface{ SetupWithManager(ctrl.Manager) error }{
			&PrefectWorkPoolReconciler{Client: mgr.GetClient(), Scheme: mgr.GetScheme(), NewPrefectClient: fakePrefectClients(prefectAPI), DefaultResyncInterval: testResyncInterval, Recorder: recorder},
			&PrefectWorkQueueReconciler{Client: mgr.GetClient(), Scheme: mgr.GetScheme(), NewPrefectClient: fakePrefectClients(prefectAPI), DefaultResyncInterval: testResyncInterval, Recorder: recorder},
			&PrefectDeploymentReconciler{Client: mgr.GetClient(), Scheme: mgr.GetScheme(), NewPrefectClient: fakePrefectClients(prefectAPI), DefaultResyncInterval: testResyncInterval, Recorder: recorder},
			&PrefectAutomationReconciler{Client: mgr.GetClient(), Scheme: mgr.GetScheme(), NewPrefectClient: fakePrefectClients(prefectAPI), DefaultResyncInterval: testResyncInterval, Recorder: recorder},
		} {
			Expect(r.SetupWithManager(mgr)).To(Succeed())
		}

		mgrCtx, cancel := context.WithCancel(ctx)
		stopped := make(chan struct{})
		go func() {
			defer GinkgoRecover()
			defer close(stopped)
			Expect(mgr.Start(mgrCtx)).To(Succeed())
		}()
		DeferCleanup(func() {
			cancel()
			Eventually(stopped).Should(BeClosed())
		})
	})

	// syncedReason returns a function that reads obj again and returns its
	// Synced condition's reason.
	syncedReason := func(obj client.Object, conditions *[]metav1.Condition) func() string {
		return func() string {
			if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(obj), obj); err != nil {
				return err.Error()
			}
			if c := meta.FindStatusCondition(*conditions, "Synced"); c != nil {
				return c.Reason
			}
			return ""
		}
	}

	// waitingFor waits until obj's Synced reason is reason, and then long
	// enough for the reconcile that its own status update triggers to finish.
	waitingFor := func(reason func() string, want string) {
		Eventually(reason).WithTimeout(watchTimeout).Should(Equal(want))
		Consistently(reason).WithTimeout(time.Second).Should(Equal(want))
	}

	It("syncs a deployment when the PrefectWorkPool it waits for syncs", func() {
		deployment := &prefectiov1.PrefectDeployment{
			ObjectMeta: metav1.ObjectMeta{Name: "uses-late-pool", Namespace: namespace},
			Spec: prefectiov1.PrefectDeploymentSpec{
				WorkPool:   prefectiov1.PrefectWorkPoolReference{Name: "late-pool"},
				Deployment: prefectiov1.PrefectDeploymentConfiguration{Entrypoint: "flows.py:flow"},
			},
		}
		Expect(k8sClient.Create(ctx, deployment)).To(Succeed())
		reason := syncedReason(deployment, &deployment.Status.Conditions)
		waitingFor(reason, "WorkPoolNotFound")

		Expect(k8sClient.Create(ctx, &prefectiov1.PrefectWorkPool{
			ObjectMeta: metav1.ObjectMeta{Name: "late-pool", Namespace: namespace},
		})).To(Succeed())

		Eventually(reason).WithTimeout(watchTimeout).Should(Equal("SyncSuccessful"))
	})

	It("syncs a work queue when the PrefectWorkPool it waits for syncs", func() {
		workQueue := &prefectiov1.PrefectWorkQueue{
			ObjectMeta: metav1.ObjectMeta{Name: "in-late-pool", Namespace: namespace},
			Spec:       prefectiov1.PrefectWorkQueueSpec{Name: "ingest", WorkPoolName: "late-queue-pool"},
		}
		Expect(k8sClient.Create(ctx, workQueue)).To(Succeed())
		reason := syncedReason(workQueue, &workQueue.Status.Conditions)
		waitingFor(reason, "WorkPoolNotFound")

		Expect(k8sClient.Create(ctx, &prefectiov1.PrefectWorkPool{
			ObjectMeta: metav1.ObjectMeta{Name: "late-queue-pool", Namespace: namespace},
		})).To(Succeed())

		Eventually(reason).WithTimeout(watchTimeout).Should(Equal("SyncSuccessful"))
	})

	It("syncs an automation when the PrefectDeployment it waits for syncs", func() {
		automation := &prefectiov1.PrefectAutomation{
			ObjectMeta: metav1.ObjectMeta{Name: "runs-late-deployment", Namespace: namespace},
			Spec: prefectiov1.PrefectAutomationSpec{
				Name:    "runs-late-deployment",
				Trigger: prefectiov1.PrefectAutomationTrigger{Event: &prefectiov1.PrefectEventTrigger{Posture: "Reactive"}},
				Actions: []prefectiov1.PrefectAutomationAction{
					{Type: "run-deployment", Source: new("selected"), DeploymentName: new("late-deployment")},
				},
			},
		}
		Expect(k8sClient.Create(ctx, automation)).To(Succeed())
		reason := syncedReason(automation, &automation.Status.Conditions)
		waitingFor(reason, "DeploymentNotFound")

		Expect(k8sClient.Create(ctx, &prefectiov1.PrefectDeployment{
			ObjectMeta: metav1.ObjectMeta{Name: "late-deployment", Namespace: namespace},
			Spec: prefectiov1.PrefectDeploymentSpec{
				WorkPool:   prefectiov1.PrefectWorkPoolReference{Name: "existing-pool"},
				Deployment: prefectiov1.PrefectDeploymentConfiguration{Entrypoint: "flows.py:flow"},
			},
		})).To(Succeed())

		Eventually(reason).WithTimeout(watchTimeout).Should(Equal("SyncSuccessful"))
	})
})
