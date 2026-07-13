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
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	prefectiov1 "github.com/PrefectHQ/prefect-operator/api/v1"
)

// needsSync gates the periodic Prefect re-check by the resync interval: it holds
// off while a fresh sync is within the interval and fires once the interval has
// elapsed, so out-of-band drift is corrected without hitting the API every loop.
func TestAutomationNeedsSync(t *testing.T) {
	const hash = "hash-1"
	id := "automation-1"

	// steady returns an automation in steady state (id + spec hash + observed
	// generation set) whose last sync was `sinceSync` ago.
	steady := func(sinceSync time.Duration) *prefectiov1.PrefectAutomation {
		a := &prefectiov1.PrefectAutomation{}
		a.Generation = 2
		a.Status.Id = &id
		a.Status.SpecHash = hash
		a.Status.ObservedGeneration = 2
		last := metav1.NewTime(time.Now().Add(-sinceSync))
		a.Status.LastSyncTime = &last
		return a
	}

	r := &PrefectAutomationReconciler{DefaultResyncInterval: 30 * time.Second}

	t.Run("holds off within the interval", func(t *testing.T) {
		if r.needsSync(steady(5*time.Second), hash) {
			t.Fatal("needsSync = true within the resync interval; want false")
		}
	})

	t.Run("fires after the interval elapses", func(t *testing.T) {
		if !r.needsSync(steady(45*time.Second), hash) {
			t.Fatal("needsSync = false after the resync interval; want true")
		}
	})

	t.Run("fires when the spec hash changed", func(t *testing.T) {
		if !r.needsSync(steady(1*time.Second), "different-hash") {
			t.Fatal("needsSync = false on spec change; want true")
		}
	})

	t.Run("fires when no id is set yet", func(t *testing.T) {
		a := steady(1 * time.Second)
		a.Status.Id = nil
		if !r.needsSync(a, hash) {
			t.Fatal("needsSync = false with no id; want true")
		}
	})

	t.Run("respects a per-resource spec.interval", func(t *testing.T) {
		a := steady(15 * time.Second)
		// A 5s spec.interval is below MinResyncInterval and clamps to 10s, so a
		// 15s-old sync is past due.
		a.Spec.Interval = &metav1.Duration{Duration: 5 * time.Second}
		if !r.needsSync(a, hash) {
			t.Fatal("needsSync = false past the clamped spec.interval; want true")
		}
	})
}
