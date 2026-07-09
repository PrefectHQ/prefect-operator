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
	"testing"

	prefectiov1 "github.com/PrefectHQ/prefect-operator/api/v1"
)

// UpdateAutomationStatus maps the Prefect API result onto the CR status (Id +
// Ready). It does NOT stamp LastSyncTime: the controller owns that, stamping it
// only after a successful sync so needsSync can gate the next re-check by the
// resync interval. This test pins that division of responsibility.
func TestUpdateAutomationStatusDoesNotStampLastSyncTime(t *testing.T) {
	k8s := &prefectiov1.PrefectAutomation{}
	UpdateAutomationStatus(k8s, &Automation{ID: "abc"})

	if k8s.Status.Id == nil || *k8s.Status.Id != "abc" {
		t.Fatalf("Status.Id = %v, want \"abc\"", k8s.Status.Id)
	}
	if !k8s.Status.Ready {
		t.Fatal("Status.Ready = false, want true")
	}
	if k8s.Status.LastSyncTime != nil {
		t.Fatal("UpdateAutomationStatus must not stamp LastSyncTime; the controller owns it")
	}
}
