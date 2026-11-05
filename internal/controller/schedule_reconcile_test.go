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

package controller

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/PrefectHQ/prefect-operator/internal/prefect"
	"github.com/PrefectHQ/prefect-operator/internal/prefect/prefecttest"
)

const (
	updateScheduleRoute = "PATCH /deployments/{id}/schedules/{schedule_id}"
	deleteScheduleRoute = "DELETE /deployments/{id}/schedules/{schedule_id}"
)

var _ = Describe("PrefectDeployment schedule reconciliation", func() {
	var (
		ctx        context.Context
		r          *PrefectDeploymentReconciler
		prefectAPI *prefecttest.Server
		client     *prefect.Client
		depID      string
	)

	newString := func(s string) *string { return &s }
	newBool := func(b bool) *bool { return &b }
	newInt := func(i int) *int { return &i }
	newFloat := func(f float64) *float64 { return &f }
	newTime := func(t time.Time) *time.Time { return &t }

	BeforeEach(func() {
		ctx = context.Background()
		r = &PrefectDeploymentReconciler{}
		prefectAPI = prefecttest.NewServer()
		DeferCleanup(prefectAPI.Close)
		client = prefect.NewClient(prefectAPI.APIURL, "", logr.Discard())
		flow, err := client.CreateOrGetFlow(ctx, &prefect.FlowSpec{Name: "f"})
		Expect(err).NotTo(HaveOccurred())
		dep, err := client.CreateOrUpdateDeployment(ctx, &prefect.DeploymentSpec{Name: "d", FlowID: flow.ID})
		Expect(err).NotTo(HaveOccurred())
		depID = dep.ID
	})

	It("creates, updates in place, and deletes by slug while keeping the schedule ID stable", func() {
		desired := []prefect.DeploymentSchedule{{
			Slug:     newString("default"),
			Active:   newBool(true),
			Schedule: prefect.Schedule{Cron: newString("* * * * *")},
		}}

		By("creating when absent")
		Expect(r.reconcileSchedules(ctx, client, depID, desired)).To(Succeed())
		got, err := client.GetDeployment(ctx, depID)
		Expect(err).NotTo(HaveOccurred())
		Expect(got.Schedules).To(HaveLen(1))
		schedID := got.Schedules[0].ID

		By("leaving the ID unchanged on an unchanged reconcile")
		Expect(r.reconcileSchedules(ctx, client, depID, desired)).To(Succeed())
		got, err = client.GetDeployment(ctx, depID)
		Expect(err).NotTo(HaveOccurred())
		Expect(got.Schedules).To(HaveLen(1))
		Expect(got.Schedules[0].ID).To(Equal(schedID))

		By("updating in place when the cron changes")
		desired[0].Schedule.Cron = newString("*/5 * * * *")
		Expect(r.reconcileSchedules(ctx, client, depID, desired)).To(Succeed())
		got, err = client.GetDeployment(ctx, depID)
		Expect(err).NotTo(HaveOccurred())
		Expect(got.Schedules).To(HaveLen(1))
		Expect(got.Schedules[0].ID).To(Equal(schedID))
		Expect(*got.Schedules[0].Schedule.Cron).To(Equal("*/5 * * * *"))

		By("deleting when no longer desired")
		Expect(r.reconcileSchedules(ctx, client, depID, nil)).To(Succeed())
		got, err = client.GetDeployment(ctx, depID)
		Expect(err).NotTo(HaveOccurred())
		Expect(got.Schedules).To(BeEmpty())
	})

	It("makes no update calls when schedules are unchanged", func() {
		desired := []prefect.DeploymentSchedule{{
			Slug:             newString("default"),
			Active:           newBool(true),
			MaxScheduledRuns: newInt(3),
			Schedule:         prefect.Schedule{Cron: newString("* * * * *")},
		}}

		Expect(r.reconcileSchedules(ctx, client, depID, desired)).To(Succeed())
		Expect(r.reconcileSchedules(ctx, client, depID, desired)).To(Succeed())
		Expect(prefectAPI.Calls(updateScheduleRoute)).To(BeZero())
	})

	It("applies a maxScheduledRuns-only change in place", func() {
		desired := []prefect.DeploymentSchedule{{
			Slug:             newString("default"),
			Active:           newBool(true),
			MaxScheduledRuns: newInt(3),
			Schedule:         prefect.Schedule{Cron: newString("* * * * *")},
		}}
		Expect(r.reconcileSchedules(ctx, client, depID, desired)).To(Succeed())
		got, err := client.GetDeployment(ctx, depID)
		Expect(err).NotTo(HaveOccurred())
		schedID := got.Schedules[0].ID

		desired[0].MaxScheduledRuns = newInt(5)
		Expect(r.reconcileSchedules(ctx, client, depID, desired)).To(Succeed())

		got, err = client.GetDeployment(ctx, depID)
		Expect(err).NotTo(HaveOccurred())
		Expect(got.Schedules).To(HaveLen(1))
		Expect(got.Schedules[0].ID).To(Equal(schedID))
		Expect(*got.Schedules[0].MaxScheduledRuns).To(Equal(5))
	})

	It("applies a parameters-only change in place", func() {
		desired := []prefect.DeploymentSchedule{{
			Slug:       newString("default"),
			Active:     newBool(true),
			Parameters: map[string]any{"foo": "bar"},
			Schedule:   prefect.Schedule{Cron: newString("* * * * *")},
		}}
		Expect(r.reconcileSchedules(ctx, client, depID, desired)).To(Succeed())
		got, err := client.GetDeployment(ctx, depID)
		Expect(err).NotTo(HaveOccurred())
		schedID := got.Schedules[0].ID

		By("not updating when the parameters are unchanged")
		Expect(r.reconcileSchedules(ctx, client, depID, desired)).To(Succeed())
		Expect(prefectAPI.Calls(updateScheduleRoute)).To(BeZero())

		By("updating in place when the parameters change")
		desired[0].Parameters = map[string]any{"foo": "baz"}
		Expect(r.reconcileSchedules(ctx, client, depID, desired)).To(Succeed())

		got, err = client.GetDeployment(ctx, depID)
		Expect(err).NotTo(HaveOccurred())
		Expect(got.Schedules).To(HaveLen(1))
		Expect(got.Schedules[0].ID).To(Equal(schedID))
		Expect(got.Schedules[0].Parameters).To(Equal(map[string]any{"foo": "baz"}))
	})

	It("applies an anchorDate-only change to an interval schedule in place", func() {
		anchor := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		desired := []prefect.DeploymentSchedule{{
			Slug:   newString("hourly"),
			Active: newBool(true),
			Schedule: prefect.Schedule{
				Interval:   newFloat(3600),
				AnchorDate: newTime(anchor),
			},
		}}
		Expect(r.reconcileSchedules(ctx, client, depID, desired)).To(Succeed())
		got, err := client.GetDeployment(ctx, depID)
		Expect(err).NotTo(HaveOccurred())
		schedID := got.Schedules[0].ID

		By("not updating when the anchor is the same instant in another zone")
		desired[0].Schedule.AnchorDate = newTime(anchor.In(time.FixedZone("UTC+5", 5*60*60)))
		Expect(r.reconcileSchedules(ctx, client, depID, desired)).To(Succeed())
		Expect(prefectAPI.Calls(updateScheduleRoute)).To(BeZero())

		By("updating in place when the anchor instant changes")
		newAnchor := anchor.Add(30 * time.Minute)
		desired[0].Schedule.AnchorDate = newTime(newAnchor)
		Expect(r.reconcileSchedules(ctx, client, depID, desired)).To(Succeed())

		got, err = client.GetDeployment(ctx, depID)
		Expect(err).NotTo(HaveOccurred())
		Expect(got.Schedules).To(HaveLen(1))
		Expect(got.Schedules[0].ID).To(Equal(schedID))
		Expect(got.Schedules[0].Schedule.AnchorDate.Equal(newAnchor)).To(BeTrue())
	})

	It("does not update an interval schedule when the CR sets no anchorDate but the server defaulted one", func() {
		By("seeding a schedule with a server-defaulted anchor date")
		Expect(client.CreateDeploymentSchedules(ctx, depID, []prefect.DeploymentSchedule{{
			Slug:   newString("hourly"),
			Active: newBool(true),
			Schedule: prefect.Schedule{
				Interval:   newFloat(3600),
				AnchorDate: newTime(time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)),
			},
		}})).To(Succeed())

		desired := []prefect.DeploymentSchedule{{
			Slug:     newString("hourly"),
			Active:   newBool(true),
			Schedule: prefect.Schedule{Interval: newFloat(3600)},
		}}
		Expect(r.reconcileSchedules(ctx, client, depID, desired)).To(Succeed())
		Expect(prefectAPI.Calls(updateScheduleRoute)).To(BeZero())
	})

	anchor := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	DescribeTable("resets a field removed from the spec to Prefect's default",
		func(with, without prefect.DeploymentSchedule, check func(prefect.DeploymentSchedule)) {
			with.Slug, without.Slug = newString("default"), newString("default")
			Expect(r.reconcileSchedules(ctx, client, depID, []prefect.DeploymentSchedule{with})).To(Succeed())
			Expect(r.reconcileSchedules(ctx, client, depID, []prefect.DeploymentSchedule{without})).To(Succeed())

			got, err := client.GetDeployment(ctx, depID)
			Expect(err).NotTo(HaveOccurred())
			Expect(got.Schedules).To(HaveLen(1))
			check(got.Schedules[0])

			By("making no calls once Prefect matches the spec")
			calls := func() int {
				return prefectAPI.Calls(createSchedulesRoute) + prefectAPI.Calls(updateScheduleRoute) + prefectAPI.Calls(deleteScheduleRoute)
			}
			before := calls()
			Expect(r.reconcileSchedules(ctx, client, depID, []prefect.DeploymentSchedule{without})).To(Succeed())
			Expect(calls()).To(Equal(before))
		},
		Entry("active",
			prefect.DeploymentSchedule{Active: newBool(false), Schedule: prefect.Schedule{Cron: newString("0 9 * * *")}},
			prefect.DeploymentSchedule{Schedule: prefect.Schedule{Cron: newString("0 9 * * *")}},
			func(s prefect.DeploymentSchedule) { Expect(s.Active).To(HaveValue(BeTrue())) }),
		Entry("parameters",
			prefect.DeploymentSchedule{Parameters: map[string]any{"foo": "bar"}, Schedule: prefect.Schedule{Cron: newString("0 9 * * *")}},
			prefect.DeploymentSchedule{Schedule: prefect.Schedule{Cron: newString("0 9 * * *")}},
			func(s prefect.DeploymentSchedule) { Expect(s.Parameters).To(BeEmpty()) }),
		Entry("maxScheduledRuns",
			prefect.DeploymentSchedule{MaxScheduledRuns: newInt(3), Schedule: prefect.Schedule{Cron: newString("0 9 * * *")}},
			prefect.DeploymentSchedule{Schedule: prefect.Schedule{Cron: newString("0 9 * * *")}},
			func(s prefect.DeploymentSchedule) { Expect(s.MaxScheduledRuns).To(BeNil()) }),
		Entry("dayOr",
			prefect.DeploymentSchedule{Schedule: prefect.Schedule{Cron: newString("0 9 * * *"), DayOr: newBool(false)}},
			prefect.DeploymentSchedule{Schedule: prefect.Schedule{Cron: newString("0 9 * * *")}},
			func(s prefect.DeploymentSchedule) { Expect(s.Schedule.DayOr).To(HaveValue(BeTrue())) }),
		Entry("a cron schedule's timezone",
			prefect.DeploymentSchedule{Schedule: prefect.Schedule{Cron: newString("0 9 * * *"), Timezone: newString("America/Chicago")}},
			prefect.DeploymentSchedule{Schedule: prefect.Schedule{Cron: newString("0 9 * * *")}},
			func(s prefect.DeploymentSchedule) { Expect(s.Schedule.Timezone).To(BeNil()) }),
		Entry("an interval schedule's timezone",
			prefect.DeploymentSchedule{Schedule: prefect.Schedule{Interval: newFloat(3600), AnchorDate: newTime(anchor), Timezone: newString("America/Chicago")}},
			prefect.DeploymentSchedule{Schedule: prefect.Schedule{Interval: newFloat(3600), AnchorDate: newTime(anchor)}},
			func(s prefect.DeploymentSchedule) { Expect(s.Schedule.Timezone).To(HaveValue(Equal("UTC"))) }),
		Entry("an rrule schedule's timezone",
			prefect.DeploymentSchedule{Schedule: prefect.Schedule{RRule: newString("FREQ=DAILY"), Timezone: newString("America/Chicago")}},
			prefect.DeploymentSchedule{Schedule: prefect.Schedule{RRule: newString("FREQ=DAILY")}},
			func(s prefect.DeploymentSchedule) { Expect(s.Schedule.Timezone).To(HaveValue(Equal("UTC"))) }),
	)

	// Schedules as Prefect stores them when the spec sets only the schedule
	// itself.
	storedCron := prefect.DeploymentSchedule{
		Active: newBool(true), Parameters: map[string]any{},
		Schedule: prefect.Schedule{Cron: newString("0 9 * * *"), DayOr: newBool(true)},
	}
	storedInterval := prefect.DeploymentSchedule{
		Active: newBool(true), Parameters: map[string]any{},
		Schedule: prefect.Schedule{Interval: newFloat(3600), AnchorDate: newTime(anchor), Timezone: newString("UTC")},
	}
	storedRRule := prefect.DeploymentSchedule{
		Active: newBool(true), Parameters: map[string]any{},
		Schedule: prefect.Schedule{RRule: newString("FREQ=DAILY"), Timezone: newString("UTC")},
	}
	cron := prefect.Schedule{Cron: newString("0 9 * * *")}

	DescribeTable("scheduleNeedsUpdate",
		func(current, desired prefect.DeploymentSchedule, want bool) {
			Expect(scheduleNeedsUpdate(current, withScheduleDefaults(desired))).To(Equal(want))
		},
		Entry("an unchanged cron schedule", storedCron, prefect.DeploymentSchedule{Schedule: cron}, false),
		Entry("a cron schedule with no parameters in Prefect",
			prefect.DeploymentSchedule{Active: newBool(true), Schedule: storedCron.Schedule},
			prefect.DeploymentSchedule{Schedule: cron}, false),
		Entry("a changed cron expression", storedCron,
			prefect.DeploymentSchedule{Schedule: prefect.Schedule{Cron: newString("0 10 * * *")}}, true),
		Entry("a changed timezone", storedCron,
			prefect.DeploymentSchedule{Schedule: prefect.Schedule{Cron: cron.Cron, Timezone: newString("America/Chicago")}}, true),
		Entry("a changed dayOr", storedCron,
			prefect.DeploymentSchedule{Schedule: prefect.Schedule{Cron: cron.Cron, DayOr: newBool(false)}}, true),
		Entry("a changed active", storedCron, prefect.DeploymentSchedule{Active: newBool(false), Schedule: cron}, true),
		Entry("a changed maxScheduledRuns", storedCron, prefect.DeploymentSchedule{MaxScheduledRuns: newInt(3), Schedule: cron}, true),
		Entry("changed parameters", storedCron,
			prefect.DeploymentSchedule{Parameters: map[string]any{"foo": "bar"}, Schedule: cron}, true),
		Entry("a changed schedule type", storedCron,
			prefect.DeploymentSchedule{Schedule: prefect.Schedule{Interval: newFloat(3600)}}, true),
		Entry("an unchanged interval schedule with no anchor date in the spec", storedInterval,
			prefect.DeploymentSchedule{Schedule: prefect.Schedule{Interval: newFloat(3600)}}, false),
		Entry("a changed interval", storedInterval,
			prefect.DeploymentSchedule{Schedule: prefect.Schedule{Interval: newFloat(60)}}, true),
		Entry("a changed anchor date", storedInterval,
			prefect.DeploymentSchedule{Schedule: prefect.Schedule{Interval: newFloat(3600), AnchorDate: newTime(anchor.Add(time.Hour))}}, true),
		Entry("an unchanged rrule schedule", storedRRule,
			prefect.DeploymentSchedule{Schedule: prefect.Schedule{RRule: newString("FREQ=DAILY")}}, false),
		Entry("a changed rrule", storedRRule,
			prefect.DeploymentSchedule{Schedule: prefect.Schedule{RRule: newString("FREQ=WEEKLY")}}, true),
	)

	It("prunes a schedule with no slug that isn't part of the desired set", func() {
		By("seeding a slugless schedule directly on the deployment")
		Expect(client.CreateDeploymentSchedules(ctx, depID, []prefect.DeploymentSchedule{{
			Schedule: prefect.Schedule{Cron: newString("0 * * * *")},
		}})).To(Succeed())

		By("reconciling to a single slugged schedule")
		desired := []prefect.DeploymentSchedule{{
			Slug:     newString("default"),
			Active:   newBool(true),
			Schedule: prefect.Schedule{Cron: newString("* * * * *")},
		}}
		Expect(r.reconcileSchedules(ctx, client, depID, desired)).To(Succeed())

		got, err := client.GetDeployment(ctx, depID)
		Expect(err).NotTo(HaveOccurred())
		Expect(got.Schedules).To(HaveLen(1))
		Expect(got.Schedules[0].Slug).To(Equal(newString("default")))
	})
})
