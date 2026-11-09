package controller

import (
	"context"
	"fmt"
	"slices"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	prefectiov1 "github.com/PrefectHQ/prefect-operator/api/v1"
	"github.com/PrefectHQ/prefect-operator/internal/podtemplate"
	"github.com/PrefectHQ/prefect-operator/internal/prefect/prefecttest"
)

var _ = Describe("PrefectWorkPool pod templates", func() {
	var (
		ctx        context.Context
		name       types.NamespacedName
		recorder   *events.FakeRecorder
		reconciler *PrefectWorkPoolReconciler
	)

	BeforeEach(func() {
		ctx = context.Background()
		namespaceName := fmt.Sprintf("work-pool-podtemplate-ns-%d", time.Now().UnixNano())
		Expect(k8sClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespaceName}})).To(Succeed())

		prefectAPI := prefecttest.NewServer()
		DeferCleanup(prefectAPI.Close)

		name = types.NamespacedName{Namespace: namespaceName, Name: "pool-with-template"}
		recorder = events.NewFakeRecorder(10)
		reconciler = &PrefectWorkPoolReconciler{
			Client:                k8sClient,
			Scheme:                k8sClient.Scheme(),
			NewPrefectClient:      fakePrefectClients(prefectAPI),
			DefaultResyncInterval: testResyncInterval,
			Recorder:              recorder,
		}
	})

	reconcileWorkPool := func() error {
		_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: name})
		return err
	}

	getWorkPool := func() *prefectiov1.PrefectWorkPool {
		GinkgoHelper()
		workPool := &prefectiov1.PrefectWorkPool{}
		Expect(k8sClient.Get(ctx, name, workPool)).To(Succeed())
		return workPool
	}

	getDeployment := func() *appsv1.Deployment {
		GinkgoHelper()
		deployment := &appsv1.Deployment{}
		Expect(k8sClient.Get(ctx, name, deployment)).To(Succeed())
		return deployment
	}

	workerContainer := func(deployment *appsv1.Deployment) *corev1.Container {
		GinkgoHelper()
		container := podtemplate.FindContainer(&deployment.Spec.Template.Spec, prefectiov1.ContainerNameWorker)
		Expect(container).NotTo(BeNil())
		return container
	}

	// createWorkPool creates a work pool with template and reconciles it until
	// its Deployment exists: once to add the finalizer, once to sync.
	createWorkPool := func(template *runtime.RawExtension) *prefectiov1.PrefectWorkPool {
		GinkgoHelper()
		workPool := &prefectiov1.PrefectWorkPool{
			ObjectMeta: metav1.ObjectMeta{Namespace: name.Namespace, Name: name.Name},
			Spec: prefectiov1.PrefectWorkPoolSpec{
				Type:               "kubernetes",
				Workers:            1,
				DeploymentTemplate: template,
			},
		}
		Expect(k8sClient.Create(ctx, workPool)).To(Succeed())
		Expect(reconcileWorkPool()).To(Succeed())
		Expect(reconcileWorkPool()).To(Succeed())
		return getWorkPool()
	}

	setDeploymentTemplate := func(template *runtime.RawExtension) error {
		GinkgoHelper()
		workPool := getWorkPool()
		workPool.Spec.DeploymentTemplate = template
		Expect(k8sClient.Update(ctx, workPool)).To(Succeed())
		return reconcileWorkPool()
	}

	It("applies the template to the worker pod", func() {
		workPool := createWorkPool(rawTemplate(`{"spec":{
			"securityContext":{"runAsNonRoot":true},
			"containers":[{"name":"prefect-worker","livenessProbe":{"failureThreshold":10}}]
		}}`))

		deployment := getDeployment()
		Expect(deployment.Spec.Template.Spec.SecurityContext.RunAsNonRoot).To(Equal(new(true)))
		probe := workerContainer(deployment).LivenessProbe
		Expect(probe.FailureThreshold).To(Equal(int32(10)))
		Expect(probe.PeriodSeconds).To(Equal(workPool.LivenessProbe().PeriodSeconds))
		Expect(probe.HTTPGet).To(Equal(workPool.LivenessProbe().HTTPGet))
	})

	It("ignores overrides of protected fields", func() {
		workPool := createWorkPool(rawTemplate(`{"spec":{"containers":[{"name":"prefect-worker","env":[
			{"name":"PREFECT_API_URL","value":"http://evil"}
		]}]}}`))

		Expect(workerContainer(getDeployment()).Env).To(ContainElement(corev1.EnvVar{
			Name:  "PREFECT_API_URL",
			Value: workPool.PrefectAPIURL(),
		}))
		Expect(workerContainer(getDeployment()).Env).NotTo(ContainElement(corev1.EnvVar{
			Name:  "PREFECT_API_URL",
			Value: "http://evil",
		}))
	})

	It("keeps settings in order when the template touches a protected env var", func() {
		workPool := &prefectiov1.PrefectWorkPool{
			ObjectMeta: metav1.ObjectMeta{Namespace: name.Namespace, Name: name.Name},
			Spec: prefectiov1.PrefectWorkPoolSpec{
				Type:    "kubernetes",
				Workers: 1,
				Settings: []corev1.EnvVar{
					{Name: "API_HOST", Value: "prefect.example.com"},
					{Name: "PREFECT_API_URL", Value: "https://$(API_HOST)/api"},
				},
				DeploymentTemplate: rawTemplate(`{"spec":{"containers":[{"name":"prefect-worker","env":[
					{"name":"PREFECT_HOME","value":"/tmp"}
				]}]}}`),
			},
		}
		Expect(k8sClient.Create(ctx, workPool)).To(Succeed())
		Expect(reconcileWorkPool()).To(Succeed())
		Expect(reconcileWorkPool()).To(Succeed())

		env := workerContainer(getDeployment()).Env
		host := slices.IndexFunc(env, func(e corev1.EnvVar) bool { return e.Name == "API_HOST" })
		reference := slices.IndexFunc(env, func(e corev1.EnvVar) bool { return e.Value == "https://$(API_HOST)/api" })
		Expect(host).To(BeNumerically(">=", 0))
		Expect(reference).To(BeNumerically(">", host))
	})

	It("removes an override deleted from the template", func() {
		createWorkPool(rawTemplate(`{"spec":{"securityContext":{"runAsNonRoot":true}}}`))
		Expect(getDeployment().Spec.Template.Spec.SecurityContext.RunAsNonRoot).NotTo(BeNil())

		Expect(setDeploymentTemplate(nil)).To(Succeed())

		Expect(getDeployment().Spec.Template.Spec.SecurityContext).To(SatisfyAny(BeNil(), Equal(&corev1.PodSecurityContext{})))
	})

	It("keeps pod template annotations other tools add", func() {
		createWorkPool(nil)
		deployment := getDeployment()
		deployment.Spec.Template.Annotations = map[string]string{"kubectl.kubernetes.io/restartedAt": "2026-11-06T00:00:00Z"}
		Expect(k8sClient.Update(ctx, deployment)).To(Succeed())
		generation := getDeployment().Generation

		Expect(reconcileWorkPool()).To(Succeed())

		deployment = getDeployment()
		Expect(deployment.Spec.Template.Annotations).To(HaveKeyWithValue("kubectl.kubernetes.io/restartedAt", "2026-11-06T00:00:00Z"))
		Expect(deployment.Generation).To(Equal(generation))
	})

	It("removes an annotation deleted from the template, and only that one", func() {
		createWorkPool(rawTemplate(`{"metadata":{"annotations":{"team":"data"}}}`))
		deployment := getDeployment()
		Expect(deployment.Spec.Template.Annotations).To(HaveKeyWithValue("team", "data"))
		deployment.Spec.Template.Annotations["other"] = "tool"
		Expect(k8sClient.Update(ctx, deployment)).To(Succeed())

		Expect(setDeploymentTemplate(nil)).To(Succeed())

		Expect(getDeployment().Spec.Template.Annotations).To(Equal(map[string]string{"other": "tool"}))
	})

	It("keeps pod template labels other tools add", func() {
		createWorkPool(nil)
		deployment := getDeployment()
		deployment.Spec.Template.Labels["other"] = "tool"
		Expect(k8sClient.Update(ctx, deployment)).To(Succeed())
		generation := getDeployment().Generation

		Expect(reconcileWorkPool()).To(Succeed())

		deployment = getDeployment()
		Expect(deployment.Spec.Template.Labels).To(HaveKeyWithValue("other", "tool"))
		Expect(deployment.Generation).To(Equal(generation))
	})

	It("reports an invalid template", func() {
		createWorkPool(nil)
		resourceVersion := getDeployment().ResourceVersion

		workPool := getWorkPool()
		workPool.Spec.DeploymentTemplate = rawTemplate(`{"spec":{"securtyContext":{}}}`)
		Expect(k8sClient.Update(ctx, workPool)).To(Succeed())

		// A bad template isn't retried with error backoff, but the work pool
		// keeps resyncing with Prefect on its usual interval.
		result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: name})
		Expect(err).NotTo(HaveOccurred())
		Expect(result.RequeueAfter).To(BeNumerically(">", 0))

		cond := meta.FindStatusCondition(getWorkPool().Status.Conditions, "DeploymentReconciled")
		Expect(cond).NotTo(BeNil())
		Expect(cond.Message).To(ContainSubstring("spec.deploymentTemplate"))
		Expect(cond.Status).To(Equal(metav1.ConditionFalse))
		Expect(cond.Reason).To(Equal("InvalidPodTemplate"))
		Expect(recorder.Events).To(Receive(Equal("Warning InvalidPodTemplate " + cond.Message)))
		Expect(getDeployment().ResourceVersion).To(Equal(resourceVersion))
	})

	It("reports the version from the prefect-worker container", func() {
		createWorkPool(rawTemplate(`{"spec":{"containers":[
			{"name":"sidecar","image":"busybox:1.36"},
			{"name":"prefect-worker","image":"prefecthq/prefect:3.9.9-python3.12"}
		]}}`))

		Expect(reconcileWorkPool()).To(Succeed())

		Expect(getWorkPool().Status.Version).To(Equal("3.9.9"))
	})
})
