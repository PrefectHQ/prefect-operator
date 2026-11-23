package main

import (
	"flag"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/rest"
	kubeconfig "sigs.k8s.io/controller-runtime/pkg/client/config"

	"github.com/PrefectHQ/prefect-operator/internal/portforward"
)

var _ = Describe("clusterConfig outside the cluster", func() {
	BeforeEach(func() {
		if _, err := rest.InClusterConfig(); err == nil {
			Skip("clusterConfig pins a context only outside the cluster")
		}
	})

	It("builds the REST config and the port-forward settings from the same context", func() {
		path := filepath.Join(GinkgoT().TempDir(), "config")
		Expect(os.WriteFile(path, []byte(`apiVersion: v1
kind: Config
clusters:
- name: local
  cluster: {server: "https://127.0.0.1:6443"}
- name: production
  cluster: {server: "https://203.0.113.10"}
users:
- name: someone
  user: {token: not-a-real-token}
contexts:
- name: local
  context: {cluster: local, user: someone}
- name: production
  context: {cluster: production, user: someone}
current-context: local
`), 0o600)).To(Succeed())

		kubeconfigFlag := flag.Lookup(kubeconfig.KubeconfigFlagName)
		previous := kubeconfigFlag.Value.String()
		Expect(kubeconfigFlag.Value.Set(path)).To(Succeed())
		DeferCleanup(func() { Expect(kubeconfigFlag.Value.Set(previous)).To(Succeed()) })

		cfg, kubectl, err := clusterConfig()
		Expect(err).NotTo(HaveOccurred())
		Expect(cfg.Host).To(Equal("https://127.0.0.1:6443"))
		Expect(kubectl).To(Equal(portforward.Kubectl{Kubeconfig: path, Context: "local"}))
	})
})
