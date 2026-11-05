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

// Routes, auth, 404 handling, and error statuses are covered by the client
// contract table; these specs cover payloads and parsing.
var _ = Describe("Work pool client", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	// serve starts a server with handler and returns a client for it.
	serve := func(handler http.HandlerFunc) *Client {
		server := httptest.NewServer(handler)
		DeferCleanup(server.Close)
		return NewClient(server.URL, "", logr.Discard())
	}

	Describe("GetWorkPool", func() {
		It("parses the pool", func() {
			client := serve(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{
					"id": "wp-1",
					"name": "k8s-pool",
					"type": "kubernetes",
					"status": "READY",
					"is_paused": true,
					"concurrency_limit": 4,
					"base_job_template": {"variables": {}}
				}`))
			})

			pool, err := client.GetWorkPool(ctx, "k8s-pool")
			Expect(err).NotTo(HaveOccurred())
			Expect(pool.ID).To(Equal("wp-1"))
			Expect(pool.Type).To(Equal("kubernetes"))
			Expect(pool.Status).To(Equal(WorkPoolStatusReady))
			Expect(pool.IsPaused).To(BeTrue())
			Expect(pool.ConcurrencyLimit).To(HaveValue(Equal(4)))
			Expect(pool.BaseJobTemplate).To(HaveKey("variables"))
		})
	})

	Describe("CreateWorkPool", func() {
		It("sends the spec and parses the created pool", func() {
			client := serve(func(w http.ResponseWriter, r *http.Request) {
				defer GinkgoRecover()
				var body map[string]any
				Expect(json.NewDecoder(r.Body).Decode(&body)).To(Succeed())
				Expect(body).To(HaveKeyWithValue("name", "k8s-pool"))
				Expect(body).To(HaveKeyWithValue("type", "kubernetes"))
				Expect(body).To(HaveKeyWithValue("base_job_template",
					HaveKeyWithValue("job_configuration", HaveKeyWithValue("image", "{{ image }}"))))

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte(`{"id": "wp-1", "name": "k8s-pool", "status": "NOT_READY"}`))
			})

			pool, err := client.CreateWorkPool(ctx, &WorkPoolSpec{
				Name: "k8s-pool",
				Type: "kubernetes",
				BaseJobTemplate: map[string]any{
					"job_configuration": map[string]any{"image": "{{ image }}"},
				},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(pool.ID).To(Equal("wp-1"))
			Expect(pool.Status).To(Equal("NOT_READY"))
		})

		It("omits unset optional fields", func() {
			client := serve(func(w http.ResponseWriter, r *http.Request) {
				defer GinkgoRecover()
				var body map[string]any
				Expect(json.NewDecoder(r.Body).Decode(&body)).To(Succeed())
				Expect(body).To(HaveKey("name"))
				Expect(body).NotTo(HaveKey("description"))
				Expect(body).NotTo(HaveKey("is_paused"))
				Expect(body).NotTo(HaveKey("concurrency_limit"))
				Expect(body).NotTo(HaveKey("base_job_template"))
				_, _ = w.Write([]byte(`{"id": "wp-1"}`))
			})

			_, err := client.CreateWorkPool(ctx, &WorkPoolSpec{Name: "k8s-pool"})
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("UpdateWorkPool", func() {
		It("sends only the set fields, never the name or type", func() {
			client := serve(func(w http.ResponseWriter, r *http.Request) {
				defer GinkgoRecover()
				var body map[string]any
				Expect(json.NewDecoder(r.Body).Decode(&body)).To(Succeed())
				// Prefect's work pool update schema forbids name and type.
				Expect(body).NotTo(HaveKey("name"))
				Expect(body).NotTo(HaveKey("type"))
				Expect(body).To(HaveKeyWithValue("base_job_template", HaveKey("variables")))
				w.WriteHeader(http.StatusNoContent)
			})

			err := client.UpdateWorkPool(ctx, "k8s-pool", &WorkPoolSpec{
				BaseJobTemplate: map[string]any{"variables": map[string]any{}},
			})
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("GetWorkerMetadata", func() {
		It("flattens the per-collection response into one map keyed by worker type", func() {
			client := serve(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{
					"prefect": {
						"process": {"type": "process", "display_name": "Process"}
					},
					"prefect-kubernetes": {
						"kubernetes": {
							"type": "kubernetes",
							"display_name": "Kubernetes",
							"default_base_job_configuration": {"job_configuration": {"namespace": "{{ namespace }}"}}
						}
					}
				}`))
			})

			metadata, err := client.GetWorkerMetadata(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(metadata).To(HaveLen(2))
			Expect(metadata["process"].DisplayName).To(Equal("Process"))
			Expect(metadata["kubernetes"].DisplayName).To(Equal("Kubernetes"))
			Expect(metadata["kubernetes"].DefaultBaseJobTemplate).To(
				HaveKeyWithValue("job_configuration", HaveKeyWithValue("namespace", "{{ namespace }}")))
		})

		It("returns an error when the response isn't keyed by collection", func() {
			client := serve(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`["not", "a", "map"]`))
			})

			metadata, err := client.GetWorkerMetadata(ctx)
			Expect(err).To(MatchError(ContainSubstring("failed to unmarshal")))
			Expect(metadata).To(BeNil())
		})
	})
})
