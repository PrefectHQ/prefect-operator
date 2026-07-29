package prefect

import "testing"

// remoteFromSpec simulates the API's view of a just-synced automation: same
// content as the spec, decoded from JSON (float64 numbers, []any slices), with
// server-assigned trigger ids and defaults layered on top.
func remoteFromSpec(t *testing.T, spec *AutomationSpec) *Automation {
	t.Helper()
	trigger, ok := normalizeJSON(spec.Trigger).(map[string]any)
	if !ok {
		t.Fatal("trigger did not normalize to a map")
	}
	trigger["id"] = "11111111-1111-1111-1111-111111111111"
	remote := &Automation{
		Name:        spec.Name,
		Description: spec.Description,
		Trigger:     trigger,
		Actions:     []map[string]any{},
	}
	if spec.Enabled != nil {
		remote.Enabled = *spec.Enabled
	}
	for _, a := range spec.Actions {
		remote.Actions = append(remote.Actions, normalizeJSON(a).(map[string]any))
	}
	return remote
}

func zombieSpec() *AutomationSpec {
	enabled := true
	return &AutomationSpec{
		Name:        "zombie-flow-detection",
		Description: "Marks flow runs as Crashed when heartbeats stop arriving.",
		Enabled:     &enabled,
		Trigger: map[string]any{
			keyType:         "event",
			"posture":       "Proactive",
			"match":         map[string]any{"prefect.resource.id": "prefect.flow-run.*"},
			"match_related": map[string]any{},
			"after":         []string{"prefect.flow-run.heartbeat"},
			"expect":        []string{"prefect.flow-run.*"},
			"for_each":      []string{"prefect.resource.id"},
			"threshold":     1,
			"within":        540,
		},
		Actions: []map[string]any{{
			keyType:   "change-flow-run-state",
			"state":   "CRASHED",
			"message": "no heartbeat",
		}},
	}
}

func TestAutomationUpToDate(t *testing.T) {
	t.Run("matches despite remote trigger id, JSON number types and server-filled action defaults", func(t *testing.T) {
		spec := zombieSpec()
		remote := remoteFromSpec(t, spec)
		// The server echoes change-flow-run-state actions with fields the spec
		// never sends (verified live on Prefect 3.6.28).
		remote.Actions[0]["name"] = nil
		remote.Actions[0]["force"] = false
		if !AutomationUpToDate(remote, spec) {
			t.Fatal("up to date = false for an identical automation; want true")
		}
	})

	t.Run("detects a trigger field change", func(t *testing.T) {
		spec := zombieSpec()
		remote := remoteFromSpec(t, spec)
		remote.Trigger["within"] = float64(300)
		if AutomationUpToDate(remote, spec) {
			t.Fatal("up to date = true with a different trigger.within; want false")
		}
	})

	t.Run("detects an action change", func(t *testing.T) {
		spec := zombieSpec()
		remote := remoteFromSpec(t, spec)
		remote.Actions[0]["state"] = "CANCELLED"
		if AutomationUpToDate(remote, spec) {
			t.Fatal("up to date = true with a different action state; want false")
		}
	})

	t.Run("detects name, description and enabled changes", func(t *testing.T) {
		spec := zombieSpec()

		remote := remoteFromSpec(t, spec)
		remote.Name = "renamed"
		if AutomationUpToDate(remote, spec) {
			t.Fatal("up to date = true with a different name; want false")
		}

		remote = remoteFromSpec(t, spec)
		remote.Description = "changed"
		if AutomationUpToDate(remote, spec) {
			t.Fatal("up to date = true with a different description; want false")
		}

		remote = remoteFromSpec(t, spec)
		remote.Enabled = false
		if AutomationUpToDate(remote, spec) {
			t.Fatal("up to date = true with a different enabled; want false")
		}
	})

	t.Run("detects an action count change", func(t *testing.T) {
		spec := zombieSpec()
		remote := remoteFromSpec(t, spec)
		remote.Actions = append(remote.Actions, map[string]any{keyType: "do-nothing"})
		if AutomationUpToDate(remote, spec) {
			t.Fatal("up to date = true with an extra remote action; want false")
		}
	})

	t.Run("nil desired action lists match empty remote lists", func(t *testing.T) {
		spec := zombieSpec()
		remote := remoteFromSpec(t, spec)
		remote.ActionsOnTrigger = []map[string]any{}
		remote.ActionsOnResolve = nil
		if !AutomationUpToDate(remote, spec) {
			t.Fatal("up to date = false with nil-vs-empty action lists; want true")
		}
	})
}

func TestPreserveTriggerIDs(t *testing.T) {
	t.Run("copies the remote trigger id", func(t *testing.T) {
		desired := zombieSpec().Trigger
		remote := map[string]any{keyType: "event", "id": "trigger-id-1"}
		PreserveTriggerIDs(desired, remote)
		if desired["id"] != "trigger-id-1" {
			t.Fatalf("desired id = %v; want trigger-id-1", desired["id"])
		}
	})

	t.Run("does not copy across a trigger type change", func(t *testing.T) {
		desired := map[string]any{keyType: "compound"}
		remote := map[string]any{keyType: "event", "id": "trigger-id-1"}
		PreserveTriggerIDs(desired, remote)
		if _, ok := desired["id"]; ok {
			t.Fatal("id copied across a trigger type change; want fresh identity")
		}
	})

	t.Run("copies compound child trigger ids pairwise", func(t *testing.T) {
		desired := map[string]any{
			keyType: "compound",
			"triggers": []map[string]any{
				{keyType: "event"},
				{keyType: "metric"},
			},
		}
		remote := map[string]any{
			keyType: "compound",
			"id":    "parent-id",
			// JSON decoding yields []any of maps.
			"triggers": []any{
				map[string]any{keyType: "event", "id": "child-1"},
				map[string]any{keyType: "metric", "id": "child-2"},
			},
		}
		PreserveTriggerIDs(desired, remote)
		children := desired["triggers"].([]map[string]any)
		if desired["id"] != "parent-id" || children[0]["id"] != "child-1" || children[1]["id"] != "child-2" {
			t.Fatalf("ids not carried over: parent=%v children=%v,%v",
				desired["id"], children[0]["id"], children[1]["id"])
		}
	})
}
