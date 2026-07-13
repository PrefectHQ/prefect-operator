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
