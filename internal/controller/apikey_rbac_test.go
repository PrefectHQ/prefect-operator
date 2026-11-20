package controller

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/yaml"

	prefectiov1 "github.com/PrefectHQ/prefect-operator/api/v1"
	"github.com/PrefectHQ/prefect-operator/internal/prefect"
)

// These tests run as a user bound to the ClusterRole that the chart renders
// from files/role.yaml, so they fail if the generated RBAC and the manager's
// client options stop matching.
var _ = Describe("Reading an API key Secret with the operator's RBAC", Ordered, func() {
	var (
		ctx           context.Context
		namespaceName string
		operatorCfg   *rest.Config
		serverRef     *prefectiov1.PrefectServerReference
	)

	BeforeAll(func() {
		ctx = context.Background()
		suffix := time.Now().UnixNano()
		namespaceName = fmt.Sprintf("apikey-rbac-%d", suffix)
		userName := fmt.Sprintf("prefect-operator-%d", suffix)

		raw, err := os.ReadFile(filepath.Join("..", "..", "deploy", "charts", "prefect-operator", "files", "role.yaml"))
		Expect(err).NotTo(HaveOccurred())
		role := &rbacv1.ClusterRole{}
		Expect(yaml.Unmarshal(raw, role)).To(Succeed())
		role.Name = userName
		Expect(k8sClient.Create(ctx, role)).To(Succeed())
		DeferCleanup(func() { Expect(k8sClient.Delete(ctx, role)).To(Succeed()) })

		binding := &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: userName},
			RoleRef:    rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: role.Name},
			Subjects:   []rbacv1.Subject{{APIGroup: rbacv1.GroupName, Kind: rbacv1.UserKind, Name: userName}},
		}
		Expect(k8sClient.Create(ctx, binding)).To(Succeed())
		DeferCleanup(func() { Expect(k8sClient.Delete(ctx, binding)).To(Succeed()) })

		user, err := testEnv.AddUser(envtest.User{Name: userName}, cfg)
		Expect(err).NotTo(HaveOccurred())
		operatorCfg = user.Config()

		Expect(k8sClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespaceName}})).To(Succeed())
		Expect(k8sClient.Create(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "prefect-api-key",
				Namespace: namespaceName,
				Labels:    map[string]string{prefectiov1.LabelAPIKey: "true"},
			},
			Data: map[string][]byte{"api-key": []byte("test-api-key-value")},
		})).To(Succeed())

		serverRef = &prefectiov1.PrefectServerReference{
			APIKey: &prefectiov1.APIKeySpec{
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "prefect-api-key"},
						Key:                  "api-key",
					},
				},
			},
		}
	})

	// startManager starts a manager as the operator's user and returns its
	// client, which reads from the manager's cache except for clientOpts.
	startManager := func(clientOpts client.Options) client.Client {
		mgr, err := ctrl.NewManager(operatorCfg, ctrl.Options{
			Scheme:                 scheme.Scheme,
			Metrics:                metricsserver.Options{BindAddress: "0"},
			HealthProbeBindAddress: "0",
			Client:                 clientOpts,
		})
		Expect(err).NotTo(HaveOccurred())

		mgrCtx, cancel := context.WithCancel(ctx)
		DeferCleanup(cancel)
		go func() {
			defer GinkgoRecover()
			Expect(mgr.Start(mgrCtx)).To(Succeed())
		}()
		Expect(mgr.GetCache().WaitForCacheSync(mgrCtx)).To(BeTrue())
		return mgr.GetClient()
	}

	It("Grants get on Secrets but not list or watch", func() {
		operatorClient, err := client.New(operatorCfg, client.Options{Scheme: scheme.Scheme})
		Expect(err).NotTo(HaveOccurred())

		Expect(operatorClient.Get(ctx, client.ObjectKey{Namespace: namespaceName, Name: "prefect-api-key"}, &corev1.Secret{})).To(Succeed())
		err = operatorClient.List(ctx, &corev1.SecretList{}, client.InNamespace(namespaceName))
		Expect(apierrors.IsForbidden(err)).To(BeTrue(), "expected Forbidden, got %v", err)
	})

	It("Reads the API key through a client that does not cache Secrets", func() {
		mgrClient := startManager(client.Options{
			Cache: &client.CacheOptions{DisableFor: prefect.UncachedObjects()},
		})

		readCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		apiKey, err := serverRef.GetAPIKey(readCtx, mgrClient, namespaceName)
		Expect(err).NotTo(HaveOccurred())
		Expect(apiKey).To(Equal("test-api-key-value"))
	})

	It("Blocks until its context ends when the client caches Secrets", func() {
		mgrClient := startManager(client.Options{})

		readCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		_, err := serverRef.GetAPIKey(readCtx, mgrClient, namespaceName)
		Expect(err).To(HaveOccurred())
		Expect(readCtx.Err()).To(MatchError(context.DeadlineExceeded))
	})
})
