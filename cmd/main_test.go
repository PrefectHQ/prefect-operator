package main

import (
	"context"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/yaml"

	prefectiov1 "github.com/PrefectHQ/prefect-operator/api/v1"
)

// These tests run as a user bound to the ClusterRole that the chart renders
// from files/role.yaml, with a manager built from managerOptions. They fail
// if the generated RBAC and the options that main passes to the manager stop
// matching.
var _ = Describe("Reading an API key Secret with the operator's RBAC", Ordered, func() {
	const (
		namespaceName = "apikey-rbac"
		userName      = "prefect-operator"
	)

	var (
		ctx         context.Context
		operatorCfg *rest.Config
		serverRef   *prefectiov1.PrefectServerReference
	)

	BeforeAll(func() {
		ctx = context.Background()

		raw, err := os.ReadFile(filepath.Join(chartFiles, "role.yaml"))
		Expect(err).NotTo(HaveOccurred())
		role := &rbacv1.ClusterRole{}
		Expect(yaml.Unmarshal(raw, role)).To(Succeed())
		Expect(adminClient.Create(ctx, role)).To(Succeed())
		Expect(adminClient.Create(ctx, &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: role.Name},
			RoleRef:    rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: role.Name},
			Subjects:   []rbacv1.Subject{{APIGroup: rbacv1.GroupName, Kind: rbacv1.UserKind, Name: userName}},
		})).To(Succeed())

		operatorCfg = addUser(userName)

		Expect(adminClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespaceName}})).To(Succeed())
		Expect(adminClient.Create(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "prefect-api-key",
				Namespace:   namespaceName,
				Annotations: map[string]string{prefectiov1.AnnotationAllowedAPIURLs: "https://api.prefect.cloud"},
			},
			Data: map[string][]byte{"api-key": []byte("test-api-key-value")},
		})).To(Succeed())

		serverRef = &prefectiov1.PrefectServerReference{
			RemoteAPIURL: new("https://api.prefect.cloud"),
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
	// client.
	startManager := func(opts ctrl.Options) client.Client {
		opts.Metrics = metricsserver.Options{BindAddress: "0"}
		opts.HealthProbeBindAddress = "0"
		mgr, err := ctrl.NewManager(operatorCfg, opts)
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
		operatorClient, err := client.New(operatorCfg, client.Options{Scheme: scheme})
		Expect(err).NotTo(HaveOccurred())

		key := client.ObjectKey{Namespace: namespaceName, Name: "prefect-api-key"}
		Expect(operatorClient.Get(ctx, key, &corev1.Secret{})).To(Succeed())
		err = operatorClient.List(ctx, &corev1.SecretList{}, client.InNamespace(namespaceName))
		Expect(apierrors.IsForbidden(err)).To(BeTrue(), "expected Forbidden, got %v", err)
	})

	It("Reads the API key through the client of a manager built from managerOptions", func() {
		mgrClient := startManager(managerOptions(0))

		readCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		apiKey, err := serverRef.GetAPIKey(readCtx, mgrClient, namespaceName)
		Expect(err).NotTo(HaveOccurred())
		Expect(apiKey).To(Equal("test-api-key-value"))
	})

	It("Blocks until its context ends when the client caches Secrets", func() {
		opts := managerOptions(0)
		opts.Client = client.Options{}
		mgrClient := startManager(opts)

		readCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		_, err := serverRef.GetAPIKey(readCtx, mgrClient, namespaceName)
		Expect(err).To(HaveOccurred())
		Expect(readCtx.Err()).To(MatchError(context.DeadlineExceeded))
	})

	It("Passes the reconcile timeout to every controller", func() {
		Expect(managerOptions(90 * time.Second).Controller.ReconciliationTimeout).To(Equal(90 * time.Second))
	})
})
