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
	"errors"
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

	Describe("UpdateAutomation", func() {
		It("returns ErrAutomationNotFound on a 404", func() {
			mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer GinkgoRecover()
				Expect(r.Method).To(Equal(http.MethodPut))
				w.WriteHeader(http.StatusNotFound)
			}))
			client = NewClient(mockServer.URL, "", logger)

			_, err := client.UpdateAutomation(ctx, "missing-id", &AutomationSpec{Name: "a"})
			Expect(err).To(HaveOccurred())
			Expect(errors.Is(err, ErrAutomationNotFound)).To(BeTrue())
		})

		It("returns a generic error on other non-2xx statuses", func() {
			mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer GinkgoRecover()
				w.WriteHeader(http.StatusInternalServerError)
			}))
			client = NewClient(mockServer.URL, "", logger)

			_, err := client.UpdateAutomation(ctx, "some-id", &AutomationSpec{Name: "a"})
			Expect(err).To(HaveOccurred())
			Expect(errors.Is(err, ErrAutomationNotFound)).To(BeFalse())
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

		It("errors when the name matches no deployment", func() {
			mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer GinkgoRecover()
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode([]Deployment{})
			}))
			client = NewClient(mockServer.URL, "", logger)

			_, err := client.FindDeploymentByName(ctx, "nope")
			Expect(err).To(HaveOccurred())
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
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("ambiguous"))
		})
	})
})
