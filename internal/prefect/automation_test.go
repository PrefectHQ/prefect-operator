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

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Automation client", func() {
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

	Describe("CreateAutomation", func() {
		It("POSTs the spec and parses the created automation", func() {
			mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer GinkgoRecover()
				var body map[string]any
				Expect(json.NewDecoder(r.Body).Decode(&body)).To(Succeed())
				Expect(body).To(HaveKeyWithValue("name", "alert"))
				Expect(body).To(HaveKeyWithValue("description", "page on failure"))
				Expect(body).To(HaveKeyWithValue("enabled", false))
				Expect(body).To(HaveKeyWithValue("trigger", HaveKeyWithValue("type", "event")))
				Expect(body).To(HaveKeyWithValue("actions", ConsistOf(HaveKeyWithValue("type", "do-nothing"))))
				Expect(body).NotTo(HaveKey("actions_on_trigger"))
				Expect(body).NotTo(HaveKey("actions_on_resolve"))

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte(`{"id": "auto-1", "name": "alert", "enabled": false}`))
			}))
			client = NewClient(mockServer.URL, "", logger)

			automation, err := client.CreateAutomation(ctx, &AutomationSpec{
				Name:        "alert",
				Description: "page on failure",
				Enabled:     new(false),
				Trigger:     map[string]any{"type": "event"},
				Actions:     []map[string]any{{"type": "do-nothing"}},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(automation.ID).To(Equal("auto-1"))
			Expect(automation.Enabled).To(BeFalse())
		})
	})

	Describe("GetAutomation", func() {
		It("parses the automation", func() {
			mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer GinkgoRecover()
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{
					"id": "auto-1",
					"name": "alert",
					"enabled": true,
					"trigger": {"type": "event", "posture": "Reactive"},
					"actions": [{"type": "cancel-flow-run"}],
					"actions_on_resolve": [{"type": "do-nothing"}]
				}`))
			}))
			client = NewClient(mockServer.URL, "", logger)

			automation, err := client.GetAutomation(ctx, "auto-1")
			Expect(err).NotTo(HaveOccurred())
			Expect(automation.Enabled).To(BeTrue())
			Expect(automation.Trigger).To(HaveKeyWithValue("posture", "Reactive"))
			Expect(automation.Actions).To(ConsistOf(HaveKeyWithValue("type", "cancel-flow-run")))
			Expect(automation.ActionsOnResolve).To(HaveLen(1))
		})
	})

	Describe("UpdateAutomation", func() {
		It("PUTs the full spec, then returns the automation as re-read", func() {
			var calls []string
			mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer GinkgoRecover()
				calls = append(calls, r.Method)
				switch r.Method {
				case http.MethodPut:
					var body map[string]any
					Expect(json.NewDecoder(r.Body).Decode(&body)).To(Succeed())
					// A full replace: unset optional fields are still sent.
					Expect(body).To(HaveKeyWithValue("description", ""))
					Expect(body).To(HaveKeyWithValue("trigger", BeNil()))
					// Prefect answers a PUT with 204 and no body.
					w.WriteHeader(http.StatusNoContent)
				case http.MethodGet:
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write([]byte(`{"id": "auto-1", "name": "renamed"}`))
				}
			}))
			client = NewClient(mockServer.URL, "", logger)

			automation, err := client.UpdateAutomation(ctx, "auto-1", &AutomationSpec{Name: "renamed"})
			Expect(err).NotTo(HaveOccurred())
			Expect(calls).To(Equal([]string{http.MethodPut, http.MethodGet}))
			Expect(automation.Name).To(Equal("renamed"))
		})
	})

	Describe("FindDeploymentByName", func() {
		It("resolves a unique name", func() {
			mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer GinkgoRecover()
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode([]Deployment{{ID: "dep-1", Name: "daily-etl"}})
			}))
			client = NewClient(mockServer.URL, "", logger)

			dep, err := client.FindDeploymentByName(ctx, "daily-etl")
			Expect(err).NotTo(HaveOccurred())
			Expect(dep.ID).To(Equal("dep-1"))
		})

		It("returns ErrNotFound when the name matches no deployment", func() {
			mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer GinkgoRecover()
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode([]Deployment{})
			}))
			client = NewClient(mockServer.URL, "", logger)

			_, err := client.FindDeploymentByName(ctx, "nope")
			Expect(err).To(MatchError(ErrNotFound))
			Expect(err.Error()).To(ContainSubstring("no deployment found"))
		})

		It("errors when the name is ambiguous across flows", func() {
			mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer GinkgoRecover()
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode([]Deployment{
					{ID: "dep-1", Name: "daily-etl", FlowID: "flow-a"},
					{ID: "dep-2", Name: "daily-etl", FlowID: "flow-b"},
				})
			}))
			client = NewClient(mockServer.URL, "", logger)

			_, err := client.FindDeploymentByName(ctx, "daily-etl")
			Expect(err).To(MatchError(ContainSubstring("ambiguous")))
			Expect(err).NotTo(MatchError(ErrNotFound))
		})
	})
})
