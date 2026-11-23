package main

import (
	"context"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	prefectiov1 "github.com/PrefectHQ/prefect-operator/api/v1"
)

// These tests apply the chart's admission policies from
// files/admission-policies/ and write Prefect resources as users with
// limited RBAC.
var _ = Describe("The chart's admission policies", Ordered, func() {
	const namespaceName = "admission-policy"

	var (
		ctx context.Context

		// poolEditor may write PrefectServers and PrefectWorkPools.
		poolEditor client.Client
		// poolDeployer may also create Deployments.
		poolDeployer client.Client
		// serverRefEditor may write PrefectWorkQueues, PrefectDeployments and
		// PrefectAutomations.
		serverRefEditor client.Client
		// serverRefReader may also get the prefect-api-key Secret and the
		// prefect-config ConfigMap.
		serverRefReader client.Client
	)

	prefectRule := func(resources ...string) rbacv1.PolicyRule {
		return rbacv1.PolicyRule{APIGroups: []string{"prefect.io"}, Resources: resources, Verbs: []string{"*"}}
	}

	// userClient binds rules to a new user in the test namespace and returns
	// a client that authenticates as that user.
	userClient := func(name string, rules ...rbacv1.PolicyRule) client.Client {
		Expect(adminClient.Create(ctx, &rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespaceName},
			Rules:      rules,
		})).To(Succeed())
		Expect(adminClient.Create(ctx, &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespaceName},
			RoleRef:    rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "Role", Name: name},
			Subjects:   []rbacv1.Subject{{APIGroup: rbacv1.GroupName, Kind: rbacv1.UserKind, Name: name}},
		})).To(Succeed())
		c, err := client.New(addUser(name), client.Options{Scheme: scheme})
		Expect(err).NotTo(HaveOccurred())
		return c
	}

	meta := func(name string) metav1.ObjectMeta {
		return metav1.ObjectMeta{Name: name, Namespace: namespaceName}
	}
	remoteServer := func(apiKey *prefectiov1.APIKeySpec) prefectiov1.PrefectServerReference {
		return prefectiov1.PrefectServerReference{RemoteAPIURL: new("https://api.prefect.cloud"), APIKey: apiKey}
	}
	secretKey := func(name string) *prefectiov1.APIKeySpec {
		return &prefectiov1.APIKeySpec{ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: name}, Key: "api-key",
		}}}
	}
	configMapKey := func(name string) *prefectiov1.APIKeySpec {
		return &prefectiov1.APIKeySpec{ValueFrom: &corev1.EnvVarSource{ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: name}, Key: "api-key",
		}}}
	}

	workPool := func(name string) *prefectiov1.PrefectWorkPool {
		return &prefectiov1.PrefectWorkPool{ObjectMeta: meta(name), Spec: prefectiov1.PrefectWorkPoolSpec{Type: "process"}}
	}
	server := func(name string) *prefectiov1.PrefectServer {
		return &prefectiov1.PrefectServer{ObjectMeta: meta(name)}
	}
	workQueue := func(name string, apiKey *prefectiov1.APIKeySpec) client.Object {
		return &prefectiov1.PrefectWorkQueue{ObjectMeta: meta(name), Spec: prefectiov1.PrefectWorkQueueSpec{
			Name: name, WorkPoolName: "pool", Server: remoteServer(apiKey),
		}}
	}
	deployment := func(name string, apiKey *prefectiov1.APIKeySpec) client.Object {
		return &prefectiov1.PrefectDeployment{ObjectMeta: meta(name), Spec: prefectiov1.PrefectDeploymentSpec{
			Server:     remoteServer(apiKey),
			WorkPool:   prefectiov1.PrefectWorkPoolReference{Name: "pool"},
			Deployment: prefectiov1.PrefectDeploymentConfiguration{Entrypoint: "flow.py:main"},
		}}
	}
	automation := func(name string, apiKey *prefectiov1.APIKeySpec) client.Object {
		return &prefectiov1.PrefectAutomation{ObjectMeta: meta(name), Spec: prefectiov1.PrefectAutomationSpec{
			Name:    name,
			Server:  remoteServer(apiKey),
			Trigger: prefectiov1.PrefectAutomationTrigger{Event: &prefectiov1.PrefectEventTrigger{Posture: "Reactive"}},
		}}
	}

	expectDenied := func(err error, message string) {
		GinkgoHelper()
		Expect(apierrors.IsForbidden(err)).To(BeTrue(), "expected Forbidden, got %v", err)
		Expect(err).To(MatchError(ContainSubstring(message)))
	}

	BeforeAll(func() {
		ctx = context.Background()
		Expect(adminClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespaceName}})).To(Succeed())

		paths, err := filepath.Glob(filepath.Join(chartFiles, "admission-policies", "*.yaml"))
		Expect(err).NotTo(HaveOccurred())
		Expect(paths).To(HaveLen(2))
		for _, path := range paths {
			raw, err := os.ReadFile(path)
			Expect(err).NotTo(HaveOccurred())
			policy := &admissionregistrationv1.ValidatingAdmissionPolicy{}
			Expect(yaml.UnmarshalStrict(raw, policy)).To(Succeed())
			Expect(adminClient.Create(ctx, policy)).To(Succeed())
			Expect(adminClient.Create(ctx, &admissionregistrationv1.ValidatingAdmissionPolicyBinding{
				ObjectMeta: metav1.ObjectMeta{Name: policy.Name},
				Spec: admissionregistrationv1.ValidatingAdmissionPolicyBindingSpec{
					PolicyName:        policy.Name,
					ValidationActions: []admissionregistrationv1.ValidationAction{admissionregistrationv1.Deny},
				},
			})).To(Succeed())
			DeferCleanup(func() {
				Expect(adminClient.Delete(ctx, &admissionregistrationv1.ValidatingAdmissionPolicyBinding{
					ObjectMeta: metav1.ObjectMeta{Name: policy.Name},
				})).To(Succeed())
				Expect(adminClient.Delete(ctx, policy)).To(Succeed())
			})
		}

		poolEditor = userClient("pool-editor", prefectRule("prefectservers", "prefectworkpools"))
		poolDeployer = userClient("pool-deployer", prefectRule("prefectservers", "prefectworkpools"),
			rbacv1.PolicyRule{APIGroups: []string{"apps"}, Resources: []string{"deployments"}, Verbs: []string{"create"}})
		serverRefEditor = userClient("server-ref-editor",
			prefectRule("prefectworkqueues", "prefectdeployments", "prefectautomations"))
		serverRefReader = userClient("server-ref-reader",
			prefectRule("prefectworkqueues", "prefectdeployments", "prefectautomations"),
			rbacv1.PolicyRule{APIGroups: []string{""}, Resources: []string{"secrets"},
				ResourceNames: []string{"prefect-api-key"}, Verbs: []string{"get"}},
			rbacv1.PolicyRule{APIGroups: []string{""}, Resources: []string{"configmaps"},
				ResourceNames: []string{"prefect-config"}, Verbs: []string{"get"}})

		By("waiting until the API server enforces both policies")
		Eventually(func() error {
			return poolEditor.Create(ctx, workPool("probe"), client.DryRunAll)
		}).Should(Satisfy(apierrors.IsForbidden))
		Eventually(func() error {
			return serverRefEditor.Create(ctx, workQueue("probe", secretKey("prefect-api-key")), client.DryRunAll)
		}).Should(Satisfy(apierrors.IsForbidden))
	})

	Context("For PrefectServer and PrefectWorkPool", func() {
		It("Denies a create by a requester who can't create Deployments", func() {
			expectDenied(poolEditor.Create(ctx, workPool("editor-pool")),
				"the operator creates a Deployment from each PrefectWorkPool, so writing its spec requires "+
					"permission to create Deployments in namespace admission-policy")
			expectDenied(poolEditor.Create(ctx, server("editor-server")),
				"the operator creates a Deployment from each PrefectServer")
		})

		It("Allows a create by a requester who can create Deployments", func() {
			Expect(poolDeployer.Create(ctx, workPool("deployer-pool"))).To(Succeed())
			Expect(poolDeployer.Create(ctx, server("deployer-server"))).To(Succeed())
		})

		It("Denies a spec change by a requester who can't create Deployments", func() {
			pool := workPool("admin-pool")
			Expect(adminClient.Create(ctx, pool)).To(Succeed())

			pool.Spec.ExtraContainers = []corev1.Container{{Name: "extra", Image: "busybox"}}
			expectDenied(poolEditor.Update(ctx, pool), "requires permission to create Deployments")
		})

		It("Allows a change that leaves the spec alone", func() {
			pool := workPool("labeled-pool")
			Expect(adminClient.Create(ctx, pool)).To(Succeed())

			pool.Labels = map[string]string{"team": "data"}
			Expect(poolEditor.Update(ctx, pool)).To(Succeed())
		})
	})

	Context("For resources with spec.server", func() {
		DescribeTable("Denies a reference to a Secret the requester can't get, and allows one it can",
			func(newObject func(string, *prefectiov1.APIKeySpec) client.Object, name string) {
				expectDenied(serverRefEditor.Create(ctx, newObject(name, secretKey("prefect-api-key"))),
					"spec.server.apiKey.valueFrom.secretKeyRef names Secret prefect-api-key, "+
						"which you do not have permission to get in namespace admission-policy")
				Expect(serverRefReader.Create(ctx, newObject(name, secretKey("prefect-api-key")))).To(Succeed())
			},
			Entry("PrefectWorkQueue", workQueue, "secret-queue"),
			Entry("PrefectDeployment", deployment, "secret-deployment"),
			Entry("PrefectAutomation", automation, "secret-automation"),
		)

		It("Checks the Secret by name", func() {
			expectDenied(serverRefReader.Create(ctx, workQueue("other-secret-queue", secretKey("other-secret"))),
				"names Secret other-secret")
		})

		It("Denies a reference to a ConfigMap the requester can't get, and allows one it can", func() {
			expectDenied(serverRefEditor.Create(ctx, workQueue("configmap-queue", configMapKey("prefect-config"))),
				"spec.server.apiKey.valueFrom.configMapKeyRef names ConfigMap prefect-config")
			Expect(serverRefReader.Create(ctx, workQueue("configmap-queue", configMapKey("prefect-config")))).To(Succeed())
		})

		It("Denies an auth Secret the requester can't get, and allows one it can", func() {
			authQueue := func() client.Object {
				queue := workQueue("auth-queue", nil).(*prefectiov1.PrefectWorkQueue)
				ref := corev1.SecretKeySelector{Key: "auth"}
				ref.Name = "prefect-api-key"
				queue.Spec.Server = prefectiov1.PrefectServerReference{
					Name: "prefect", Auth: &prefectiov1.ServerAuth{SecretKeyRef: ref},
				}
				return queue
			}
			expectDenied(serverRefEditor.Create(ctx, authQueue()),
				"spec.server.auth.secretKeyRef names Secret prefect-api-key, "+
					"which you do not have permission to get in namespace admission-policy")
			Expect(serverRefReader.Create(ctx, authQueue())).To(Succeed())
		})

		It("Allows a server reference without a Secret or ConfigMap", func() {
			Expect(serverRefEditor.Create(ctx, workQueue("no-key-queue", nil))).To(Succeed())
			Expect(serverRefEditor.Create(ctx, workQueue("literal-key-queue",
				&prefectiov1.APIKeySpec{Value: new("pnu_literal")}))).To(Succeed())
		})

		It("Checks any spec change", func() {
			queue := workQueue("admin-queue", secretKey("prefect-api-key")).(*prefectiov1.PrefectWorkQueue)
			Expect(adminClient.Create(ctx, queue)).To(Succeed())

			queue.Spec.ConcurrencyLimit = new(int32(2))
			expectDenied(serverRefEditor.Update(ctx, queue), "names Secret prefect-api-key")
			Expect(serverRefReader.Update(ctx, queue)).To(Succeed())

			queue.Spec.Server.RemoteAPIURL = new("https://prefect.example.com")
			expectDenied(serverRefEditor.Update(ctx, queue), "names Secret prefect-api-key")
		})

		It("Allows a change that leaves the spec alone", func() {
			deploy := deployment("labeled-deployment", secretKey("prefect-api-key"))
			Expect(adminClient.Create(ctx, deploy)).To(Succeed())

			deploy.SetLabels(map[string]string{"team": "data"})
			Expect(serverRefEditor.Update(ctx, deploy)).To(Succeed())
		})
	})
})
