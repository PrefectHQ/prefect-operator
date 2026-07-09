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

	prefectiov1 "github.com/PrefectHQ/prefect-operator/api/v1"
)

// UpdateAutomationStatus never stamps LastSyncTime, so needsSync sees a nil
// LastSyncTime on every pass and returns true — the controller reconciles
// against Prefect each loop and self-heals an out-of-band edit/delete within one
// requeue interval, matching PrefectDeployment's continuous reconcile.
func TestAutomationNeedsSync(t *testing.T) {
	r := &PrefectAutomationReconciler{}
	id := "automation-1"
	hash := "hash-1"

	// Steady state as persisted by UpdateAutomationStatus: id + spec hash +
	// observed generation set, but LastSyncTime left nil.
	steady := &prefectiov1.PrefectAutomation{}
	steady.Generation = 2
	steady.Status.Id = &id
	steady.Status.SpecHash = hash
	steady.Status.ObservedGeneration = 2

	if !r.needsSync(steady, hash) {
		t.Fatal("needsSync = false in steady state; want true so out-of-band changes self-heal every loop")
	}
}
