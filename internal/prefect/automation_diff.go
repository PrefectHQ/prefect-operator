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

// setKeys are trigger fields the server models as sets: it stores and returns
// their elements in arbitrary order, so element order must not count as drift.
var setKeys = map[string]bool{keyExpect: true, keyAfter: true, keyForEach: true}

// subsetMatches reports whether every field present in desired equals its
// remote counterpart; remote-only keys don't count as drift. Maps recurse per
// key and slices compare elementwise, except set-typed trigger fields which
// compare as multisets.
func subsetMatches(desired, remote any) bool {
	switch d := desired.(type) {
	case map[string]any:
		r, ok := remote.(map[string]any)
		if !ok {
			return false
		}
		for k, dv := range d {
			if setKeys[k] {
				if !unorderedMatches(dv, r[k]) {
					return false
				}
				continue
			}
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
