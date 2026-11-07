package controller

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	prefectiov1 "github.com/PrefectHQ/prefect-operator/api/v1"
	"github.com/PrefectHQ/prefect-operator/internal/podtemplate"
)

func rawTemplate(template string) *runtime.RawExtension {
	return &runtime.RawExtension{Raw: []byte(template)}
}

var _ = Describe("PrefectServer pod templates", func() {
	var (
		ctx        context.Context
		name       types.NamespacedName
		recorder   *events.FakeRecorder
		reconciler *PrefectServerReconciler
	)

	ephemeral := prefectiov1.PrefectServerSpec{}
	sqlite := prefectiov1.PrefectServerSpec{
		SQLite: &prefectiov1.SQLiteConfiguration{
			StorageClassName: "standard",
			Size:             resource.MustParse("512Mi"),
		},
	}
	postgres := prefectiov1.PrefectServerSpec{
		Postgres: &prefectiov1.PostgresConfiguration{
			Host:     new("some-postgres-server"),
			Port:     new(15432),
			User:     new("a-prefect-user"),
			Password: new("this-is-a-bad-idea"),
			Database: new("some-prefect"),
		},
	}

	BeforeEach(func() {
		ctx = context.Background()
		namespaceName := fmt.Sprintf("podtemplate-ns-%d", time.Now().UnixNano())
		Expect(k8sClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespaceName}})).To(Succeed())

		name = types.NamespacedName{Namespace: namespaceName, Name: "prefect-with-template"}
		recorder = events.NewFakeRecorder(10)
		reconciler = &PrefectServerReconciler{
			Client:   k8sClient,
			Scheme:   k8sClient.Scheme(),
			Recorder: recorder,
		}
	})

	reconcileServer := func() error {
		_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: name})
		return err
	}

	getServer := func() *prefectiov1.PrefectServer {
		GinkgoHelper()
		server := &prefectiov1.PrefectServer{}
		Expect(k8sClient.Get(ctx, name, server)).To(Succeed())
		return server
	}

	getDeployment := func() *appsv1.Deployment {
		GinkgoHelper()
		deployment := &appsv1.Deployment{}
		Expect(k8sClient.Get(ctx, name, deployment)).To(Succeed())
		return deployment
	}

	serverContainer := func(deployment *appsv1.Deployment) *corev1.Container {
		GinkgoHelper()
		container := podtemplate.FindContainer(&deployment.Spec.Template.Spec, prefectiov1.ContainerNamePrefectServer)
		Expect(container).NotTo(BeNil())
		return container
	}

	// createServer creates a server with spec and reconciles it until its
	// Deployment exists.
	createServer := func(spec prefectiov1.PrefectServerSpec) *prefectiov1.PrefectServer {
		GinkgoHelper()
		server := &prefectiov1.PrefectServer{
			ObjectMeta: metav1.ObjectMeta{Namespace: name.Namespace, Name: name.Name},
			Spec:       *spec.DeepCopy(),
		}
		Expect(k8sClient.Create(ctx, server)).To(Succeed())
		Expect(reconcileServer()).To(Succeed())
		if spec.Postgres != nil {
			migrateAndReconcile(ctx, getServer())
		}
		return getServer()
	}

	setDeploymentTemplate := func(template *runtime.RawExtension) error {
		GinkgoHelper()
		server := getServer()
		server.Spec.DeploymentTemplate = template
		Expect(k8sClient.Update(ctx, server)).To(Succeed())
		return reconcileServer()
	}

	migrationJobs := func() []batchv1.Job {
		GinkgoHelper()
		jobs := &batchv1.JobList{}
		Expect(k8sClient.List(ctx, jobs, client.InNamespace(name.Namespace))).To(Succeed())
		return jobs.Items
	}

	Describe("the migration Job", func() {
		It("gets migrationJobTemplate and not deploymentTemplate", func() {
			spec := *postgres.DeepCopy()
			spec.MigrationJobTemplate = rawTemplate(`{"spec":{"securityContext":{"runAsUser":1001}}}`)
			createServer(spec)

			jobs := migrationJobs()
			Expect(jobs).To(HaveLen(1))
			Expect(jobs[0].Spec.Template.Spec.SecurityContext.RunAsUser).To(Equal(new(int64(1001))))
			Expect(getDeployment().Spec.Template.Spec.SecurityContext.RunAsUser).To(BeNil())
		})

		It("doesn't get deploymentTemplate", func() {
			spec := *postgres.DeepCopy()
			spec.DeploymentTemplate = rawTemplate(`{"spec":{"securityContext":{"runAsUser":1001}}}`)
			createServer(spec)

			jobs := migrationJobs()
			Expect(jobs).To(HaveLen(1))
			Expect(jobs[0].Spec.Template.Spec.SecurityContext.RunAsUser).To(BeNil())
		})

		It("starts a new migration Job when the template changes", func() {
			createServer(*postgres.DeepCopy())
			Expect(migrationJobs()).To(HaveLen(1))
			first := migrationJobs()[0].Name

			server := getServer()
			server.Spec.MigrationJobTemplate = rawTemplate(`{"spec":{"securityContext":{"runAsUser":1001}}}`)
			Expect(k8sClient.Update(ctx, server)).To(Succeed())
			Expect(reconcileServer()).To(Succeed())

			jobs := migrationJobs()
			names := make([]string, 0, len(jobs))
			for _, job := range jobs {
				names = append(names, job.Name)
			}
			Expect(names).To(HaveLen(2))
			Expect(names).To(ContainElement(first))
		})

		It("reports an invalid migrationJobTemplate", func() {
			createServer(*postgres.DeepCopy())

			server := getServer()
			server.Spec.MigrationJobTemplate = rawTemplate(`{"spec":{"securtyContext":{}}}`)
			Expect(k8sClient.Update(ctx, server)).To(Succeed())
			Expect(reconcileServer()).To(MatchError(ContainSubstring("spec.migrationJobTemplate")))

			cond := meta.FindStatusCondition(getServer().Status.Conditions, "MigrationJobReconciled")
			Expect(cond).NotTo(BeNil())
			Expect(cond.Status).To(Equal(metav1.ConditionFalse))
			Expect(cond.Reason).To(Equal("InvalidPodTemplate"))
			Expect(recorder.Events).To(Receive(Equal("Warning InvalidPodTemplate " + cond.Message)))
		})
	})

	DescribeTable("applies the template to the server pod",
		func(spec prefectiov1.PrefectServerSpec) {
			spec.DeploymentTemplate = rawTemplate(`{"spec":{
				"securityContext":{"runAsUser":1001},
				"containers":[{"name":"prefect-server","readinessProbe":{"initialDelaySeconds":30}}]
			}}`)
			server := createServer(spec)

			deployment := getDeployment()
			Expect(deployment.Spec.Template.Spec.SecurityContext.RunAsUser).To(Equal(new(int64(1001))))
			probe := serverContainer(deployment).ReadinessProbe
			Expect(probe.InitialDelaySeconds).To(Equal(int32(30)))
			Expect(probe.PeriodSeconds).To(Equal(server.ReadinessProbe().PeriodSeconds))
		},
		Entry("with an ephemeral backend", ephemeral),
		Entry("with a SQLite backend", sqlite),
		Entry("with a PostgreSQL backend", postgres),
	)

	It("lets the template win over a typed field", func() {
		spec := *ephemeral.DeepCopy()
		spec.NodeSelector = map[string]string{"a": "typed"}
		spec.DeploymentTemplate = rawTemplate(`{"spec":{"nodeSelector":{"a":"template"}}}`)
		createServer(spec)

		Expect(getDeployment().Spec.Template.Spec.NodeSelector).To(HaveKeyWithValue("a", "template"))
	})

	It("removes an override deleted from the template", func() {
		spec := *ephemeral.DeepCopy()
		spec.DeploymentTemplate = rawTemplate(`{"spec":{
			"securityContext":{"runAsUser":1001},
			"containers":[{"name":"prefect-server","readinessProbe":{"initialDelaySeconds":30}}]
		}}`)
		server := createServer(spec)
		Expect(getDeployment().Spec.Template.Spec.SecurityContext.RunAsUser).NotTo(BeNil())

		Expect(setDeploymentTemplate(nil)).To(Succeed())

		deployment := getDeployment()
		Expect(deployment.Spec.Template.Spec.SecurityContext).To(SatisfyAny(BeNil(), Equal(&corev1.PodSecurityContext{})))
		Expect(serverContainer(deployment).ReadinessProbe.InitialDelaySeconds).To(Equal(server.ReadinessProbe().InitialDelaySeconds))
	})

	It("ignores overrides of protected fields", func() {
		spec := *ephemeral.DeepCopy()
		spec.DeploymentTemplate = rawTemplate(`{
			"metadata":{"labels":{"app":"x"}},
			"spec":{"containers":[{"name":"prefect-server","env":[{"name":"PREFECT_HOME","value":"/tmp"}]}]}
		}`)
		server := createServer(spec)

		deployment := getDeployment()
		for key, value := range server.ServerLabels() {
			Expect(deployment.Spec.Template.Labels).To(HaveKeyWithValue(key, value))
		}
		Expect(serverContainer(deployment).Env).To(ContainElement(corev1.EnvVar{
			Name:  prefectiov1.EnvPrefectHome,
			Value: prefectiov1.PrefectHomePath,
		}))
		Expect(serverContainer(deployment).Env).NotTo(ContainElement(corev1.EnvVar{
			Name:  prefectiov1.EnvPrefectHome,
			Value: "/tmp",
		}))
	})

	It("doesn't roll out on a no-op reconcile", func() {
		spec := *ephemeral.DeepCopy()
		spec.DeploymentTemplate = rawTemplate(`{"spec":{"securityContext":{"runAsUser":1001}}}`)
		createServer(spec)
		generation := getDeployment().Generation

		Expect(reconcileServer()).To(Succeed())

		Expect(getDeployment().Generation).To(Equal(generation))
	})

	It("keeps a kubectl rollout restart", func() {
		createServer(*ephemeral.DeepCopy())
		deployment := getDeployment()
		deployment.Spec.Template.Annotations = map[string]string{restartedAtAnnotation: "2026-11-06T00:00:00Z"}
		Expect(k8sClient.Update(ctx, deployment)).To(Succeed())

		Expect(reconcileServer()).To(Succeed())

		Expect(getDeployment().Spec.Template.Annotations).To(HaveKeyWithValue(restartedAtAnnotation, "2026-11-06T00:00:00Z"))
	})

	It("reports an invalid template", func() {
		createServer(*ephemeral.DeepCopy())
		resourceVersion := getDeployment().ResourceVersion

		err := setDeploymentTemplate(rawTemplate(`{"spec":{"securtyContext":{}}}`))
		Expect(err).To(MatchError(ContainSubstring("spec.deploymentTemplate")))

		cond := meta.FindStatusCondition(getServer().Status.Conditions, "DeploymentReconciled")
		Expect(cond).NotTo(BeNil())
		Expect(cond.Status).To(Equal(metav1.ConditionFalse))
		Expect(cond.Reason).To(Equal("InvalidPodTemplate"))
		Expect(recorder.Events).To(Receive(Equal("Warning InvalidPodTemplate " + cond.Message)))
		Expect(getDeployment().ResourceVersion).To(Equal(resourceVersion))
	})
})
