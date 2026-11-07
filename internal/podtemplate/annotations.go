package podtemplate

import (
	"maps"
	"slices"
	"strings"
)

// ManagedAnnotationsAnnotation is set on a Deployment, not on its pod template,
// to record which pod template annotations the operator set. Recording it there
// means updating it doesn't roll out new pods.
const ManagedAnnotationsAnnotation = "prefect.io/pod-template-annotations"

// MergeAnnotations returns the pod template annotations to apply and the new
// value of ManagedAnnotationsAnnotation. It starts from the live annotations,
// removes the ones the operator set before (listed in managed) that desired no
// longer has, and sets desired. Annotations that other tools add, such as the
// one `kubectl rollout restart` sets, are kept, so the operator doesn't undo
// what they did and roll out new pods again.
func MergeAnnotations(live, desired map[string]string, managed string) (map[string]string, string) {
	annotations := maps.Clone(live)
	for _, key := range strings.Split(managed, ",") {
		if _, ok := desired[key]; !ok {
			delete(annotations, key)
		}
	}

	if len(desired) > 0 && annotations == nil {
		annotations = map[string]string{}
	}
	maps.Copy(annotations, desired)
	if len(annotations) == 0 {
		annotations = nil
	}

	return annotations, strings.Join(slices.Sorted(maps.Keys(desired)), ",")
}
