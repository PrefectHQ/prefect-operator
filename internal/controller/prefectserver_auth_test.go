package controller

import (
	"context"
	"errors"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	prefectiov1 "github.com/PrefectHQ/prefect-operator/api/v1"
)

var _ = Describe("PrefectServer auth", func() {
	var (
		ctx        context.Context
		name       types.NamespacedName
		recorder   *events.FakeRecorder
		reconciler *PrefectServerReconciler
	)

	BeforeEach(func() {
		ctx = context.Background()
		namespaceName := fmt.Sprintf("server-auth-ns-%d", time.Now().UnixNano())
		Expect(k8sClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespaceName}})).To(Succeed())

		name = types.NamespacedName{Namespace: namespaceName, Name: "prefect"}
		recorder = events.NewFakeRecorder(10)
		reconciler = &PrefectServerReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(), Recorder: recorder}

		Expect(k8sClient.Create(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Namespace: name.Namespace, Name: "prefect-auth"},
			Data:       map[string][]byte{"auth-string": []byte("admin:hunter2")},
		})).To(Succeed())
	})

	setAuthString := func(data map[string][]byte) {
		GinkgoHelper()
		secret := &corev1.Secret{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: name.Namespace, Name: "prefect-auth"}, secret)).To(Succeed())
		secret.Data = data
		Expect(k8sClient.Update(ctx, secret)).To(Succeed())
	}

	condition := func(conditionType string) *metav1.Condition {
		GinkgoHelper()
		server := &prefectiov1.PrefectServer{}
		Expect(k8sClient.Get(ctx, name, server)).To(Succeed())
		return meta.FindStatusCondition(server.Status.Conditions, conditionType)
	}

	newServer := func(optional *bool) *prefectiov1.PrefectServer {
		return &prefectiov1.PrefectServer{
			ObjectMeta: metav1.ObjectMeta{Namespace: name.Namespace, Name: name.Name},
			Spec: prefectiov1.PrefectServerSpec{Auth: &prefectiov1.ServerAuth{SecretKeyRef: corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: "prefect-auth"},
				Key:                  "auth-string",
				Optional:             optional,
			}}},
		}
	}

	reconcileServer := func() error {
		_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: name})
		return err
	}

	expectUnsupported := func(image string) {
		GinkgoHelper()
		message := fmt.Sprintf("spec.auth requires Prefect 3.6.22 or later, and the server's image is %q", image)
		err := reconcileServer()
		Expect(err).To(MatchError(ContainSubstring(message)))
		Expect(errors.Is(err, reconcile.TerminalError(nil))).To(BeTrue())

		for _, conditionType := range []string{"DeploymentReconciled", "AuthSupported"} {
			cond := condition(conditionType)
			Expect(cond).NotTo(BeNil(), conditionType)
			Expect(cond.Status).To(Equal(metav1.ConditionFalse))
			Expect(cond.Reason).To(Equal("UnsupportedVersion"))
			Expect(cond.Message).To(Equal(message))
		}
		Expect(recorder.Events).To(Receive(Equal("Warning UnsupportedVersion " + message)))

		Expect(apierrors.IsNotFound(k8sClient.Get(ctx, name, &appsv1.Deployment{}))).To(BeTrue())
	}

	It("doesn't deploy a version that does not support auth", func() {
		server := newServer(nil)
		server.Spec.Version = new("3.6.21")
		Expect(k8sClient.Create(ctx, server)).To(Succeed())
		expectUnsupported("prefecthq/prefect:3.6.21-python3.12")

		Expect(k8sClient.Get(ctx, name, server)).To(Succeed())
		server.Spec.Version = new("3.6.22")
		Expect(k8sClient.Update(ctx, server)).To(Succeed())
		Expect(reconcileServer()).To(Succeed())
		Expect(k8sClient.Get(ctx, name, &appsv1.Deployment{})).To(Succeed())
		Expect(condition("AuthSupported").Status).To(Equal(metav1.ConditionTrue))
	})

	It("reports an image whose version it can't tell", func() {
		server := newServer(nil)
		server.Spec.Image = new("registry.example.com/prefect:3.1.0")
		Expect(k8sClient.Create(ctx, server)).To(Succeed())
		Expect(reconcileServer()).To(Succeed())
		Expect(k8sClient.Get(ctx, name, &appsv1.Deployment{})).To(Succeed())

		cond := condition("AuthSupported")
		Expect(cond.Status).To(Equal(metav1.ConditionUnknown))
		Expect(cond.Reason).To(Equal("VersionUnknown"))
		Expect(cond.Message).To(ContainSubstring(`"registry.example.com/prefect:3.1.0"`))

		Expect(k8sClient.Get(ctx, name, server)).To(Succeed())
		server.Spec.Auth = nil
		Expect(k8sClient.Update(ctx, server)).To(Succeed())
		Expect(reconcileServer()).To(Succeed())
		Expect(condition("AuthSupported")).To(BeNil())
	})

	DescribeTable("doesn't deploy without a valid auth string",
		func(data map[string][]byte, message string) {
			setAuthString(data)
			Expect(k8sClient.Create(ctx, newServer(nil))).To(Succeed())

			err := reconcileServer()
			Expect(err).To(MatchError(message))
			Expect(errors.Is(err, reconcile.TerminalError(nil))).To(BeFalse())
			cond := condition("DeploymentReconciled")
			Expect(cond).NotTo(BeNil())
			Expect(cond.Status).To(Equal(metav1.ConditionFalse))
			Expect(cond.Reason).To(Equal("InvalidAuth"))
			Expect(cond.Message).To(Equal(message))
			Expect(recorder.Events).To(Receive(Equal("Warning InvalidAuth " + message)))
			Expect(condition("AuthSupported")).To(BeNil())
			Expect(apierrors.IsNotFound(k8sClient.Get(ctx, name, &appsv1.Deployment{}))).To(BeTrue())

			setAuthString(map[string][]byte{"auth-string": []byte("admin:hunter2")})
			Expect(reconcileServer()).To(Succeed())
			Expect(k8sClient.Get(ctx, name, &appsv1.Deployment{})).To(Succeed())
		},
		Entry("when it is empty", map[string][]byte{"auth-string": nil},
			`key auth-string in secret prefect-auth must be an auth string in the form "username:password"`),
		Entry("when it is blank", map[string][]byte{"auth-string": []byte(" \n")},
			`key auth-string in secret prefect-auth must be an auth string in the form "username:password"`),
		Entry("without a password", map[string][]byte{"auth-string": []byte("admin:\n")},
			`key auth-string in secret prefect-auth must be an auth string in the form "username:password"`),
		Entry("without a username", map[string][]byte{"auth-string": []byte(":hunter2")},
			`key auth-string in secret prefect-auth must be an auth string in the form "username:password"`),
		Entry("when it ends with a newline", map[string][]byte{"auth-string": []byte("admin:hunter2\n")},
			"key auth-string in secret prefect-auth must not start or end with whitespace, such as a newline"),
		Entry("when it starts with a space", map[string][]byte{"auth-string": []byte(" admin:hunter2")},
			"key auth-string in secret prefect-auth must not start or end with whitespace, such as a newline"),
		Entry("when it isn't ASCII", map[string][]byte{"auth-string": []byte("admin:h\u00fcnter2")},
			"key auth-string in secret prefect-auth must contain only ASCII characters"),
		Entry("without the key", map[string][]byte{"other": []byte("admin:hunter2")},
			"key auth-string not found in secret prefect-auth"),
	)

	It("doesn't deploy when settings set the auth string in another case", func() {
		server := newServer(nil)
		server.Spec.Settings = []corev1.EnvVar{{Name: "prefect_server_api_auth_string", Value: "admin:other"}}
		Expect(k8sClient.Create(ctx, server)).To(Succeed())

		message := "settings must not set prefect_server_api_auth_string when auth is set"
		err := reconcileServer()
		Expect(err).To(MatchError(ContainSubstring(message)))
		Expect(errors.Is(err, reconcile.TerminalError(nil))).To(BeTrue())
		cond := condition("DeploymentReconciled")
		Expect(cond).NotTo(BeNil())
		Expect(cond.Status).To(Equal(metav1.ConditionFalse))
		Expect(cond.Reason).To(Equal("InvalidAuth"))
		Expect(cond.Message).To(Equal(message))
		Expect(recorder.Events).To(Receive(Equal("Warning InvalidAuth " + message)))
		Expect(apierrors.IsNotFound(k8sClient.Get(ctx, name, &appsv1.Deployment{}))).To(BeTrue())
	})

	It("restores the auth string that a deploymentTemplate sets", func() {
		server := newServer(nil)
		server.Spec.DeploymentTemplate = rawTemplate(
			`{"spec":{"containers":[{"name":"prefect-server","env":[{"name":"PREFECT_SERVER_API_AUTH_STRING","value":"admin:other"}]}]}}`)
		Expect(k8sClient.Create(ctx, server)).To(Succeed())
		Expect(reconcileServer()).To(Succeed())

		deployment := &appsv1.Deployment{}
		Expect(k8sClient.Get(ctx, name, deployment)).To(Succeed())
		Expect(deployment.Spec.Template.Spec.Containers[0].Env).To(ContainElement(And(
			HaveField("Name", "PREFECT_SERVER_API_AUTH_STRING"),
			HaveField("Value", ""),
			HaveField("ValueFrom.SecretKeyRef.Name", "prefect-auth"),
		)))
	})

	It("rejects the auth string in settings, only with auth", func() {
		server := newServer(nil)
		server.Spec.Settings = []corev1.EnvVar{{Name: "PREFECT_SERVER_API_AUTH_STRING", Value: "admin:other"}}
		Expect(k8sClient.Create(ctx, server)).To(MatchError(ContainSubstring(
			"settings must not set PREFECT_SERVER_API_AUTH_STRING when auth is set")))

		server.Spec.Auth = nil
		Expect(k8sClient.Create(ctx, server)).To(Succeed())
		Expect(reconcileServer()).To(Succeed())
		Expect(k8sClient.Get(ctx, name, &appsv1.Deployment{})).To(Succeed())
	})

	It("doesn't report AuthSupported on a running server with no valid auth string", func() {
		server := newServer(nil)
		server.Spec.Version = new("3.6.22")
		auth := server.Spec.Auth
		server.Spec.Auth = nil
		Expect(k8sClient.Create(ctx, server)).To(Succeed())
		Expect(reconcileServer()).To(Succeed())

		setAuthString(map[string][]byte{"auth-string": nil})
		Expect(k8sClient.Get(ctx, name, server)).To(Succeed())
		server.Spec.Auth = auth
		Expect(k8sClient.Update(ctx, server)).To(Succeed())
		Expect(reconcileServer()).NotTo(Succeed())
		Expect(condition("AuthSupported")).To(BeNil())

		setAuthString(map[string][]byte{"auth-string": []byte("admin:hunter2")})
		Expect(reconcileServer()).To(Succeed())
		Expect(condition("AuthSupported").Status).To(Equal(metav1.ConditionTrue))

		// The condition goes away again with the Secret's value.
		setAuthString(map[string][]byte{"auth-string": nil})
		Expect(reconcileServer()).NotTo(Succeed())
		Expect(condition("AuthSupported")).To(BeNil())
	})

	It("doesn't deploy without the Secret", func() {
		Expect(k8sClient.Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: name.Namespace, Name: "prefect-auth"}})).To(Succeed())
		Expect(k8sClient.Create(ctx, newServer(nil))).To(Succeed())
		Expect(reconcileServer()).To(MatchError(ContainSubstring("failed to get secret prefect-auth")))
		Expect(condition("DeploymentReconciled").Reason).To(Equal("InvalidAuth"))
	})

	It("checks the image that a deploymentTemplate sets", func() {
		server := newServer(nil)
		server.Spec.DeploymentTemplate = rawTemplate(
			`{"spec":{"containers":[{"name":"prefect-server","image":"prefecthq/prefect:3.1.0"}]}}`)
		Expect(k8sClient.Create(ctx, server)).To(Succeed())
		expectUnsupported("prefecthq/prefect:3.1.0")
	})

	It("deploys an earlier version without auth", func() {
		server := newServer(nil)
		server.Spec.Auth = nil
		server.Spec.Version = new("3.1.0")
		Expect(k8sClient.Create(ctx, server)).To(Succeed())
		Expect(reconcileServer()).To(Succeed())
		Expect(k8sClient.Get(ctx, name, &appsv1.Deployment{})).To(Succeed())
	})

	It("rejects an optional Secret reference", func() {
		Expect(k8sClient.Create(ctx, newServer(new(true)))).To(MatchError(ContainSubstring("secretKeyRef.optional must not be true")))
		Expect(k8sClient.Create(ctx, newServer(new(false)))).To(Succeed())
	})
})
