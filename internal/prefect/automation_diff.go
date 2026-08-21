package prefect

import (
	"encoding/json"
	"reflect"
	"sort"
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
	if !triggerMatches(normalizeJSON(desired.Trigger), normalizeJSON(remote.Trigger)) {
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

// unorderedTriggerKeys are event-trigger fields that the server stores as sets.
var unorderedTriggerKeys = map[string]bool{keyExpect: true, keyAfter: true, keyForEach: true}

// triggerMatches compares a trigger and recurses into its child triggers.
// Event-trigger set fields compare as multisets.
func triggerMatches(desired, remote any) bool {
	d, ok := desired.(map[string]any)
	if !ok {
		return subsetMatches(desired, remote)
	}
	r, ok := remote.(map[string]any)
	if !ok {
		return false
	}
	isEventTrigger := d[keyType] == triggerTypeEvent
	for k, dv := range d {
		switch {
		case isEventTrigger && unorderedTriggerKeys[k]:
			if !unorderedMatches(dv, r[k]) {
				return false
			}
		case k == keyTriggers:
			if !triggerListMatches(dv, r[k]) {
				return false
			}
		default:
			if !subsetMatches(dv, r[k]) {
				return false
			}
		}
	}
	return true
}

func triggerListMatches(desired, remote any) bool {
	d, dok := desired.([]any)
	r, rok := remote.([]any)
	if !dok || !rok || len(d) != len(r) {
		return false
	}
	for i := range d {
		if !triggerMatches(d[i], r[i]) {
			return false
		}
	}
	return true
}

// subsetMatches reports whether every field present in desired equals its
// remote counterpart. Remote-only keys do not count as drift. Maps recurse per
// key, and slices compare elementwise.
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

// unorderedMatches compares two slices as multisets of their JSON encodings;
// non-slice values fall back to ordered matching.
func unorderedMatches(desired, remote any) bool {
	d, dok := desired.([]any)
	r, rok := remote.([]any)
	if !dok || !rok {
		return subsetMatches(desired, remote)
	}
	if len(d) != len(r) {
		return false
	}
	dk, rk := jsonSorted(d), jsonSorted(r)
	for i := range dk {
		if dk[i] != rk[i] {
			return false
		}
	}
	return true
}

func jsonSorted(items []any) []string {
	keys := make([]string, 0, len(items))
	for _, it := range items {
		b, err := json.Marshal(it)
		if err != nil {
			keys = append(keys, "")
			continue
		}
		keys = append(keys, string(b))
	}
	sort.Strings(keys)
	return keys
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
