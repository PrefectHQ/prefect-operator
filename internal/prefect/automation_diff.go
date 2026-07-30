package prefect

import (
	"encoding/json"
	"reflect"
)

// AutomationUpToDate reports whether the remote automation already matches
// every operator-managed field of the desired spec, so the update PUT (which
// resets the trigger's ID and therefore its proactive state) can be skipped.
// Remote-only fields (trigger IDs, server defaults) are ignored.
func AutomationUpToDate(remote *Automation, desired *AutomationSpec) bool {
	if remote == nil || desired == nil {
		return false
	}
	if remote.Name != desired.Name || remote.Description != desired.Description {
		return false
	}
	if desired.Enabled != nil && *desired.Enabled != remote.Enabled {
		return false
	}
	if !subsetMatches(normalizeJSON(desired.Trigger), normalizeJSON(remote.Trigger)) {
		return false
	}
	return actionsMatch(desired.Actions, remote.Actions) &&
		actionsMatch(desired.ActionsOnTrigger, remote.ActionsOnTrigger) &&
		actionsMatch(desired.ActionsOnResolve, remote.ActionsOnResolve)
}

// actionsMatch compares action lists pairwise. A nil desired list matches an
// empty remote one: the PUT payload replaces omitted lists with [].
func actionsMatch(desired, remote []map[string]any) bool {
	if len(desired) != len(remote) {
		return false
	}
	for i := range desired {
		if !subsetMatches(normalizeJSON(desired[i]), normalizeJSON(remote[i])) {
			return false
		}
	}
	return true
}

// subsetMatches reports whether every field present in desired equals its
// remote counterpart; remote-only keys don't count as drift. Maps recurse per
// key and slices compare elementwise.
func subsetMatches(desired, remote any) bool {
	switch d := desired.(type) {
	case map[string]any:
		r, ok := remote.(map[string]any)
		if !ok {
			return false
		}
		for k, dv := range d {
			if !subsetMatches(dv, r[k]) {
				return false
			}
		}
		return true
	case []any:
		r, ok := remote.([]any)
		if !ok || len(d) != len(r) {
			return false
		}
		for i := range d {
			if !subsetMatches(d[i], r[i]) {
				return false
			}
		}
		return true
	default:
		return reflect.DeepEqual(desired, remote)
	}
}

// normalizeJSON round-trips a value through JSON so Go-typed desired values
// (int thresholds, []string lists) compare against the API's decoding
// (float64 numbers, []any).
func normalizeJSON(v any) any {
	if v == nil {
		return nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return v
	}
	var out any
	if err := json.Unmarshal(b, &out); err != nil {
		return v
	}
	return out
}
