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
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// recordedRequest is what the contract server saw for one request.
type recordedRequest struct {
	method string
	path   string // escaped, as sent on the wire
	header http.Header
	body   []byte
}

// contractCall describes one Client method: how to call it, the route it
// must hit, and how it answers a 404.
type contractCall struct {
	name   string
	method string
	path   string
	// okBody is the JSON the server answers a successful call with.
	okBody string
	call   func(ctx context.Context, c *Client) (any, error)
	// on404 asserts the result of a call that Prefect answers with 404.
	on404 func(result any, err error)
}

// Not-found outcomes, one per kind of operation (see ErrNotFound).
var (
	// returnsNil is the outcome for Get methods.
	returnsNil = func(result any, err error) {
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(BeNil())
	}
	// succeeds is the outcome for Delete methods.
	succeeds = func(_ any, err error) {
		Expect(err).NotTo(HaveOccurred())
	}
	// failsNotFound is the outcome for every other method. The error still
	// carries the status and body for logs and conditions.
	failsNotFound = func(_ any, err error) {
		Expect(err).To(MatchError(ErrNotFound))
		Expect(err).To(MatchError(ContainSubstring("status 404")))
	}
)

// noResult adapts a method that only returns an error.
func noResult(err error) (any, error) { return nil, err }

var contractCalls = []contractCall{
	// Deployments
	{
		name: "CreateOrUpdateDeployment", method: http.MethodPost, path: "/deployments/",
		okBody: `{"id": "dep-1"}`,
		call: func(ctx context.Context, c *Client) (any, error) {
			return c.CreateOrUpdateDeployment(ctx, &DeploymentSpec{Name: "etl", FlowID: "flow-1"})
		},
		on404: failsNotFound,
	},
	{
		name: "GetDeployment", method: http.MethodGet, path: "/deployments/dep-1",
		okBody: `{"id": "dep-1"}`,
		call: func(ctx context.Context, c *Client) (any, error) {
			return c.GetDeployment(ctx, "dep-1")
		},
		on404: returnsNil,
	},
	{
		name: "UpdateDeployment", method: http.MethodPatch, path: "/deployments/dep-1",
		okBody: `{"id": "dep-1"}`,
		call: func(ctx context.Context, c *Client) (any, error) {
			return c.UpdateDeployment(ctx, "dep-1", &DeploymentSpec{Name: "etl"}, nil)
		},
		on404: failsNotFound,
	},
	{
		name: "DeleteDeployment", method: http.MethodDelete, path: "/deployments/dep-1",
		call: func(ctx context.Context, c *Client) (any, error) {
			return noResult(c.DeleteDeployment(ctx, "dep-1"))
		},
		on404: succeeds,
	},
	{
		name: "FindDeploymentByName", method: http.MethodPost, path: "/deployments/filter",
		okBody: `[{"id": "dep-1", "name": "etl"}]`,
		call: func(ctx context.Context, c *Client) (any, error) {
			return c.FindDeploymentByName(ctx, "etl")
		},
		on404: failsNotFound,
	},

	// Deployment schedules
	{
		name: "CreateDeploymentSchedules", method: http.MethodPost, path: "/deployments/dep-1/schedules",
		call: func(ctx context.Context, c *Client) (any, error) {
			return noResult(c.CreateDeploymentSchedules(ctx, "dep-1", []DeploymentSchedule{{}}))
		},
		on404: failsNotFound,
	},
	{
		name: "UpdateDeploymentSchedule", method: http.MethodPatch, path: "/deployments/dep-1/schedules/sched-1",
		call: func(ctx context.Context, c *Client) (any, error) {
			return noResult(c.UpdateDeploymentSchedule(ctx, "dep-1", "sched-1", DeploymentScheduleUpdate{}))
		},
		on404: failsNotFound,
	},
	{
		name: "DeleteDeploymentSchedule", method: http.MethodDelete, path: "/deployments/dep-1/schedules/sched-1",
		call: func(ctx context.Context, c *Client) (any, error) {
			return noResult(c.DeleteDeploymentSchedule(ctx, "dep-1", "sched-1"))
		},
		on404: succeeds,
	},

	// Flows
	{
		name: "GetFlowByName", method: http.MethodGet, path: "/flows/name/etl",
		okBody: `{"id": "flow-1", "name": "etl"}`,
		call: func(ctx context.Context, c *Client) (any, error) {
			return c.GetFlowByName(ctx, "etl")
		},
		on404: returnsNil,
	},

	// Work pools
	{
		name: "GetWorkPool", method: http.MethodGet, path: "/work_pools/pool",
		okBody: `{"id": "wp-1"}`,
		call: func(ctx context.Context, c *Client) (any, error) {
			return c.GetWorkPool(ctx, "pool")
		},
		on404: returnsNil,
	},
	{
		name: "CreateWorkPool", method: http.MethodPost, path: "/work_pools/",
		okBody: `{"id": "wp-1"}`,
		call: func(ctx context.Context, c *Client) (any, error) {
			return c.CreateWorkPool(ctx, &WorkPoolSpec{Name: "pool"})
		},
		on404: failsNotFound,
	},
	{
		name: "UpdateWorkPool", method: http.MethodPatch, path: "/work_pools/pool",
		call: func(ctx context.Context, c *Client) (any, error) {
			return noResult(c.UpdateWorkPool(ctx, "pool", &WorkPoolSpec{}))
		},
		on404: failsNotFound,
	},
	{
		name: "DeleteWorkPool", method: http.MethodDelete, path: "/work_pools/pool",
		call: func(ctx context.Context, c *Client) (any, error) {
			return noResult(c.DeleteWorkPool(ctx, "pool"))
		},
		on404: succeeds,
	},
	{
		name: "GetWorkerMetadata", method: http.MethodGet, path: "/collections/views/aggregate-worker-metadata",
		okBody: `{}`,
		call: func(ctx context.Context, c *Client) (any, error) {
			return c.GetWorkerMetadata(ctx)
		},
		// The route is missing, not an object, but the outcome is the same.
		on404: returnsNil,
	},

	// Work queues
	{
		name: "GetWorkQueue", method: http.MethodGet, path: "/work_pools/pool/queues/ingest",
		okBody: `{"id": "q-1"}`,
		call: func(ctx context.Context, c *Client) (any, error) {
			return c.GetWorkQueue(ctx, "pool", "ingest")
		},
		on404: returnsNil,
	},
	{
		name: "CreateWorkQueue", method: http.MethodPost, path: "/work_pools/pool/queues",
		okBody: `{"id": "q-1"}`,
		call: func(ctx context.Context, c *Client) (any, error) {
			return c.CreateWorkQueue(ctx, "pool", &WorkQueueSpec{Name: "ingest"})
		},
		// A 404 here means the parent pool is missing.
		on404: failsNotFound,
	},
	{
		name: "UpdateWorkQueue", method: http.MethodPatch, path: "/work_pools/pool/queues/ingest",
		call: func(ctx context.Context, c *Client) (any, error) {
			return noResult(c.UpdateWorkQueue(ctx, "pool", "ingest", &WorkQueueSpec{Name: "ingest"}, nil))
		},
		on404: failsNotFound,
	},
	{
		name: "DeleteWorkQueue", method: http.MethodDelete, path: "/work_pools/pool/queues/ingest",
		call: func(ctx context.Context, c *Client) (any, error) {
			return noResult(c.DeleteWorkQueue(ctx, "pool", "ingest"))
		},
		on404: succeeds,
	},

	// Automations
	{
		name: "CreateAutomation", method: http.MethodPost, path: "/automations/",
		okBody: `{"id": "auto-1"}`,
		call: func(ctx context.Context, c *Client) (any, error) {
			return c.CreateAutomation(ctx, &AutomationSpec{Name: "alert"})
		},
		on404: failsNotFound,
	},
	{
		name: "GetAutomation", method: http.MethodGet, path: "/automations/auto-1",
		okBody: `{"id": "auto-1"}`,
		call: func(ctx context.Context, c *Client) (any, error) {
			return c.GetAutomation(ctx, "auto-1")
		},
		on404: returnsNil,
	},
	{
		// UpdateAutomation re-reads the automation after the PUT.
		name: "UpdateAutomation", method: http.MethodPut, path: "/automations/auto-1",
		okBody: `{"id": "auto-1"}`,
		call: func(ctx context.Context, c *Client) (any, error) {
			return c.UpdateAutomation(ctx, "auto-1", &AutomationSpec{Name: "alert"})
		},
		on404: failsNotFound,
	},
	{
		name: "DeleteAutomation", method: http.MethodDelete, path: "/automations/auto-1",
		call: func(ctx context.Context, c *Client) (any, error) {
			return noResult(c.DeleteAutomation(ctx, "auto-1"))
		},
		on404: succeeds,
	},
}

