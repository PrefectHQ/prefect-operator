/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
*/

package controller

import (
	"context"
	"testing"

	"github.com/PrefectHQ/prefect-operator/internal/prefect"
)

func TestReconcileSchedules(t *testing.T) {
	ctx := context.Background()
	r := &PrefectDeploymentReconciler{}
	mock := prefect.NewMockClient()

	dep, err := mock.CreateOrUpdateDeployment(ctx, &prefect.DeploymentSpec{Name: "d", FlowID: "f"})
	if err != nil {
		t.Fatalf("seed deployment: %v", err)
	}
	depID := dep.ID

	cron := "* * * * *"
	slug := "default"
	active := true
	desired := []prefect.DeploymentSchedule{{
		Slug:     &slug,
		Active:   &active,
		Schedule: prefect.Schedule{Cron: &cron},
	}}

	if err := r.reconcileSchedules(ctx, mock, depID, desired); err != nil {
		t.Fatalf("create: %v", err)
	}
	got, _ := mock.GetDeployment(ctx, depID)
	if len(got.Schedules) != 1 {
		t.Fatalf("want 1 schedule after create, got %d", len(got.Schedules))
	}
	schedID := got.Schedules[0].ID

	if err := r.reconcileSchedules(ctx, mock, depID, desired); err != nil {
		t.Fatalf("noop: %v", err)
	}
	got2, _ := mock.GetDeployment(ctx, depID)
	if len(got2.Schedules) != 1 || got2.Schedules[0].ID != schedID {
		t.Fatalf("schedule ID churned on unchanged reconcile: %+v", got2.Schedules)
	}

	newCron := "*/5 * * * *"
	desired[0].Schedule.Cron = &newCron
	if err := r.reconcileSchedules(ctx, mock, depID, desired); err != nil {
		t.Fatalf("update: %v", err)
	}
	got3, _ := mock.GetDeployment(ctx, depID)
	if len(got3.Schedules) != 1 || got3.Schedules[0].ID != schedID {
		t.Fatalf("update should keep the same schedule ID: %+v", got3.Schedules)
	}
	if got3.Schedules[0].Schedule.Cron == nil || *got3.Schedules[0].Schedule.Cron != newCron {
		t.Fatalf("cron not updated in place: %+v", got3.Schedules[0].Schedule)
	}

	if err := r.reconcileSchedules(ctx, mock, depID, nil); err != nil {
		t.Fatalf("delete: %v", err)
	}
	got4, _ := mock.GetDeployment(ctx, depID)
	if len(got4.Schedules) != 0 {
		t.Fatalf("want 0 schedules after delete, got %d", len(got4.Schedules))
	}
}
