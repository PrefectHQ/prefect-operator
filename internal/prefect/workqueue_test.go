package prefect

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Work queue client", func() {
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

	Describe("GetWorkQueue", func() {
		It("parses the queue on a 200", func() {
			mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer GinkgoRecover()
				Expect(r.URL.Path).To(Equal("/work_pools/pool/queues/ingest"))
				w.Header().Set("Content-Type", "application/json")
				limit := int32(2)
				_ = json.NewEncoder(w).Encode(WorkQueue{ID: "q-1", Name: "ingest", ConcurrencyLimit: &limit})
			}))
			client = NewClient(mockServer.URL, "", logger)

			queue, err := client.GetWorkQueue(ctx, "pool", "ingest")
			Expect(err).NotTo(HaveOccurred())
			Expect(queue.ID).To(Equal("q-1"))
			Expect(queue.ConcurrencyLimit).To(HaveValue(Equal(int32(2))))
		})

		It("path-escapes pool and queue names", func() {
			// Prefect allows spaces, '#' and '?' in names; unescaped, Go drops
			// everything after '#' as a fragment and '?' starts a query string.
			var gotPath string
			mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer GinkgoRecover()
				gotPath = r.URL.EscapedPath()
				w.WriteHeader(http.StatusNotFound)
			}))
			client = NewClient(mockServer.URL, "", logger)

			_, err := client.GetWorkQueue(ctx, "my pool", "batch #2?")
			Expect(err).NotTo(HaveOccurred())
			Expect(gotPath).To(Equal("/work_pools/my%20pool/queues/batch%20%232%3F"))
		})
	})

	Describe("CreateWorkQueue", func() {
		It("POSTs to the pool-scoped route and parses the result", func() {
			mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer GinkgoRecover()
				Expect(r.Method).To(Equal(http.MethodPost))
				Expect(r.URL.EscapedPath()).To(Equal("/work_pools/my%20pool/queues"))
				var body map[string]any
				Expect(json.NewDecoder(r.Body).Decode(&body)).To(Succeed())
				Expect(body["name"]).To(Equal("ingest"))
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(WorkQueue{ID: "q-1", Name: "ingest"})
			}))
			client = NewClient(mockServer.URL, "", logger)

			queue, err := client.CreateWorkQueue(ctx, "my pool", &WorkQueueSpec{Name: "ingest"})
			Expect(err).NotTo(HaveOccurred())
			Expect(queue.ID).To(Equal("q-1"))
		})

		It("omits unset optional fields from the payload", func() {
			// The server applies updates with exclude_unset semantics; a
			// non-pointer is_paused would land as false in every payload.
			mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer GinkgoRecover()
				var body map[string]any
				Expect(json.NewDecoder(r.Body).Decode(&body)).To(Succeed())
				Expect(body).To(HaveKey("name"))
				Expect(body).NotTo(HaveKey("is_paused"))
				Expect(body).NotTo(HaveKey("concurrency_limit"))
				Expect(body).NotTo(HaveKey("priority"))
				Expect(body).NotTo(HaveKey("description"))
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(WorkQueue{ID: "q-1", Name: "ingest"})
			}))
			client = NewClient(mockServer.URL, "", logger)

			_, err := client.CreateWorkQueue(ctx, "pool", &WorkQueueSpec{Name: "ingest"})
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("UpdateWorkQueue", func() {
		It("PATCHes only the set fields, never the name", func() {
			mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer GinkgoRecover()
				Expect(r.Method).To(Equal(http.MethodPatch))
				Expect(r.URL.EscapedPath()).To(Equal("/work_pools/pool/queues/ingest"))
				var body map[string]any
				Expect(json.NewDecoder(r.Body).Decode(&body)).To(Succeed())
				// The queue name is the route key; sending it could rename.
				Expect(body).NotTo(HaveKey("name"))
				Expect(body).To(HaveKeyWithValue("concurrency_limit", float64(3)))
				Expect(body).NotTo(HaveKey("is_paused"))
				Expect(body).NotTo(HaveKey("description"))
				Expect(body).NotTo(HaveKey("priority"))
				w.WriteHeader(http.StatusNoContent)
			}))
			client = NewClient(mockServer.URL, "", logger)

			limit := int32(3)
			err := client.UpdateWorkQueue(ctx, "pool", "ingest", &WorkQueueSpec{Name: "ingest", ConcurrencyLimit: &limit}, nil)
			Expect(err).NotTo(HaveOccurred())
		})

		It("sends explicit defaults for cleared fields", func() {
			mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer GinkgoRecover()
				var body map[string]any
				Expect(json.NewDecoder(r.Body).Decode(&body)).To(Succeed())
				// Create-time defaults: null limit (nullable), "" description
				// (NOT NULL column), false is_paused (plain bool server-side).
				Expect(body).To(HaveKeyWithValue("concurrency_limit", BeNil()))
				Expect(body).To(HaveKeyWithValue("description", ""))
				Expect(body).To(HaveKeyWithValue("is_paused", false))
				w.WriteHeader(http.StatusNoContent)
			}))
			client = NewClient(mockServer.URL, "", logger)

			err := client.UpdateWorkQueue(ctx, "pool", "ingest", &WorkQueueSpec{Name: "ingest"},
				[]string{"concurrencyLimit", "description", "isPaused"})
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("DeleteWorkQueue", func() {
		It("DELETEs the pool-scoped route", func() {
			mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer GinkgoRecover()
				Expect(r.Method).To(Equal(http.MethodDelete))
				Expect(r.URL.EscapedPath()).To(Equal("/work_pools/pool/queues/ingest"))
				w.WriteHeader(http.StatusNoContent)
			}))
			client = NewClient(mockServer.URL, "", logger)

			Expect(client.DeleteWorkQueue(ctx, "pool", "ingest")).To(Succeed())
		})

		It("returns an error on other non-2xx statuses (e.g. default-queue rejection)", func() {
			mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer GinkgoRecover()
				w.WriteHeader(http.StatusConflict)
			}))
			client = NewClient(mockServer.URL, "", logger)

			Expect(client.DeleteWorkQueue(ctx, "pool", "default")).NotTo(Succeed())
		})
	})
})
