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
