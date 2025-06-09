/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package prefect

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"
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

	Describe("Authentication", func() {
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
					Name: "test-flow",
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(expectedFlow)
			}))

			By("Creating client with empty API key")
			client = NewClient(mockServer.URL, "", logger)

			By("Calling GetFlowByName")
			flow, err := client.GetFlowByName(ctx, "test-flow")

			By("Verifying request succeeds without authentication")
			Expect(err).NotTo(HaveOccurred())
			Expect(flow).NotTo(BeNil())
			Expect(flow.Name).To(Equal("test-flow"))
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
			flow, err := client.GetFlowByName(ctx, "test-flow")

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
			flow, err := client.GetFlowByName(ctx, "test-flow")

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
				_ = json.NewEncoder(w).Encode(Flow{ID: "test", Name: "test-flow"})
			}))

			By("Creating client with mock server URL")
			client = NewClient(mockServer.URL, "test-api-key", logger)

			By("Creating cancelled context")
			cancelCtx, cancel := context.WithCancel(ctx)
			cancel()

			By("Calling GetFlowByName with cancelled context")
			flow, err := client.GetFlowByName(cancelCtx, "test-flow")

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
			flow, err := client.GetFlowByName(ctx, "test-flow")

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
			flow, err := client.GetFlowByName(ctx, "test-flow")

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
	})

	Describe("GetFlowByName", func() {
		It("Should successfully retrieve a flow by name", func() {
			By("Setting up mock server with flow response")
			expectedFlow := Flow{
				ID:      "flow-12345",
				Created: time.Now(),
				Updated: time.Now(),
				Name:    "test-flow",
				Tags:    []string{"test", "example"},
				Labels:  map[string]string{"env": "test"},
			}

			mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer GinkgoRecover()
				// Verify request method and path
				Expect(r.Method).To(Equal("GET"))
				Expect(r.URL.Path).To(Equal("/flows/name/test-flow"))
				Expect(r.Header.Get("Content-Type")).To(Equal("application/json"))

				// Return mock flow response
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(expectedFlow)
			}))

			By("Creating client with mock server URL")
			client = NewClient(mockServer.URL, "test-api-key", logger)

			By("Calling GetFlowByName")
			flow, err := client.GetFlowByName(ctx, "test-flow")

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
				Tags:    []string{"new", "created"},
				Labels:  map[string]string{"source": "operator"},
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
				Tags:   []string{"new", "created"},
				Labels: map[string]string{"source": "operator"},
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
				Tags:   []string{"new", "tag"},
				Labels: map[string]string{"source": "operator"},
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
				ID:           "deployment-12345",
				Created:      time.Now(),
				Updated:      time.Now(),
				Name:         "test-deployment",
				FlowID:       "flow-123",
				Paused:       false,
				Tags:         []string{"test", "deployment"},
				Parameters:   map[string]interface{}{"param1": "value1"},
				Entrypoint:   ptr.To("flows.py:main_flow"),
				WorkPoolName: ptr.To("kubernetes"),
			}

			mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer GinkgoRecover()
				Expect(r.Method).To(Equal("POST"))
				Expect(r.URL.Path).To(Equal("/deployments/"))
				Expect(r.Header.Get("Content-Type")).To(Equal("application/json"))

				// Verify request body contains deployment spec
				var deploymentSpec DeploymentSpec
				_ = json.NewDecoder(r.Body).Decode(&deploymentSpec)
				Expect(deploymentSpec.Name).To(Equal("test-deployment"))
				Expect(deploymentSpec.FlowID).To(Equal("flow-123"))

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(expectedDeployment)
			}))

			By("Creating client with mock server URL")
			client = NewClient(mockServer.URL, "test-api-key", logger)

			By("Calling CreateOrUpdateDeployment")
			deploymentSpec := &DeploymentSpec{
				Name:         "test-deployment",
				FlowID:       "flow-123",
				Tags:         []string{"test", "deployment"},
				Parameters:   map[string]interface{}{"param1": "value1"},
				Entrypoint:   ptr.To("flows.py:main_flow"),
				WorkPoolName: ptr.To("kubernetes"),
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
				ID:           "deployment-12345",
				Created:      time.Now(),
				Updated:      time.Now(),
				Name:         "test-deployment",
				FlowID:       "flow-123",
				Paused:       false,
				Status:       "READY",
				Tags:         []string{"test"},
				WorkPoolName: ptr.To("kubernetes"),
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
			deployment, err := client.GetDeployment(ctx, "deployment-12345")

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
				ID:           "deployment-12345",
				Created:      time.Now().Add(-time.Hour),
				Updated:      time.Now(),
				Name:         "updated-deployment",
				FlowID:       "flow-123",
				Paused:       true,
				Tags:         []string{"updated", "test"},
				Parameters:   map[string]interface{}{"param1": "updated_value"},
				WorkPoolName: ptr.To("kubernetes"),
			}

			mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer GinkgoRecover()
				Expect(r.Method).To(Equal("PATCH"))
				Expect(r.URL.Path).To(Equal("/deployments/deployment-12345"))
				Expect(r.Header.Get("Content-Type")).To(Equal("application/json"))

				// Verify request body contains updated deployment spec
				var deploymentSpec DeploymentSpec
				_ = json.NewDecoder(r.Body).Decode(&deploymentSpec)
				Expect(deploymentSpec.Name).To(Equal("updated-deployment"))
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
				FlowID:     "flow-123",
				Paused:     ptr.To(true),
				Tags:       []string{"updated", "test"},
				Parameters: map[string]interface{}{"param1": "updated_value"},
			}
			deployment, err := client.UpdateDeployment(ctx, "deployment-12345", deploymentSpec)

			By("Verifying deployment was updated")
			Expect(err).NotTo(HaveOccurred())
			Expect(deployment).NotTo(BeNil())
			Expect(deployment.ID).To(Equal("deployment-12345"))
			Expect(deployment.Name).To(Equal("updated-deployment"))
			Expect(deployment.Paused).To(BeTrue())
		})
	})

	Describe("DeleteDeployment", func() {
		It("Should delete a deployment", func() {
			By("Setting up mock server for deployment deletion")
			mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer GinkgoRecover()
				Expect(r.Method).To(Equal("DELETE"))
				Expect(r.URL.Path).To(Equal("/deployments/deployment-12345"))

				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"detail": "Deployment deleted successfully"}`))
			}))

			By("Creating client with mock server URL")
			client = NewClient(mockServer.URL, "test-api-key", logger)

			By("Calling DeleteDeployment")
			err := client.DeleteDeployment(ctx, "deployment-12345")

			By("Verifying deployment was deleted")
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("GetDeploymentByName", func() {
		It("Should return error for unimplemented method", func() {
			By("Creating client")
			client = NewClient("http://test.com", "test-api-key", logger)

			By("Calling GetDeploymentByName")
			deployment, err := client.GetDeploymentByName(ctx, "test-deployment", "flow-123")

			By("Verifying unimplemented error")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("GetDeploymentByName not yet implemented"))
			Expect(deployment).To(BeNil())
		})
	})
})