var _ = Describe("Client contract", func() {
	const apiKey = "test-api-key"

	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	// serve starts a server that answers every request with status and body
	// and records what it received.
	serve := func(status int, body string) (*httptest.Server, func() []recordedRequest) {
		var (
			mu   sync.Mutex
			reqs []recordedRequest
		)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer GinkgoRecover()
			reqBody, err := io.ReadAll(r.Body)
			Expect(err).NotTo(HaveOccurred())
			mu.Lock()
			reqs = append(reqs, recordedRequest{
				method: r.Method,
				path:   r.URL.EscapedPath(),
				header: r.Header.Clone(),
				body:   reqBody,
			})
			mu.Unlock()
			if body != "" {
				w.Header().Set("Content-Type", "application/json")
			}
			w.WriteHeader(status)
			_, _ = io.WriteString(w, body)
		}))
		DeferCleanup(server.Close)
		return server, func() []recordedRequest {
			mu.Lock()
			defer mu.Unlock()
			return slices.Clone(reqs)
		}
	}

	It("path-escapes names", func() {
		// spec.deployment.flowName can be any Prefect name, and Prefect names
		// may contain spaces, '#', and '?'.
		server, requests := serve(http.StatusOK, `{"id": "flow-1"}`)
		client := NewClient(server.URL, "", logr.Discard())

		_, err := client.GetFlowByName(ctx, "etl #2?")
		Expect(err).NotTo(HaveOccurred())
		_, err = client.GetWorkPool(ctx, "my pool")
		Expect(err).NotTo(HaveOccurred())

		reqs := requests()
		paths := make([]string, len(reqs))
		for i, r := range reqs {
			paths[i] = r.path
		}
		Expect(paths).To(Equal([]string{"/flows/name/etl%20%232%3F", "/work_pools/my%20pool"}))
	})

	// UpdateDeployment and UpdateAutomation read the object back after Prefect
	// answers the update with 204. An object deleted in between fails the read.
	DescribeTable("returns ErrNotFound when the read-back after an update 404s",
		func(call func(context.Context, *Client) (any, error)) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet {
					w.WriteHeader(http.StatusNotFound)
					return
				}
				w.WriteHeader(http.StatusNoContent)
			}))
			DeferCleanup(server.Close)

			result, err := call(ctx, NewClient(server.URL, "", logr.Discard()))
			Expect(err).To(MatchError(ErrNotFound))
			Expect(result).To(BeNil())
		},
		Entry("UpdateDeployment", func(ctx context.Context, c *Client) (any, error) {
			return c.UpdateDeployment(ctx, "dep-1", &DeploymentSpec{Name: "etl"}, nil)
		}),
		Entry("UpdateAutomation", func(ctx context.Context, c *Client) (any, error) {
			return c.UpdateAutomation(ctx, "auto-1", &AutomationSpec{Name: "alert"})
		}),
	)

	for _, cc := range contractCalls {
		Describe(cc.name, func() {
			It("calls "+cc.method+" "+cc.path+" with the API key", func() {
				server, requests := serve(http.StatusOK, cc.okBody)

				_, err := cc.call(ctx, NewClient(server.URL, apiKey, logr.Discard()))
				Expect(err).NotTo(HaveOccurred())

				reqs := requests()
				Expect(reqs).NotTo(BeEmpty())
				Expect(reqs[0].method).To(Equal(cc.method))
				Expect(reqs[0].path).To(Equal(cc.path))
				for _, r := range reqs {
					Expect(r.header.Get("Authorization")).To(Equal("Bearer " + apiKey))
					if len(r.body) > 0 {
						Expect(r.header.Get("Content-Type")).To(Equal("application/json"))
					}
				}
			})

			It("sends no Authorization header without an API key", func() {
				server, requests := serve(http.StatusOK, cc.okBody)

				_, err := cc.call(ctx, NewClient(server.URL, "", logr.Discard()))
				Expect(err).NotTo(HaveOccurred())

				for _, r := range requests() {
					Expect(r.header).NotTo(HaveKey("Authorization"))
				}
			})

			It("handles a 404", func() {
				server, _ := serve(http.StatusNotFound, `{"detail": "Not found."}`)

				result, err := cc.call(ctx, NewClient(server.URL, apiKey, logr.Discard()))
				cc.on404(result, err)
			})

			It("returns an error carrying the status and body on a 500", func() {
				server, _ := serve(http.StatusInternalServerError, `{"detail": "boom"}`)

				_, err := cc.call(ctx, NewClient(server.URL, apiKey, logr.Discard()))
				Expect(err).To(MatchError(And(ContainSubstring("status 500"), ContainSubstring("boom"))))
				Expect(err).NotTo(MatchError(ErrNotFound))
			})
		})
	}
})
