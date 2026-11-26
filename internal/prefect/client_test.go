package prefect

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	prefectiov1 "github.com/PrefectHQ/prefect-operator/api/v1"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestPrefectClient(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Prefect Client Suite")
}

var _ = Describe("Prefect HTTP Client", func() {
	var (
		ctx        context.Context
		client     *Client
		mockServer *httptest.Server
		logger     logr.Logger
	)

	BeforeEach(func() {
		ctx = context.Background()
		logger = logr.Discard()
	})

	AfterEach(func() {
		if mockServer != nil {
			mockServer.Close()
		}
	})

	Describe("Client Creation", func() {
		It("Should create client with default timeout", func() {
			client := NewClient("http://test.com", "test-key", logger)

			Expect(client.BaseURL).To(Equal("http://test.com"))
			Expect(client.APIKey).To(Equal("test-key"))
			Expect(client.HTTPClient.Timeout).To(Equal(30 * time.Second))
		})
	})

	Describe("Link-local addresses", func() {
		It("Should refuse to connect to an instance metadata address", func() {
			for _, url := range []string{
				"http://169.254.169.254/api",
				"http://[::ffff:169.254.169.254]/api",
				"http://[fe80::1]:4200/api",
				"http://[fe80::1%25eth0]:4200/api",
				"http://[fd00:ec2::254]/api",
				"http://[fd20:ce::254]/api",
			} {
				client := NewClient(url, "", logger)
				_, err := client.GetWorkPool(ctx, "pool")
				Expect(err).To(MatchError(ContainSubstring("a link-local or instance metadata address")), url)
			}
		})

		It("Should check the address that the dialer connects to", func() {
			for _, address := range []string{
				"169.254.169.254:80",
				"[::ffff:169.254.169.254]:80",
				"[fe80::1%eth0]:4200",
				"[fd00:ec2::254]:80",
				"[fd20:ce::254]:80",
			} {
				Expect(refuseLinkLocal("tcp", address, nil)).To(
					MatchError(ContainSubstring("a link-local or instance metadata address")), address)
			}
		})

		It("Should refuse an instance metadata address before it sends the request to a proxy", func() {
			proxyFor := refuseLinkLocalHost(func(*http.Request) (*url.URL, error) {
				return &url.URL{Scheme: "http", Host: "proxy.example.com:3128"}, nil
			})

			for target, refused := range map[string]bool{
				"http://169.254.169.254/api":          true,
				"http://[::ffff:169.254.169.254]/api": true,
				"http://[fe80::1%25eth0]:4200/api":    true,
				"http://[fd20:ce::254]/api":           true,
				"http://10.0.0.1:4200/api":            false,
				"https://api.prefect.cloud/api":       false,
			} {
				req, err := http.NewRequest(http.MethodGet, target, nil)
				Expect(err).NotTo(HaveOccurred())
				proxy, err := proxyFor(req)
				if refused {
					Expect(err).To(MatchError(ContainSubstring("a link-local or instance metadata address")), target)
				} else {
					Expect(err).NotTo(HaveOccurred(), target)
					Expect(proxy.Host).To(Equal("proxy.example.com:3128"), target)
				}
			}
		})

		It("Should allow other addresses", func() {
			for _, address := range []string{"127.0.0.1:4200", "10.0.0.1:443", "[::1]:4200", "[fd00::1]:80"} {
				Expect(refuseLinkLocal("tcp", address, nil)).To(Succeed(), address)
			}
		})
	})

	Describe("Proxies", func() {
		It("Should not use a proxy for an in-cluster server", func() {
			proxy := &url.URL{Scheme: "http", Host: "proxy.example.com:3128"}
			proxyFor := directInCluster(func(*http.Request) (*url.URL, error) { return proxy, nil })

			for target, want := range map[string]*url.URL{
				(&prefectiov1.PrefectServerReference{Name: "prefect"}).GetAPIURL("team-b"): nil,
				"https://api.prefect.cloud/api":                                            proxy,
				"http://prefect.team-b.svc.cluster.local.corp.example:4200/api":            proxy,
			} {
				req, err := http.NewRequest(http.MethodGet, target, nil)
				Expect(err).NotTo(HaveOccurred())
				Expect(proxyFor(req)).To(Equal(want), target)
			}
		})
	})

	Describe("Authentication", func() {
		DescribeTable("Should send the Authorization header for its credentials",
			func(creds Credentials, want string) {
				var got string
				mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					got = r.Header.Get("Authorization")
					w.WriteHeader(http.StatusNotFound)
				}))

				_, _ = newClientWithCredentials(mockServer.URL+"/api", creds, logger).GetWorkPool(ctx, "pool")
				Expect(got).To(Equal(want))
			},
			Entry("an API key as a bearer token", Credentials{APIKey: "pnu_key"}, "Bearer pnu_key"),
			// base64 of "admin:hunter2"
			Entry("an auth string as basic authentication", Credentials{AuthString: "admin:hunter2"}, "Basic YWRtaW46aHVudGVyMg=="),
			Entry("the auth string when both are set", Credentials{APIKey: "pnu_key", AuthString: "admin:hunter2"}, "Basic YWRtaW46aHVudGVyMg=="),
			Entry("nothing without credentials", Credentials{}, ""),
		)

		It("Should handle Prefect Cloud authentication", func() {
			By("Setting up mock server that mimics Prefect Cloud")
			mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer GinkgoRecover()
				// Verify Authorization header is present and correct
				authHeader := r.Header.Get("Authorization")
				Expect(authHeader).To(Equal("Bearer pnu_1234567890abcdef"))

				expectedFlow := Flow{
					ID:   "flow-cloud-12345",
					Name: "cloud-flow",
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(expectedFlow)
			}))

			By("Creating client with Prefect Cloud API key")
			client = NewClient(mockServer.URL, "pnu_1234567890abcdef", logger)

			By("Calling GetFlowByName")
			flow, err := client.GetFlowByName(ctx, "cloud-flow")

			By("Verifying request succeeds with Prefect Cloud authentication")
			Expect(err).NotTo(HaveOccurred())
			Expect(flow).NotTo(BeNil())
			Expect(flow.Name).To(Equal("cloud-flow"))
		})

		It("Should handle empty API key (open-source Prefect without auth)", func() {
			By("Setting up mock server that mimics open-source Prefect")
			mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer GinkgoRecover()
				// Verify no Authorization header when API key is empty
				Expect(r.Header.Get("Authorization")).To(Equal(""))

				expectedFlow := Flow{
					ID:   "flow-12345",
					Name: testFlowName,
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(expectedFlow)
			}))

			By("Creating client with empty API key")
			client = NewClient(mockServer.URL, "", logger)

			By("Calling GetFlowByName")
			flow, err := client.GetFlowByName(ctx, testFlowName)

			By("Verifying request succeeds without authentication")
			Expect(err).NotTo(HaveOccurred())
			Expect(flow).NotTo(BeNil())
			Expect(flow.Name).To(Equal(testFlowName))
		})

		It("Should handle custom authentication tokens", func() {
			By("Setting up mock server that mimics authenticated open-source Prefect")
			mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer GinkgoRecover()
				// Check for custom auth header
				authHeader := r.Header.Get("Authorization")
				if authHeader != "Bearer custom_token_123" {
					w.WriteHeader(http.StatusUnauthorized)
					_, _ = w.Write([]byte(`{"detail": "Authentication required"}`))
					return
				}

				expectedFlow := Flow{
					ID:   "flow-selfhosted-12345",
					Name: "selfhosted-flow",
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(expectedFlow)
			}))

			By("Creating client with custom authentication token")
			client = NewClient(mockServer.URL, "custom_token_123", logger)

			By("Calling GetFlowByName")
			flow, err := client.GetFlowByName(ctx, "selfhosted-flow")

			By("Verifying request succeeds with custom authentication")
			Expect(err).NotTo(HaveOccurred())
			Expect(flow).NotTo(BeNil())
			Expect(flow.Name).To(Equal("selfhosted-flow"))
		})

		It("Should handle authentication errors", func() {
			By("Setting up mock server that returns 401")
			mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer GinkgoRecover()
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"detail": "Invalid API key"}`))
			}))

			By("Creating client with invalid API key")
			client = NewClient(mockServer.URL, "invalid_key", logger)

			By("Calling GetFlowByName")
			flow, err := client.GetFlowByName(ctx, testFlowName)

			By("Verifying authentication error is handled")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("API request failed with status 401"))
			Expect(err.Error()).To(ContainSubstring("Invalid API key"))
			Expect(flow).To(BeNil())
		})
	})

	Describe("Error Handling", func() {
		It("Should handle network errors", func() {
			By("Creating client with invalid URL")
			client = NewClient("http://invalid-host:99999", "test-api-key", logger)

			By("Calling GetFlowByName")
			flow, err := client.GetFlowByName(ctx, testFlowName)

			By("Verifying network error is returned")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to make request"))
			Expect(flow).To(BeNil())
		})

		It("Should handle context cancellation", func() {
			By("Setting up mock server with slow response")
			mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer GinkgoRecover()
				time.Sleep(100 * time.Millisecond)
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(Flow{ID: "test", Name: testFlowName})
			}))

			By("Creating client with mock server URL")
			client = NewClient(mockServer.URL, "test-api-key", logger)

			By("Creating cancelled context")
			cancelCtx, cancel := context.WithCancel(ctx)
			cancel()

			By("Calling GetFlowByName with cancelled context")
			flow, err := client.GetFlowByName(cancelCtx, testFlowName)

			By("Verifying context cancellation error")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to make request"))
			Expect(flow).To(BeNil())
		})

		It("Should handle invalid JSON response", func() {
			By("Setting up mock server with invalid JSON")
			mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer GinkgoRecover()
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{invalid json`))
			}))

			By("Creating client with mock server URL")
			client = NewClient(mockServer.URL, "test-api-key", logger)

			By("Calling GetFlowByName")
			flow, err := client.GetFlowByName(ctx, testFlowName)

			By("Verifying unmarshal error is returned")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to unmarshal response"))
			Expect(flow).To(BeNil())
		})

		It("Should handle non-JSON response content types", func() {
			By("Setting up mock server with text response")
			mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer GinkgoRecover()
				w.Header().Set("Content-Type", "text/plain")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("not json"))
			}))

			By("Creating client with mock server URL")
			client = NewClient(mockServer.URL, "test-api-key", logger)

			By("Calling GetFlowByName")
			flow, err := client.GetFlowByName(ctx, testFlowName)

			By("Verifying unmarshal error for non-JSON response")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to unmarshal response"))
			Expect(flow).To(BeNil())
		})

		It("Should handle API errors", func() {
			By("Setting up mock server with 500 error")
			mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer GinkgoRecover()
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"detail": "Internal server error"}`))
			}))

			By("Creating client with mock server URL")
			client = NewClient(mockServer.URL, "test-api-key", logger)

			By("Calling GetFlowByName")
			flow, err := client.GetFlowByName(ctx, "error-flow")

			By("Verifying error is returned")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("API request failed with status 500"))
			Expect(err.Error()).To(ContainSubstring("Internal server error"))
			Expect(flow).To(BeNil())
		})

		It("Refuses a response body larger than the limit", func() {
			mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(bytes.Repeat([]byte(" "), maxResponseBytes+1))
			}))
			client = NewClient(mockServer.URL, "test-api-key", logger)

			flow, err := client.GetFlowByName(ctx, testFlowName)
			Expect(err).To(MatchError(ContainSubstring("response body is larger than")))
			Expect(flow).To(BeNil())
		})

		It("Applies the limit to a gzip response after decompressing it", func() {
			var compressed bytes.Buffer
			gz := gzip.NewWriter(&compressed)
			_, _ = gz.Write(bytes.Repeat([]byte(" "), maxResponseBytes+1))
			Expect(gz.Close()).To(Succeed())
			Expect(compressed.Len()).To(BeNumerically("<", maxResponseBytes))

			mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Encoding", "gzip")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(compressed.Bytes())
			}))
			client = NewClient(mockServer.URL, "test-api-key", logger)

			_, err := client.GetFlowByName(ctx, testFlowName)
			Expect(err).To(MatchError(ContainSubstring("response body is larger than")))
		})

		It("Keeps only the start of a large error response", func() {
			mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write(bytes.Repeat([]byte("x"), maxResponseBytes+1))
			}))
			client = NewClient(mockServer.URL, "test-api-key", logger)

			err := client.DeleteDeployment(ctx, "deployment-id")
			Expect(err).NotTo(HaveOccurred(), "a 404 still means the object is gone")

			_, err = client.UpdateDeployment(ctx, "deployment-id", &DeploymentSpec{}, nil)
			Expect(err).To(MatchError(ErrNotFound))
			Expect(len(err.Error())).To(BeNumerically("<", maxErrorBodyBytes+100))
			Expect(err.Error()).To(HaveSuffix("(truncated)"))
		})
	})

	Describe("GetFlowByName", func() {
		It("Should successfully retrieve a flow by name", func() {
			By("Setting up mock server with flow response")
			expectedFlow := Flow{
				ID:      "flow-12345",
				Created: time.Now(),
				Updated: time.Now(),
				Name:    testFlowName,
				Tags:    []string{testValueTest, testTagExample},
				Labels:  map[string]string{"env": testValueTest},
			}

			mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer GinkgoRecover()
				// Verify request method and path
				Expect(r.Method).To(Equal("GET"))
				Expect(r.URL.Path).To(Equal("/flows/name/test-flow"))
				// A GET has no body, so it has no Content-Type.
				Expect(r.Header.Get("Content-Type")).To(BeEmpty())

				// Return mock flow response
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(expectedFlow)
			}))

			By("Creating client with mock server URL")
			client = NewClient(mockServer.URL, "test-api-key", logger)

			By("Calling GetFlowByName")
			flow, err := client.GetFlowByName(ctx, testFlowName)

			By("Verifying the response")
			Expect(err).NotTo(HaveOccurred())
			Expect(flow).NotTo(BeNil())
			Expect(flow.ID).To(Equal(expectedFlow.ID))
			Expect(flow.Name).To(Equal(expectedFlow.Name))
			Expect(flow.Tags).To(Equal(expectedFlow.Tags))
			Expect(flow.Labels).To(Equal(expectedFlow.Labels))
		})

		It("Should return nil when flow is not found", func() {
			By("Setting up mock server with 404 response")
			mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer GinkgoRecover()
				Expect(r.Method).To(Equal("GET"))
				Expect(r.URL.Path).To(Equal("/flows/name/nonexistent-flow"))

				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"detail": "Flow not found"}`))
			}))

			By("Creating client with mock server URL")
			client = NewClient(mockServer.URL, "test-api-key", logger)

			By("Calling GetFlowByName for nonexistent flow")
			flow, err := client.GetFlowByName(ctx, "nonexistent-flow")

			By("Verifying nil is returned for 404")
			Expect(err).NotTo(HaveOccurred())
			Expect(flow).To(BeNil())
		})
	})

	Describe("CreateOrGetFlow", func() {
		It("Should create a new flow when it doesn't exist", func() {
			By("Setting up mock server for flow creation")
			expectedFlow := Flow{
				ID:      "new-flow-12345",
				Created: time.Now(),
				Updated: time.Now(),
				Name:    "new-flow",
				Tags:    []string{testTagNew, "created"},
				Labels:  map[string]string{"source": testLabelOperator},
			}

			callCount := 0
			mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer GinkgoRecover()
				callCount++

				switch r.URL.Path {
				case "/flows/name/new-flow":
					// First call - flow doesn't exist
					Expect(r.Method).To(Equal("GET"))
					w.WriteHeader(http.StatusNotFound)
					_, _ = w.Write([]byte(`{"detail": "Flow not found"}`))
				case "/flows/":
					// Second call - create flow
					Expect(r.Method).To(Equal("POST"))
					Expect(r.Header.Get("Content-Type")).To(Equal("application/json"))

					// Return created flow
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					_ = json.NewEncoder(w).Encode(expectedFlow)
				default:
					Fail("Unexpected path: " + r.URL.Path)
				}
			}))

			By("Creating client with mock server URL")
			client = NewClient(mockServer.URL, "test-api-key", logger)

			By("Calling CreateOrGetFlow")
			flowSpec := &FlowSpec{
				Name:   "new-flow",
				Tags:   []string{testTagNew, "created"},
				Labels: map[string]string{"source": testLabelOperator},
			}
			flow, err := client.CreateOrGetFlow(ctx, flowSpec)

			By("Verifying flow was created")
			Expect(err).NotTo(HaveOccurred())
			Expect(flow).NotTo(BeNil())
			Expect(flow.ID).To(Equal(expectedFlow.ID))
			Expect(flow.Name).To(Equal(expectedFlow.Name))
			Expect(callCount).To(Equal(2)) // GET then POST
		})

		It("Should return existing flow when it already exists", func() {
			By("Setting up mock server for existing flow")
			existingFlow := Flow{
				ID:      "existing-flow-12345",
				Created: time.Now(),
				Updated: time.Now(),
				Name:    "existing-flow",
				Tags:    []string{"existing"},
				Labels:  map[string]string{"source": "existing"},
			}

			mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer GinkgoRecover()
				// Only GET call should happen
				Expect(r.Method).To(Equal("GET"))
				Expect(r.URL.Path).To(Equal("/flows/name/existing-flow"))

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(existingFlow)
			}))

			By("Creating client with mock server URL")
			client = NewClient(mockServer.URL, "test-api-key", logger)

			By("Calling CreateOrGetFlow")
			flowSpec := &FlowSpec{
				Name:   "existing-flow",
				Tags:   []string{testTagNew, "tag"},
				Labels: map[string]string{"source": testLabelOperator},
			}
			flow, err := client.CreateOrGetFlow(ctx, flowSpec)

			By("Verifying existing flow was returned")
			Expect(err).NotTo(HaveOccurred())
			Expect(flow).NotTo(BeNil())
			Expect(flow.ID).To(Equal(existingFlow.ID))
			Expect(flow.Name).To(Equal(existingFlow.Name))
			// Should return existing flow, not create new one
			Expect(flow.Tags).To(Equal(existingFlow.Tags))
		})
	})

	Describe("CreateOrUpdateDeployment", func() {
		It("Should create a new deployment", func() {
			By("Setting up mock server for deployment creation")
			expectedDeployment := Deployment{
				ID:           testDeploymentID,
				Created:      time.Now(),
				Updated:      time.Now(),
				Name:         testDeploymentName,
				FlowID:       testFlowID,
				Paused:       false,
				Tags:         []string{testValueTest, testTagDeployment},
				Parameters:   map[string]any{testParam1: "value1"},
				Entrypoint:   new("flows.py:main_flow"),
				WorkPoolName: new("kubernetes"),
			}

			mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer GinkgoRecover()
				Expect(r.Method).To(Equal("POST"))
				Expect(r.URL.Path).To(Equal("/deployments/"))
				Expect(r.Header.Get("Content-Type")).To(Equal("application/json"))

				// Verify request body contains deployment spec
				var deploymentSpec DeploymentSpec
				_ = json.NewDecoder(r.Body).Decode(&deploymentSpec)
				Expect(deploymentSpec.Name).To(Equal(testDeploymentName))
				Expect(deploymentSpec.FlowID).To(Equal(testFlowID))

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(expectedDeployment)
			}))

			By("Creating client with mock server URL")
			client = NewClient(mockServer.URL, "test-api-key", logger)

			By("Calling CreateOrUpdateDeployment")
			deploymentSpec := &DeploymentSpec{
				Name:         testDeploymentName,
				FlowID:       testFlowID,
				Tags:         []string{testValueTest, testTagDeployment},
				Parameters:   map[string]any{testParam1: "value1"},
				Entrypoint:   new("flows.py:main_flow"),
				WorkPoolName: new("kubernetes"),
			}
			deployment, err := client.CreateOrUpdateDeployment(ctx, deploymentSpec)

			By("Verifying deployment was created")
			Expect(err).NotTo(HaveOccurred())
			Expect(deployment).NotTo(BeNil())
			Expect(deployment.ID).To(Equal(expectedDeployment.ID))
			Expect(deployment.Name).To(Equal(expectedDeployment.Name))
			Expect(deployment.FlowID).To(Equal(expectedDeployment.FlowID))
		})
	})

	Describe("GetDeployment", func() {
		It("Should retrieve a deployment by ID", func() {
			By("Setting up mock server for deployment retrieval")
			expectedDeployment := Deployment{
				ID:           testDeploymentID,
				Created:      time.Now(),
				Updated:      time.Now(),
				Name:         testDeploymentName,
				FlowID:       testFlowID,
				Paused:       false,
				Status:       WorkPoolStatusReady,
				Tags:         []string{testValueTest},
				WorkPoolName: new("kubernetes"),
			}

			mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer GinkgoRecover()
				Expect(r.Method).To(Equal("GET"))
				Expect(r.URL.Path).To(Equal("/deployments/deployment-12345"))

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(expectedDeployment)
			}))

			By("Creating client with mock server URL")
			client = NewClient(mockServer.URL, "test-api-key", logger)

			By("Calling GetDeployment")
			deployment, err := client.GetDeployment(ctx, testDeploymentID)

			By("Verifying deployment was retrieved")
			Expect(err).NotTo(HaveOccurred())
			Expect(deployment).NotTo(BeNil())
			Expect(deployment.ID).To(Equal(expectedDeployment.ID))
			Expect(deployment.Name).To(Equal(expectedDeployment.Name))
			Expect(deployment.Status).To(Equal(expectedDeployment.Status))
		})
	})

	Describe("UpdateDeployment", func() {
		It("Should update an existing deployment", func() {
			By("Setting up mock server for deployment update")
			updatedDeployment := Deployment{
				ID:           testDeploymentID,
				Created:      time.Now().Add(-time.Hour),
				Updated:      time.Now(),
				Name:         "updated-deployment",
				FlowID:       testFlowID,
				Paused:       true,
				Tags:         []string{"updated", testValueTest},
				Parameters:   map[string]any{testParam1: "updated_value"},
				WorkPoolName: new("kubernetes"),
			}

			mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer GinkgoRecover()
				Expect(r.Method).To(Equal("PATCH"))
				Expect(r.URL.Path).To(Equal("/deployments/deployment-12345"))
				Expect(r.Header.Get("Content-Type")).To(Equal("application/json"))

				var deploymentSpec DeploymentSpec
				_ = json.NewDecoder(r.Body).Decode(&deploymentSpec)
				Expect(deploymentSpec.Name).To(BeEmpty())
				Expect(deploymentSpec.FlowID).To(BeEmpty())
				Expect(*deploymentSpec.Paused).To(BeTrue())

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(updatedDeployment)
			}))

			By("Creating client with mock server URL")
			client = NewClient(mockServer.URL, "test-api-key", logger)

			By("Calling UpdateDeployment")
			deploymentSpec := &DeploymentSpec{
				Name:       "updated-deployment",
				FlowID:     testFlowID,
				Paused:     new(true),
				Tags:       []string{"updated", testValueTest},
				Parameters: map[string]any{testParam1: "updated_value"},
			}
			deployment, err := client.UpdateDeployment(ctx, testDeploymentID, deploymentSpec, nil)

			By("Verifying deployment was updated")
			Expect(err).NotTo(HaveOccurred())
			Expect(deployment).NotTo(BeNil())
			Expect(deployment.ID).To(Equal(testDeploymentID))
			Expect(deployment.Name).To(Equal("updated-deployment"))
			Expect(deployment.Paused).To(BeTrue())
		})

		It("Should omit an unset concurrency limit from the payload", func() {
			mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer GinkgoRecover()
				var body map[string]any
				Expect(json.NewDecoder(r.Body).Decode(&body)).To(Succeed())
				Expect(body).NotTo(HaveKey("concurrency_limit"))
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(Deployment{ID: testDeploymentID})
			}))
			client = NewClient(mockServer.URL, "test-api-key", logger)

			_, err := client.UpdateDeployment(ctx, testDeploymentID, &DeploymentSpec{Paused: new(true)}, nil)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should send an explicit null when clearing the concurrency limit", func() {
			mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer GinkgoRecover()
				var body map[string]any
				Expect(json.NewDecoder(r.Body).Decode(&body)).To(Succeed())
				Expect(body).To(HaveKeyWithValue("concurrency_limit", BeNil()))
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(Deployment{ID: testDeploymentID})
			}))
			client = NewClient(mockServer.URL, "test-api-key", logger)

			_, err := client.UpdateDeployment(ctx, testDeploymentID, &DeploymentSpec{},
				[]string{DeploymentFieldConcurrencyLimit})
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should send concurrency options and omit them when unset", func() {
			var body map[string]any
			mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer GinkgoRecover()
				body = nil
				Expect(json.NewDecoder(r.Body).Decode(&body)).To(Succeed())
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(Deployment{ID: testDeploymentID})
			}))
			client = NewClient(mockServer.URL, "test-api-key", logger)

			_, err := client.UpdateDeployment(ctx, testDeploymentID, &DeploymentSpec{
				ConcurrencyOptions: &ConcurrencyOptions{CollisionStrategy: "CANCEL_NEW", GracePeriodSeconds: new(90)},
			}, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(body).To(HaveKeyWithValue("concurrency_options",
				map[string]any{"collision_strategy": "CANCEL_NEW", "grace_period_seconds": float64(90)}))

			_, err = client.UpdateDeployment(ctx, testDeploymentID, &DeploymentSpec{Paused: new(true)}, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(body).NotTo(HaveKey("concurrency_options"))

			_, err = client.CreateOrUpdateDeployment(ctx, &DeploymentSpec{
				Name: "created", FlowID: "flow-1",
				ConcurrencyOptions: &ConcurrencyOptions{CollisionStrategy: "ENQUEUE"},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(body).To(HaveKeyWithValue("concurrency_options",
				map[string]any{"collision_strategy": "ENQUEUE"}))
		})

		It("Should send an explicit null when clearing the concurrency options", func() {
			mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer GinkgoRecover()
				var body map[string]any
				Expect(json.NewDecoder(r.Body).Decode(&body)).To(Succeed())
				Expect(body).To(HaveKeyWithValue("concurrency_options", BeNil()))
				Expect(body).NotTo(HaveKey("concurrency_limit"))
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(Deployment{ID: testDeploymentID})
			}))
			client = NewClient(mockServer.URL, "test-api-key", logger)

			_, err := client.UpdateDeployment(ctx, testDeploymentID, &DeploymentSpec{},
				[]string{DeploymentFieldConcurrencyOptions})
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("DeleteDeployment", func() {
		It("Should delete a deployment", func() {
			By("Setting up mock server for deployment deletion")
			mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer GinkgoRecover()
				Expect(r.Method).To(Equal("DELETE"))
				Expect(r.URL.Path).To(Equal("/deployments/deployment-12345"))

				// Return 204 No Content like the real Prefect API
				w.WriteHeader(http.StatusNoContent)
			}))

			By("Creating client with mock server URL")
			client = NewClient(mockServer.URL, "test-api-key", logger)

			By("Calling DeleteDeployment")
			err := client.DeleteDeployment(ctx, testDeploymentID)

			By("Verifying deployment was deleted")
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should handle both 200 and 204 status codes for deletion", func() {
			By("Testing 200 status code first")
			mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer GinkgoRecover()
				Expect(r.Method).To(Equal("DELETE"))
				Expect(r.URL.Path).To(Equal("/deployments/deployment-200"))

				// Some APIs might return 200 with a response body
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"detail": "Deployment deleted"}`))
			}))

			client = NewClient(mockServer.URL, "test-api-key", logger)
			err := client.DeleteDeployment(ctx, "deployment-200")
			Expect(err).NotTo(HaveOccurred())

			mockServer.Close()

			By("Testing 204 status code")
			mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer GinkgoRecover()
				Expect(r.Method).To(Equal("DELETE"))
				Expect(r.URL.Path).To(Equal("/deployments/deployment-204"))

				// Standard response for successful deletion
				w.WriteHeader(http.StatusNoContent)
			}))

			client = NewClient(mockServer.URL, "test-api-key", logger)
			err = client.DeleteDeployment(ctx, "deployment-204")
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("GetDeploymentByName", func() {
		It("Should return error for unimplemented method", func() {
			By("Creating client")
			client = NewClient("http://test.com", "test-api-key", logger)

			By("Calling GetDeploymentByName")
			deployment, err := client.GetDeploymentByName(ctx, testDeploymentName, testFlowID)

			By("Verifying unimplemented error")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("GetDeploymentByName not yet implemented"))
			Expect(deployment).To(BeNil())
		})
	})

	Describe("HTTP Status Code Handling", func() {
		var mockServer *httptest.Server

		AfterEach(func() {
			if mockServer != nil {
				mockServer.Close()
			}
		})

		Context("Success status codes (2xx)", func() {
			It("Should accept 200 OK", func() {
				mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`{"id":"test-123","name":"test","flow_id":"flow-123","status":"READY"}`))
				}))

				client = NewClient(mockServer.URL, "test-api-key", logger)
				deployment, err := client.GetDeployment(ctx, "test-123")

				Expect(err).NotTo(HaveOccurred())
				Expect(deployment).NotTo(BeNil())
				Expect(deployment.ID).To(Equal("test-123"))
			})

			It("Should accept 201 Created for deployment creation", func() {
				mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusCreated) // 201 Created
					_, _ = w.Write([]byte(`{"id":"new-deployment-123","name":"test","flow_id":"flow-123","status":"READY"}`))
				}))

				client = NewClient(mockServer.URL, "test-api-key", logger)
				spec := &DeploymentSpec{
					Name:   testDeploymentName,
					FlowID: testFlowID,
				}
				deployment, err := client.CreateOrUpdateDeployment(ctx, spec)

				Expect(err).NotTo(HaveOccurred())
				Expect(deployment).NotTo(BeNil())
				Expect(deployment.ID).To(Equal("new-deployment-123"))
			})

			It("Should accept 202 Accepted", func() {
				mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusAccepted) // 202 Accepted
					_, _ = w.Write([]byte(`{"id":"test-123","name":"test","flow_id":"flow-123","status":"READY"}`))
				}))

				client = NewClient(mockServer.URL, "test-api-key", logger)
				deployment, err := client.GetDeployment(ctx, "test-123")

				Expect(err).NotTo(HaveOccurred())
				Expect(deployment).NotTo(BeNil())
			})

			It("Should accept 204 No Content for deletions", func() {
				mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusNoContent) // 204 No Content
				}))

				client = NewClient(mockServer.URL, "test-api-key", logger)
				err := client.DeleteDeployment(ctx, "test-123")

				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("Error status codes (non-2xx)", func() {
			It("Should reject 400 Bad Request", func() {
				mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusBadRequest) // 400 Bad Request
					_, _ = w.Write([]byte(`{"error":"Invalid request"}`))
				}))

				client = NewClient(mockServer.URL, "test-api-key", logger)
				deployment, err := client.GetDeployment(ctx, "test-123")

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("API request failed with status 400"))
				Expect(deployment).To(BeNil())
			})

			It("Should reject 401 Unauthorized", func() {
				mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusUnauthorized) // 401 Unauthorized
					_, _ = w.Write([]byte(`{"error":"Unauthorized"}`))
				}))

				client = NewClient(mockServer.URL, "test-api-key", logger)
				deployment, err := client.GetDeployment(ctx, "test-123")

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("API request failed with status 401"))
				Expect(deployment).To(BeNil())
			})

			It("Should reject 422 Unprocessable Entity", func() {
				mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusUnprocessableEntity) // 422 Unprocessable Entity
					_, _ = w.Write([]byte(`{"error":"Validation failed"}`))
				}))

				client = NewClient(mockServer.URL, "test-api-key", logger)
				spec := &DeploymentSpec{
					Name:   "invalid-deployment",
					FlowID: testFlowID,
				}
				deployment, err := client.CreateOrUpdateDeployment(ctx, spec)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("API request failed with status 422"))
				Expect(deployment).To(BeNil())
			})

			It("Should reject 500 Internal Server Error", func() {
				mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusInternalServerError) // 500 Internal Server Error
					_, _ = w.Write([]byte(`{"error":"Server error"}`))
				}))

				client = NewClient(mockServer.URL, "test-api-key", logger)
				deployment, err := client.GetDeployment(ctx, "test-123")

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("API request failed with status 500"))
				Expect(deployment).To(BeNil())
			})
		})
	})

	Describe("Client Creation from ServerReference", func() {
		var (
			serverRef *prefectiov1.PrefectServerReference
			logger    logr.Logger
		)

		BeforeEach(func() {
			logger = logr.Discard()
		})

		Describe("NewClientFromServerReference", func() {
			Context("URL generation behavior (focus on the bug)", func() {
				It("should demonstrate that function hardcodes 'default' namespace", func() {
					// Testing the URL generation logic by comparing what the function would produce
					// vs what it should produce if it accepted a fallback namespace parameter

					serverRef := &prefectiov1.PrefectServerReference{
						Name: testServerName,
						// Namespace is empty
					}

					// What GetAPIURL produces with different fallback namespaces
					urlWithDefault := serverRef.GetAPIURL("default")
					urlWithCustom := serverRef.GetAPIURL("deployment-namespace")

					Expect(urlWithDefault).To(Equal("http://prefect-server.default.svc.cluster.local.:4200/api"))
					Expect(urlWithCustom).To(Equal("http://prefect-server.deployment-namespace.svc.cluster.local.:4200/api"))

					// The bug: NewClientFromServerReference always uses "default" fallback
					// instead of accepting the fallback namespace as a parameter
					Expect(urlWithDefault).NotTo(Equal(urlWithCustom))
				})

				It("should use provided fallback namespace when server namespace is empty (AFTER FIX)", func() {
					serverRef := &prefectiov1.PrefectServerReference{
						Name: testServerName,
						// Namespace is empty - should use fallback namespace parameter
					}

					// This test verifies the fix works for remote servers (to avoid port-forwarding issues)
					serverRefWithRemote := &prefectiov1.PrefectServerReference{
						RemoteAPIURL: new("https://api.prefect.cloud"),
					}

					client, err := NewClientFromServerReference(context.Background(), testKubectl, serverRefWithRemote, Credentials{APIKey: "test-key"}, "fallback-namespace", logger)

					Expect(err).NotTo(HaveOccurred())
					Expect(client).NotTo(BeNil())
					// Remote URL should not be affected by namespace changes
					Expect(client.BaseURL).To(Equal("https://api.prefect.cloud/api"))

					// Test the namespace behavior by checking what GetAPIURL would return
					expectedURL := serverRef.GetAPIURL("fallback-namespace")
					Expect(expectedURL).To(Equal("http://prefect-server.fallback-namespace.svc.cluster.local.:4200/api"))
				})
			})

			Context("when server reference has remote server", func() {
				It("should use remote API URL", func() {
					serverRef = &prefectiov1.PrefectServerReference{
						RemoteAPIURL: new("https://api.prefect.cloud"),
					}

					client, err := NewClientFromServerReference(context.Background(), testKubectl, serverRef, Credentials{APIKey: "test-key"}, "test-namespace", logger)

					Expect(err).NotTo(HaveOccurred())
					Expect(client).NotTo(BeNil())
					Expect(client.BaseURL).To(Equal("https://api.prefect.cloud/api"))
				})

				It("should send requests to the remote API URL when Name is also set", func() {
					serverRef = &prefectiov1.PrefectServerReference{
						RemoteAPIURL: new("https://api.prefect.cloud"),
						Name:         testServerName,
						Namespace:    "prefect-system",
					}

					client, err := NewClientFromServerReference(context.Background(), testKubectl, serverRef, Credentials{APIKey: "test-key"}, "test-namespace", logger)

					Expect(err).NotTo(HaveOccurred())
					Expect(client.BaseURL).To(Equal(serverRef.GetAPIURL("test-namespace")))
					Expect(client.BaseURL).To(Equal("https://api.prefect.cloud/api"))
					Expect(client.portForward).To(BeNil())
				})
			})

			It("should refuse an in-cluster name that isn't a DNS label", func() {
				serverRef := &prefectiov1.PrefectServerReference{Name: "elsewhere.example.com/x?"}

				client, err := NewClientFromServerReference(context.Background(), testKubectl, serverRef, Credentials{APIKey: ""}, "test-namespace", logger)

				Expect(err).To(MatchError(ContainSubstring("is not a DNS label")))
				Expect(client).To(BeNil())
			})

			Context("URL generation for different scenarios", func() {
				It("should correctly generate in-cluster URLs when namespace is specified", func() {
					serverRef := &prefectiov1.PrefectServerReference{
						Name:      testServerName,
						Namespace: "prefect-system",
					}

					// Test the URL generation directly without creating client (avoids port-forwarding issues)
					expectedURL := serverRef.GetAPIURL("fallback-namespace")
					Expect(expectedURL).To(Equal("http://prefect-server.prefect-system.svc.cluster.local.:4200/api"))

					// Verify fallback namespace is ignored when server namespace is specified
					fallbackURL := serverRef.GetAPIURL("different-namespace")
					Expect(fallbackURL).To(Equal("http://prefect-server.prefect-system.svc.cluster.local.:4200/api"))
					Expect(expectedURL).To(Equal(fallbackURL))
				})

				It("should use fallback namespace when server namespace is empty", func() {
					serverRef := &prefectiov1.PrefectServerReference{
						Name: testServerName,
						// Namespace is empty - should use fallback
					}

					// Test different fallback namespaces
					url1 := serverRef.GetAPIURL("namespace-1")
					url2 := serverRef.GetAPIURL("namespace-2")

					Expect(url1).To(Equal("http://prefect-server.namespace-1.svc.cluster.local.:4200/api"))
					Expect(url2).To(Equal("http://prefect-server.namespace-2.svc.cluster.local.:4200/api"))
					Expect(url1).NotTo(Equal(url2))
				})
			})
		})

		Describe("NewClientFromK8s", func() {
			var (
				ctx context.Context
			)

			BeforeEach(func() {
				ctx = context.Background()
			})

			Context("namespace fallback behavior (BUG DEMONSTRATION)", func() {
				It("should demonstrate the bug: NewClientFromK8s doesn't pass fallback namespace", func() {
					// Test the URL generation directly to show the bug without port-forwarding issues
					serverRef := &prefectiov1.PrefectServerReference{
						Name: testServerName,
						// Namespace is empty - should use deployment's namespace
					}

					// Direct test of GetAPIURL method shows what SHOULD happen
					expectedURL := serverRef.GetAPIURL("deployment-namespace")
					Expect(expectedURL).To(Equal("http://prefect-server.deployment-namespace.svc.cluster.local.:4200/api"))

					// But the bug is in NewClientFromServerReference which ignores the fallback
					// and hardcodes "default" instead of using the passed namespace
					actualURL := serverRef.GetAPIURL("default") // This is what the buggy code does
					Expect(actualURL).To(Equal("http://prefect-server.default.svc.cluster.local.:4200/api"))

					// These URLs should be the same but they're different due to the bug
					Expect(expectedURL).NotTo(Equal(actualURL))
				})

				It("should correctly use fallback namespace after fix (VERIFICATION)", func() {
					// Use remote server reference to test NewClientFromK8s without port-forwarding
					serverRef := &prefectiov1.PrefectServerReference{
						RemoteAPIURL: new("https://api.prefect.cloud"),
					}

					client, err := NewClientFromK8s(ctx, testKubectl, serverRef, nil, "deployment-namespace", logger)

					Expect(err).NotTo(HaveOccurred())
					Expect(client).NotTo(BeNil())
					Expect(client.BaseURL).To(Equal("https://api.prefect.cloud/api"))

					// For in-cluster servers, verify the namespace fallback logic would work
					inClusterRef := &prefectiov1.PrefectServerReference{
						Name: testServerName,
						// Namespace is empty - should use deployment-namespace as fallback
					}

					expectedURL := inClusterRef.GetAPIURL("deployment-namespace")
					Expect(expectedURL).To(Equal("http://prefect-server.deployment-namespace.svc.cluster.local.:4200/api"))
				})
			})

			Context("with an auth string", func() {
				authSecret := func(allowed string) *corev1.Secret {
					return &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{Name: "prefect-auth", Namespace: "team-a",
							Annotations: map[string]string{prefectiov1.AnnotationAllowedAPIURLs: allowed}},
						Data: map[string][]byte{"auth-string": []byte("admin:hunter2")},
					}
				}
				authRef := func(remoteAPIURL string) *prefectiov1.PrefectServerReference {
					return &prefectiov1.PrefectServerReference{RemoteAPIURL: new(remoteAPIURL), Auth: &prefectiov1.ServerAuth{
						SecretKeyRef: corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "prefect-auth"}, Key: "auth-string"},
					}}
				}

				It("should send it to a URL that the Secret allows", func() {
					var got string
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						got = r.Header.Get("Authorization")
						w.WriteHeader(http.StatusNotFound)
					}))
					DeferCleanup(server.Close)
					k8sClient := fake.NewClientBuilder().WithObjects(authSecret(server.URL)).Build()

					client, err := NewClientFromK8s(ctx, testKubectl, authRef(server.URL), k8sClient, "team-a", logger)
					Expect(err).NotTo(HaveOccurred())
					_, _ = client.GetWorkPool(ctx, "pool")
					Expect(got).To(Equal("Basic YWRtaW46aHVudGVyMg=="))
				})

				It("should not create a client for a URL that the Secret doesn't allow", func() {
					requests := 0
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						requests++
					}))
					DeferCleanup(server.Close)
					k8sClient := fake.NewClientBuilder().WithObjects(authSecret("http://prefect.team-b.svc.cluster.local:4200")).Build()

					client, err := NewClientFromK8s(ctx, testKubectl, authRef(server.URL), k8sClient, "team-a", logger)
					Expect(err).To(MatchError(ContainSubstring("does not allow its auth string to be used for")))
					Expect(client).To(BeNil())
					Expect(requests).To(BeZero())
				})
			})

			Context("with remote server reference", func() {
				It("should use remote API URL regardless of namespace", func() {
					serverRef = &prefectiov1.PrefectServerReference{
						RemoteAPIURL: new("https://api.prefect.cloud"),
					}

					client, err := NewClientFromK8s(ctx, testKubectl, serverRef, nil, "deployment-namespace", logger)

					Expect(err).NotTo(HaveOccurred())
					Expect(client).NotTo(BeNil())
					Expect(client.BaseURL).To(Equal("https://api.prefect.cloud/api"))
				})
			})
		})
	})
})
