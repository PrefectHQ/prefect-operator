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
	"io"
	"net/http"
	"net/http/httptest"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Deployment schedule client", func() {
	var (
		ctx    context.Context
		client *Client
		// body and contentType hold what the server received.
		body        []byte
		contentType string
	)

	BeforeEach(func() {
		ctx = context.Background()
		body, contentType = nil, ""
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer GinkgoRecover()
			var err error
			body, err = io.ReadAll(r.Body)
			Expect(err).NotTo(HaveOccurred())
			contentType = r.Header.Get("Content-Type")
			w.WriteHeader(http.StatusOK)
		}))
		DeferCleanup(server.Close)
		client = NewClient(server.URL, "", logr.Discard())
	})

	decoded := func() any {
		var v any
		Expect(json.Unmarshal(body, &v)).To(Succeed())
		return v
	}

	It("CreateDeploymentSchedules sends the schedules as a JSON array", func() {
		Expect(client.CreateDeploymentSchedules(ctx, "dep-1", []DeploymentSchedule{
			{Slug: new("hourly"), Schedule: Schedule{Interval: new(3600.0)}, Active: new(true)},
			{Slug: new("nightly"), Schedule: Schedule{Cron: new("0 2 * * *"), Timezone: new("UTC")}},
		})).To(Succeed())

		Expect(contentType).To(Equal("application/json"))
		Expect(decoded()).To(ConsistOf(
			And(
				HaveKeyWithValue("slug", "hourly"),
				HaveKeyWithValue("schedule", HaveKeyWithValue("interval", 3600.0)),
				HaveKeyWithValue("active", true),
			),
			And(
				HaveKeyWithValue("slug", "nightly"),
				HaveKeyWithValue("schedule", And(
					HaveKeyWithValue("cron", "0 2 * * *"),
					HaveKeyWithValue("timezone", "UTC"),
				)),
				Not(HaveKey("active")),
			),
		))
	})

	It("UpdateDeploymentSchedule sends only the fields that are set", func() {
		Expect(client.UpdateDeploymentSchedule(ctx, "dep-1", "sched-1", DeploymentScheduleUpdate{
			Schedule: &Schedule{Cron: new("*/5 * * * *")},
			Active:   new(false),
		})).To(Succeed())

		Expect(contentType).To(Equal("application/json"))
		Expect(decoded()).To(And(
			HaveKeyWithValue("schedule", HaveKeyWithValue("cron", "*/5 * * * *")),
			HaveKeyWithValue("active", false),
			Not(HaveKey("max_scheduled_runs")),
			Not(HaveKey("parameters")),
		))
	})

	It("UpdateDeploymentSchedule sends empty parameters to clear them", func() {
		Expect(client.UpdateDeploymentSchedule(ctx, "dep-1", "sched-1", DeploymentScheduleUpdate{
			Parameters: map[string]any{},
		})).To(Succeed())

		Expect(decoded()).To(HaveKeyWithValue("parameters", BeEmpty()))
	})

	It("DeleteDeploymentSchedule sends no body and no Content-Type", func() {
		Expect(client.DeleteDeploymentSchedule(ctx, "dep-1", "sched-1")).To(Succeed())

		Expect(body).To(BeEmpty())
		Expect(contentType).To(BeEmpty())
	})
})
