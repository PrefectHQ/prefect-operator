package main

import (
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

// chartFiles is the chart directory that holds the generated RBAC, the CRDs
// and the admission policies.
var chartFiles = filepath.Join("..", "deploy", "charts", "prefect-operator", "files")

var (
	testEnv     *envtest.Environment
	cfg         *rest.Config
	adminClient client.Client
)

func TestMain(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Manager Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	if os.Getenv("KUBEBUILDER_ASSETS") == "" {
		Fail("KUBEBUILDER_ASSETS is not set. Run `make test`, or set it the way the Makefile's test target does.")
	}
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join(chartFiles, "crds")},
		ErrorIfCRDPathMissing: true,
	}
	var err error
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())

	adminClient, err = client.New(cfg, client.Options{Scheme: scheme})
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
	Expect(testEnv.Stop()).To(Succeed())
})

// addUser adds a user to the test API server and returns a config that
// authenticates as that user. The user has no RBAC until a test binds some.
func addUser(name string) *rest.Config {
	user, err := testEnv.AddUser(envtest.User{Name: name}, cfg)
	Expect(err).NotTo(HaveOccurred())
	return user.Config()
}
