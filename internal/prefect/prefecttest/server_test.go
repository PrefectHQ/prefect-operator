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

package prefecttest_test

import (
	"bytes"
	"context"
	"net/http"
	"testing"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/PrefectHQ/prefect-operator/internal/prefect"
	"github.com/PrefectHQ/prefect-operator/internal/prefect/prefecttest"
)

func TestPrefecttest(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Fake Prefect API Suite")
}

// These specs drive the fake with the real client, so they also check that
// the two agree on routes and JSON.
var _ = Describe("Fake Prefect API", func() {
	var (
		ctx    context.Context
		server *prefecttest.Server
		client *prefect.Client
	)

	BeforeEach(func() {
		ctx = context.Background()
		server = prefecttest.NewServer()
		DeferCleanup(server.Close)
		client = prefect.NewClient(server.APIURL, "", logr.Discard())
	})

	createPool := func(name string) *prefect.WorkPool {
		pool, err := client.CreateWorkPool(ctx, &prefect.WorkPoolSpec{Name: name, Type: "kubernetes"})
		Expect(err).NotTo(HaveOccurred())
		return pool
	}

	createDeployment := func(spec *prefect.DeploymentSpec) *prefect.Deployment {
		deployment, err := client.CreateOrUpdateDeployment(ctx, spec)
		Expect(err).NotTo(HaveOccurred())
		return deployment
	}

	Describe("routing", func() {
		It("answers 501 for a request it has no route for", func() {
			resp, err := http.Post(server.APIURL+"/deployments/not-a-route", "application/json", bytes.NewReader([]byte(`{}`)))
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusNotImplemented))
		})

		It("answers 404 for an ID that is not a UUID", func() {
			deployment, err := client.GetDeployment(ctx, "dep-1")
			Expect(err).NotTo(HaveOccurred())
			Expect(deployment).To(BeNil())
		})

		It("fails a route on request and counts its calls", func() {
			server.Fail("POST /work_pools/", http.StatusInternalServerError)
			_, err := client.CreateWorkPool(ctx, &prefect.WorkPoolSpec{Name: "pool"})
			Expect(err).To(MatchError(ContainSubstring("status 500")))

			server.ClearFailures()
			createPool("pool")
			Expect(server.Calls("POST /work_pools/")).To(Equal(2))
		})

		It("panics on a route it does not serve", func() {
			Expect(func() { server.Calls("PATCH /work_pool/{name}") }).To(PanicWith(ContainSubstring("no route")))
		})
	})

	Describe("deployments", func() {
		var flow *prefect.Flow

		BeforeEach(func() {
			createPool("pool")
			var err error
			flow, err = client.CreateOrGetFlow(ctx, &prefect.FlowSpec{Name: "etl"})
			Expect(err).NotTo(HaveOccurred())
		})

		It("requires the work pool to exist", func() {
			_, err := client.CreateOrUpdateDeployment(ctx, &prefect.DeploymentSpec{
				Name: "daily", FlowID: flow.ID, WorkPoolName: new("missing"),
			})
			Expect(err).To(MatchError(prefect.ErrNotFound))
		})

		It("uses the pool's default queue, or creates the named queue", func() {
			deployment := createDeployment(&prefect.DeploymentSpec{
				Name: "daily", FlowID: flow.ID, WorkPoolName: new("pool"),
			})
			Expect(deployment.WorkQueueName).To(HaveValue(Equal("default")))

			createDeployment(&prefect.DeploymentSpec{
				Name: "hourly", FlowID: flow.ID, WorkPoolName: new("pool"), WorkQueueName: new("fast"),
			})
			queue, err := client.GetWorkQueue(ctx, "pool", "fast")
			Expect(err).NotTo(HaveOccurred())
			Expect(queue).NotTo(BeNil())
		})

		It("updates the deployment with the same flow and name", func() {
			first := createDeployment(&prefect.DeploymentSpec{Name: "daily", FlowID: flow.ID, Version: new("1")})
			second := createDeployment(&prefect.DeploymentSpec{Name: "daily", FlowID: flow.ID, Version: new("2")})
			Expect(second.ID).To(Equal(first.ID))
			Expect(second.Version).To(HaveValue(Equal("2")))
		})

		It("applies only the fields an update sends", func() {
			created := createDeployment(&prefect.DeploymentSpec{
				Name: "daily", FlowID: flow.ID, Description: new("nightly"), ConcurrencyLimit: new(2),
			})
			Expect(created.ConcurrencyLimit).To(BeNil(), "Prefect answers null for the deprecated field")
			Expect(created.GlobalConcurrencyLimit).To(HaveField("Limit", 2))

			updated, err := client.UpdateDeployment(ctx, created.ID,
				&prefect.DeploymentSpec{Name: "daily", Version: new("2")},
				[]string{prefect.DeploymentFieldConcurrencyLimit})
			Expect(err).NotTo(HaveOccurred())
			Expect(updated.Version).To(HaveValue(Equal("2")))
			Expect(updated.Description).To(HaveValue(Equal("nightly")))
			Expect(updated.GlobalConcurrencyLimit).To(BeNil())
		})

		It("rejects an update that sets the name", func() {
			created := createDeployment(&prefect.DeploymentSpec{Name: "daily", FlowID: flow.ID})
			req, err := http.NewRequest(http.MethodPatch, server.APIURL+"/deployments/"+created.ID,
				bytes.NewReader([]byte(`{"name": "renamed"}`)))
			Expect(err).NotTo(HaveOccurred())
			resp, err := http.DefaultClient.Do(req)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusUnprocessableEntity))
		})

		It("fills in schedule defaults and rejects a duplicate slug", func() {
			deployment := createDeployment(&prefect.DeploymentSpec{Name: "daily", FlowID: flow.ID})
			Expect(client.CreateDeploymentSchedules(ctx, deployment.ID, []prefect.DeploymentSchedule{
				{Slug: new("cron"), Schedule: prefect.Schedule{Cron: new("0 0 * * *")}},
				{Slug: new("interval"), Schedule: prefect.Schedule{Interval: new(3600.0)}},
			})).To(Succeed())

			read, err := client.GetDeployment(ctx, deployment.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(read.Schedules).To(HaveLen(2))
			cron, interval := read.Schedules[0], read.Schedules[1]
			Expect(cron.Active).To(HaveValue(BeTrue()))
			Expect(cron.Schedule.DayOr).To(HaveValue(BeTrue()))
			Expect(cron.Schedule.Timezone).To(BeNil())
			Expect(interval.Schedule.Timezone).To(HaveValue(Equal("UTC")))
			Expect(interval.Schedule.AnchorDate).NotTo(BeNil())

			err = client.CreateDeploymentSchedules(ctx, deployment.ID, []prefect.DeploymentSchedule{
				{Slug: new("cron"), Schedule: prefect.Schedule{Cron: new("0 1 * * *")}},
			})
			Expect(err).To(MatchError(ContainSubstring("status 409")))
		})

		It("leaves a schedule field unchanged when an update sets it to null", func() {
			deployment := createDeployment(&prefect.DeploymentSpec{Name: "daily", FlowID: flow.ID})
			Expect(client.CreateDeploymentSchedules(ctx, deployment.ID, []prefect.DeploymentSchedule{
				{Slug: new("cron"), Schedule: prefect.Schedule{Cron: new("0 0 * * *")}, MaxScheduledRuns: new(3)},
			})).To(Succeed())
			read, err := client.GetDeployment(ctx, deployment.ID)
			Expect(err).NotTo(HaveOccurred())

			req, err := http.NewRequest(http.MethodPatch,
				server.APIURL+"/deployments/"+deployment.ID+"/schedules/"+read.Schedules[0].ID,
				bytes.NewReader([]byte(`{"max_scheduled_runs": null, "active": false}`)))
			Expect(err).NotTo(HaveOccurred())
			resp, err := http.DefaultClient.Do(req)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusNoContent))

			read, err = client.GetDeployment(ctx, deployment.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(read.Schedules[0].MaxScheduledRuns).To(HaveValue(Equal(3)))
			Expect(read.Schedules[0].Active).To(HaveValue(BeFalse()))
		})
	})

	Describe("work pools and queues", func() {
		It("creates a pool NOT_READY with a default queue", func() {
			pool := createPool("pool")
			Expect(pool.Status).To(Equal("NOT_READY"))

			queue, err := client.GetWorkQueue(ctx, "pool", "default")
			Expect(err).NotTo(HaveOccurred())
			Expect(pool.DefaultQueueID).To(HaveValue(Equal(queue.ID)))
			Expect(queue.Priority).To(HaveValue(Equal(int32(1))))
		})

		It("rejects a duplicate or reserved pool name", func() {
			createPool("pool")
			_, err := client.CreateWorkPool(ctx, &prefect.WorkPoolSpec{Name: "pool"})
			Expect(err).To(MatchError(ContainSubstring("status 409")))
			_, err = client.CreateWorkPool(ctx, &prefect.WorkPoolSpec{Name: "Prefect-pool"})
			Expect(err).To(MatchError(ContainSubstring("status 403")))
		})

		It("gives a new queue the next free priority", func() {
			createPool("pool")
			queue, err := client.CreateWorkQueue(ctx, "pool", &prefect.WorkQueueSpec{Name: "ingest"})
			Expect(err).NotTo(HaveOccurred())
			Expect(queue.Priority).To(HaveValue(Equal(int32(2))))
		})

		It("refuses to delete the pool's default queue", func() {
			createPool("pool")
			Expect(client.DeleteWorkQueue(ctx, "pool", "default")).To(MatchError(ContainSubstring("status 500")))
		})

		It("deletes a pool's queues with the pool", func() {
			createPool("pool")
			Expect(client.DeleteWorkPool(ctx, "pool")).To(Succeed())
			createPool("pool")
			_, err := client.CreateWorkQueue(ctx, "pool", &prefect.WorkQueueSpec{Name: "ingest"})
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("automations", func() {
		It("replaces the automation on update and gives the trigger a new ID", func() {
			created, err := client.CreateAutomation(ctx, &prefect.AutomationSpec{
				Name:        "alert",
				Description: "page on failure",
				Trigger:     map[string]any{"type": "event"},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(created.Trigger).To(HaveKey("id"))

			updated, err := client.UpdateAutomation(ctx, created.ID, &prefect.AutomationSpec{
				Name:    "alert",
				Trigger: map[string]any{"type": "event"},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(updated.ID).To(Equal(created.ID))
			Expect(updated.Description).To(BeEmpty())
			Expect(updated.Trigger["id"]).NotTo(Equal(created.Trigger["id"]))
		})
	})
})
