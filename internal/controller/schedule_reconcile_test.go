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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/PrefectHQ/prefect-operator/internal/prefect"
)

var _ = Describe("PrefectDeployment schedule reconciliation", func() {
	var (
		ctx   context.Context
		r     *PrefectDeploymentReconciler
		mock  *prefect.MockClient
		depID string
	)

	newString := func(s string) *string { return &s }
	newBool := func(b bool) *bool { return &b }
	newInt := func(i int) *int { return &i }
	newFloat := func(f float64) *float64 { return &f }
	newTime := func(t time.Time) *time.Time { return &t }

	BeforeEach(func() {
		ctx = context.Background()
		r = &PrefectDeploymentReconciler{}
		mock = prefect.NewMockClient()
		dep, err := mock.CreateOrUpdateDeployment(ctx, &prefect.DeploymentSpec{Name: "d", FlowID: "f"})
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
		Expect(r.reconcileSchedules(ctx, mock, depID, desired)).To(Succeed())
		got, err := mock.GetDeployment(ctx, depID)
		Expect(err).NotTo(HaveOccurred())
		Expect(got.Schedules).To(HaveLen(1))
		schedID := got.Schedules[0].ID

		By("leaving the ID unchanged on an unchanged reconcile")
		Expect(r.reconcileSchedules(ctx, mock, depID, desired)).To(Succeed())
		got, err = mock.GetDeployment(ctx, depID)
		Expect(err).NotTo(HaveOccurred())
		Expect(got.Schedules).To(HaveLen(1))
		Expect(got.Schedules[0].ID).To(Equal(schedID))

		By("updating in place when the cron changes")
		desired[0].Schedule.Cron = newString("*/5 * * * *")
		Expect(r.reconcileSchedules(ctx, mock, depID, desired)).To(Succeed())
		got, err = mock.GetDeployment(ctx, depID)
		Expect(err).NotTo(HaveOccurred())
		Expect(got.Schedules).To(HaveLen(1))
		Expect(got.Schedules[0].ID).To(Equal(schedID))
		Expect(*got.Schedules[0].Schedule.Cron).To(Equal("*/5 * * * *"))

		By("deleting when no longer desired")
		Expect(r.reconcileSchedules(ctx, mock, depID, nil)).To(Succeed())
		got, err = mock.GetDeployment(ctx, depID)
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

		Expect(r.reconcileSchedules(ctx, mock, depID, desired)).To(Succeed())
		Expect(r.reconcileSchedules(ctx, mock, depID, desired)).To(Succeed())
		Expect(mock.UpdateScheduleCalls).To(BeZero())
	})

	It("applies a maxScheduledRuns-only change in place", func() {
		desired := []prefect.DeploymentSchedule{{
			Slug:             newString("default"),
			Active:           newBool(true),
			MaxScheduledRuns: newInt(3),
			Schedule:         prefect.Schedule{Cron: newString("* * * * *")},
		}}
		Expect(r.reconcileSchedules(ctx, mock, depID, desired)).To(Succeed())
		got, err := mock.GetDeployment(ctx, depID)
		Expect(err).NotTo(HaveOccurred())
		schedID := got.Schedules[0].ID

		desired[0].MaxScheduledRuns = newInt(5)
		Expect(r.reconcileSchedules(ctx, mock, depID, desired)).To(Succeed())

		got, err = mock.GetDeployment(ctx, depID)
		Expect(err).NotTo(HaveOccurred())
		Expect(got.Schedules).To(HaveLen(1))
		Expect(got.Schedules[0].ID).To(Equal(schedID))
		Expect(*got.Schedules[0].MaxScheduledRuns).To(Equal(5))
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
		Expect(r.reconcileSchedules(ctx, mock, depID, desired)).To(Succeed())
		got, err := mock.GetDeployment(ctx, depID)
		Expect(err).NotTo(HaveOccurred())
		schedID := got.Schedules[0].ID

		By("not updating when the anchor is the same instant in another zone")
		desired[0].Schedule.AnchorDate = newTime(anchor.In(time.FixedZone("UTC+5", 5*60*60)))
		Expect(r.reconcileSchedules(ctx, mock, depID, desired)).To(Succeed())
		Expect(mock.UpdateScheduleCalls).To(BeZero())

		By("updating in place when the anchor instant changes")
		newAnchor := anchor.Add(30 * time.Minute)
		desired[0].Schedule.AnchorDate = newTime(newAnchor)
		Expect(r.reconcileSchedules(ctx, mock, depID, desired)).To(Succeed())

		got, err = mock.GetDeployment(ctx, depID)
		Expect(err).NotTo(HaveOccurred())
		Expect(got.Schedules).To(HaveLen(1))
		Expect(got.Schedules[0].ID).To(Equal(schedID))
		Expect(got.Schedules[0].Schedule.AnchorDate.Equal(newAnchor)).To(BeTrue())
	})

	It("does not update an interval schedule when the CR sets no anchorDate but the server defaulted one", func() {
		By("seeding a schedule with a server-defaulted anchor date")
		Expect(mock.CreateDeploymentSchedules(ctx, depID, []prefect.DeploymentSchedule{{
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
		Expect(r.reconcileSchedules(ctx, mock, depID, desired)).To(Succeed())
		Expect(mock.UpdateScheduleCalls).To(BeZero())
	})

	It("prunes a schedule with no slug that isn't part of the desired set", func() {
		By("seeding a slugless schedule directly on the deployment")
		Expect(mock.CreateDeploymentSchedules(ctx, depID, []prefect.DeploymentSchedule{{
			Schedule: prefect.Schedule{Cron: newString("0 * * * *")},
		}})).To(Succeed())

		By("reconciling to a single slugged schedule")
		desired := []prefect.DeploymentSchedule{{
			Slug:     newString("default"),
			Active:   newBool(true),
			Schedule: prefect.Schedule{Cron: newString("* * * * *")},
		}}
		Expect(r.reconcileSchedules(ctx, mock, depID, desired)).To(Succeed())

		got, err := mock.GetDeployment(ctx, depID)
		Expect(err).NotTo(HaveOccurred())
		Expect(got.Schedules).To(HaveLen(1))
		Expect(got.Schedules[0].Slug).To(Equal(newString("default")))
	})
})
