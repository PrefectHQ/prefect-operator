package prefect

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	prefectiov1 "github.com/PrefectHQ/prefect-operator/api/v1"
	"github.com/PrefectHQ/prefect-operator/internal/portforward"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// fakeKubectl is a kubectl stub for `kubectl port-forward ... svc/<name> ...`.
// It logs its arguments, prints the "Forwarding from" line for the port in
// FAKE_PORT_<name>, and sleeps until it is killed. With FAKE_KUBECTL_FAIL set,
// it exits with an error instead.
const fakeKubectl = `#!/bin/sh
echo "$@" >> "$FAKE_KUBECTL_LOG"
if [ -n "$FAKE_KUBECTL_FAIL" ]; then
	echo "error: $FAKE_KUBECTL_FAIL" >&2
	exit 1
fi
for arg; do
	case "$arg" in svc/*) name="${arg#svc/}" ;; esac
done
eval port="\$FAKE_PORT_$name"
echo "Forwarding from 127.0.0.1:$port -> 4200"
exec sleep 600
`

// recordingServer is a Prefect API stand-in that records the API keys it
// receives.
type recordingServer struct {
	*httptest.Server
	mu   sync.Mutex
	keys []string
}

func newRecordingServer() *recordingServer {
	s := &recordingServer{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/health" {
			s.mu.Lock()
			s.keys = append(s.keys, r.Header.Get("Authorization"))
			s.mu.Unlock()
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"name": "pool", "type": "process"}`))
	}))
	return s
}

func (s *recordingServer) receivedKeys() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.keys...)
}

func (s *recordingServer) port() string {
	u, err := url.Parse(s.URL)
	Expect(err).NotTo(HaveOccurred())
	return u.Port()
}

var _ = Describe("Port-forwarding to an in-cluster server", func() {
	var (
		ctx     context.Context
		logger  logr.Logger
		logFile string
	)

	BeforeEach(func() {
		if isRunningInCluster() {
			Skip("the client port-forwards only outside the cluster")
		}
		ctx = context.Background()
		logger = logr.Discard()

		dir := GinkgoT().TempDir()
		Expect(os.WriteFile(filepath.Join(dir, "kubectl"), []byte(fakeKubectl), 0o755)).To(Succeed())
		logFile = filepath.Join(dir, "kubectl.log")
		GinkgoT().Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
		GinkgoT().Setenv("FAKE_KUBECTL_LOG", logFile)
	})

	kubectlCalls := func() []string {
		data, err := os.ReadFile(logFile)
		if os.IsNotExist(err) {
			return nil
		}
		Expect(err).NotTo(HaveOccurred())
		return strings.Split(strings.TrimSpace(string(data)), "\n")
	}

	It("forwards to the Service in the fallback namespace when the reference has no namespace", func() {
		server := newRecordingServer()
		DeferCleanup(server.Close)
		GinkgoT().Setenv("FAKE_PORT_alpha", server.port())

		client, err := NewClientFromServerReference(ctx,
			&prefectiov1.PrefectServerReference{Name: "alpha"}, "key-a", "team-a", logger)
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(client.Close)

		Expect(kubectlCalls()).To(ConsistOf(
			"port-forward --namespace team-a --address 127.0.0.1 svc/alpha :4200"))
		Expect(client.BaseURL).To(Equal("http://127.0.0.1:" + server.port() + "/api"))
	})

	It("sends each client's API key only through its own forward", func() {
		alpha := newRecordingServer()
		DeferCleanup(alpha.Close)
		beta := newRecordingServer()
		DeferCleanup(beta.Close)
		GinkgoT().Setenv("FAKE_PORT_alpha", alpha.port())
		GinkgoT().Setenv("FAKE_PORT_beta", beta.port())

		alphaClient, err := NewClientFromServerReference(ctx,
			&prefectiov1.PrefectServerReference{Name: "alpha", Namespace: "team-a"}, "key-a", "team-a", logger)
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(alphaClient.Close)
		betaClient, err := NewClientFromServerReference(ctx,
			&prefectiov1.PrefectServerReference{Name: "beta", Namespace: "team-b"}, "key-b", "team-b", logger)
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(betaClient.Close)

		_, err = betaClient.GetWorkPool(ctx, "pool")
		Expect(err).NotTo(HaveOccurred())

		Expect(beta.receivedKeys()).To(Equal([]string{"Bearer key-b"}))
		Expect(alpha.receivedKeys()).To(BeEmpty())
	})

	It("refuses requests after the client is closed", func() {
		server := newRecordingServer()
		DeferCleanup(server.Close)
		GinkgoT().Setenv("FAKE_PORT_alpha", server.port())

		client, err := NewClientFromServerReference(ctx,
			&prefectiov1.PrefectServerReference{Name: "alpha", Namespace: "team-a"}, "key-a", "team-a", logger)
		Expect(err).NotTo(HaveOccurred())
		client.Close()

		_, err = client.GetWorkPool(ctx, "pool")
		Expect(err).To(MatchError(ContainSubstring("port-forward to the Prefect server")))
		Expect(server.receivedKeys()).To(BeEmpty())
	})

	It("stops kubectl when the context ends", func() {
		server := newRecordingServer()
		DeferCleanup(server.Close)
		GinkgoT().Setenv("FAKE_PORT_alpha", server.port())

		clientCtx, cancel := context.WithCancel(ctx)
		client, err := NewClientFromServerReference(clientCtx,
			&prefectiov1.PrefectServerReference{Name: "alpha", Namespace: "team-a"}, "key-a", "team-a", logger)
		Expect(err).NotTo(HaveOccurred())

		cancel()
		Eventually(client.portForward.Done()).WithTimeout(5 * time.Second).Should(BeClosed())
	})

	It("returns kubectl's error when kubectl exits before forwarding", func() {
		GinkgoT().Setenv("FAKE_KUBECTL_FAIL", `services "alpha" not found`)

		_, err := NewClientFromServerReference(ctx,
			&prefectiov1.PrefectServerReference{Name: "alpha", Namespace: "team-a"}, "key-a", "team-a", logger)
		Expect(err).To(MatchError(ContainSubstring(`services "alpha" not found`)))
	})

	It("refuses to forward without a namespace", func() {
		_, err := portforward.Start(ctx, "", "alpha", 4200)
		Expect(err).To(MatchError(ContainSubstring("needs a namespace")))
		Expect(kubectlCalls()).To(BeEmpty())
	})
})

var _ = Describe("Redirects", func() {
	It("does not follow a redirect or send the API key to its URL", func() {
		var redirectedKeys []string
		var mu sync.Mutex
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/elsewhere" {
				mu.Lock()
				redirectedKeys = append(redirectedKeys, r.Header.Get("Authorization"))
				mu.Unlock()
				return
			}
			http.Redirect(w, r, "/elsewhere", http.StatusTemporaryRedirect)
		}))
		DeferCleanup(server.Close)

		client := NewClient(server.URL+"/api", "key-a", logr.Discard())
		_, err := client.GetWorkPool(context.Background(), "pool")

		Expect(err).To(MatchError(ContainSubstring("does not follow redirects")))
		mu.Lock()
		defer mu.Unlock()
		Expect(redirectedKeys).To(BeEmpty())
	})
})
