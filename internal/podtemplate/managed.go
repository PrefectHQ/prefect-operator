package podtemplate

import (
	"maps"
	"slices"
	"strings"
)

// These annotations are set on a Deployment, not on its pod template, to record
// which pod template labels and annotations the operator set. Recording them
// there means updating them doesn't roll out new pods.
const (
	ManagedLabelsAnnotation      = "prefect.io/pod-template-labels"
	ManagedAnnotationsAnnotation = "prefect.io/pod-template-annotations"
)

// MergeManaged returns the pod template labels or annotations to apply, and
// the new list of the keys the operator set. It starts from live, removes the
// keys the operator set before (listed in managed) that desired no longer has,
// and sets desired. Keys that other tools add, such as the annotation
// `kubectl rollout restart` sets, are kept, so the operator doesn't undo what
// they did and roll out new pods again.
func MergeManaged(live, desired map[string]string, managed string) (map[string]string, string) {
	merged := maps.Clone(live)
	for _, key := range strings.Split(managed, ",") {
		if _, ok := desired[key]; !ok {
			delete(merged, key)
		}
	}

	if len(desired) > 0 && merged == nil {
		merged = map[string]string{}
	}
	maps.Copy(merged, desired)
	if len(merged) == 0 {
		merged = nil
	}

	return merged, strings.Join(slices.Sorted(maps.Keys(desired)), ",")
}
