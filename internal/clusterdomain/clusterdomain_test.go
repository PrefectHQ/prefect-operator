package clusterdomain

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Resolve", func() {
	resolvConf := func(content string) string {
		path := filepath.Join(GinkgoT().TempDir(), "resolv.conf")
		Expect(os.WriteFile(path, []byte(content), 0o600)).To(Succeed())
		return path
	}

	DescribeTable("without a domain",
		func(content, expected string) {
			Expect(Resolve("", resolvConf(content))).To(Equal(expected))
		},
		Entry("reads the domain from a pod's search domains",
			"nameserver 10.96.0.10\n"+
				"search prefect-system.svc.k8s.example.com svc.k8s.example.com k8s.example.com corp.example\n"+
				"options ndots:5\n",
			"k8s.example.com"),
		Entry("removes trailing dots", "search ns.svc.k8s.example.com. svc.k8s.example.com. k8s.example.com.\n", "k8s.example.com"),
		Entry("reads the domain in a namespace named svc",
			"search svc.svc.k8s.example.com svc.k8s.example.com k8s.example.com\n", "k8s.example.com"),
		Entry("uses the last search line",
			"search ns.svc.first.example svc.first.example first.example\n"+
				"search ns.svc.last.example svc.last.example last.example\n",
			"last.example"),
		Entry("uses the default when the last search line names no cluster domain",
			"search ns.svc.first.example svc.first.example first.example\nsearch corp.example\n", "cluster.local"),
		Entry("ignores search domains that are not the cluster's",
			"search corp.example svc.corp.example mysvc.corp.example\n", "cluster.local"),
		Entry("ignores search domains without the namespace's",
			"search other.example svc.corp.example corp.example\n", "cluster.local"),
		Entry("ignores comments", "# search ns.svc.k8s.example.com svc.k8s.example.com k8s.example.com\n", "cluster.local"),
		Entry("uses the default for an empty file", "", "cluster.local"),
	)

	It("uses the default when the file does not exist", func() {
		Expect(Resolve("", filepath.Join(GinkgoT().TempDir(), "missing"))).To(Equal("cluster.local"))
	})

	It("prefers the given domain", func() {
		Expect(Resolve("flag.example", resolvConf("search ns.svc.k8s.example.com svc.k8s.example.com k8s.example.com\n"))).To(Equal("flag.example"))
		Expect(Resolve("flag.example.", resolvConf(""))).To(Equal("flag.example"))
	})

	It("returns an error for a domain that is not a DNS name", func() {
		_, err := Resolve("example.com/x?", resolvConf(""))
		Expect(err).To(MatchError(ContainSubstring(`the cluster domain "example.com/x?" is not a DNS name`)))
	})

	It("returns an error when it can't read the file", func() {
		_, err := Resolve("", GinkgoT().TempDir())
		Expect(err).To(HaveOccurred())
	})
})
