// Package podtemplate applies user-supplied pod templates to the pod templates
// the operator builds.
//
// The operator builds a pod template from the typed fields of a resource, then
// applies the user's template to it as a Kubernetes strategic merge patch, so
// the user's template wins. Finally it restores the protected fields: the
// parts of the pod the operator itself depends on, such as the selector
// labels, the API port and the data volume. An override of a protected field
// is ignored rather than rejected.
package podtemplate

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	sigsjson "sigs.k8s.io/json"
)

// Protected names the parts of a pod template the operator always restores
// from the base after applying a user template.
type Protected struct {
	// Labels are pod template label keys, such as the Deployment's selector labels.
	Labels []string
	// Volumes are pod volume names.
	Volumes []string
	// Containers are keyed by container or init container name. A protected
	// container that the template deletes is put back whole.
	Containers map[string]ProtectedContainer
}

// ProtectedContainer names the parts of one container the operator restores.
type ProtectedContainer struct {
	// Ports are container port names.
	Ports []string
	// VolumeMounts are volume mount names. Any other mount at one of their
	// paths is dropped too, so the pod doesn't end up with two mounts at one path.
	VolumeMounts []string
	// Env are environment variable names.
	Env []string
}

// InvalidTemplateError reports a template that couldn't be applied.
type InvalidTemplateError struct {
	// Field is the path of the template in the resource, such as "spec.deploymentTemplate".
	Field string
	Err   error
}

func (e *InvalidTemplateError) Error() string {
	return fmt.Sprintf("invalid %s: %v", e.Field, e.Err)
}

func (e *InvalidTemplateError) Unwrap() error {
	return e.Err
}

// Apply applies template to base as a strategic merge patch, then restores the
// protected fields from base. A nil or empty template returns base unchanged.
// Errors are returned as an *InvalidTemplateError naming field.
func Apply(base corev1.PodTemplateSpec, template *runtime.RawExtension, field string, protected Protected) (corev1.PodTemplateSpec, error) {
	if template == nil || len(template.Raw) == 0 {
		return base, nil
	}

	invalid := func(err error) (corev1.PodTemplateSpec, error) {
		return corev1.PodTemplateSpec{}, &InvalidTemplateError{Field: field, Err: err}
	}

	baseJSON, err := json.Marshal(base)
	if err != nil {
		return invalid(err)
	}

	mergedJSON, err := strategicpatch.StrategicMergePatch(baseJSON, template.Raw, corev1.PodTemplateSpec{})
	if err != nil {
		return invalid(err)
	}

	// Decode strictly, as the API server does: an unknown field, or a field
	// name in the wrong case, is an error rather than silently dropped.
	var merged corev1.PodTemplateSpec
	strictErrs, err := sigsjson.UnmarshalStrict(mergedJSON, &merged)
	if err != nil {
		return invalid(err)
	}
	if len(strictErrs) > 0 {
		return invalid(errors.Join(strictErrs...))
	}

	if err := checkContainerNames(&merged.Spec); err != nil {
		return invalid(err)
	}

	var patched templateEnv
	if err := json.Unmarshal(template.Raw, &patched); err != nil {
		return invalid(err)
	}

	keepEnvOrder(merged.Spec.InitContainers, base.Spec.InitContainers)
	keepEnvOrder(merged.Spec.Containers, base.Spec.Containers)
	overrideDuplicateEnv(merged.Spec.InitContainers, patched.Spec.InitContainers)
	overrideDuplicateEnv(merged.Spec.Containers, patched.Spec.Containers)
	restore(&merged, &base, protected)
	return merged, nil
}

// FindContainer returns the container named name in spec.Containers, or nil.
func FindContainer(spec *corev1.PodSpec, name string) *corev1.Container {
	if i := findIn(spec.Containers, name); i >= 0 {
		return &spec.Containers[i]
	}
	return nil
}

// checkContainerNames reports a container name used more than once, such as an
// init container a template adds with the name of one of the operator's
// containers. Kubernetes would reject the pod template.
func checkContainerNames(spec *corev1.PodSpec) error {
	seen := map[string]bool{}
	for _, c := range slices.Concat(spec.InitContainers, spec.Containers) {
		if seen[c.Name] {
			return fmt.Errorf("container name %q is used more than once", c.Name)
		}
		seen[c.Name] = true
	}
	return nil
}

// templateEnv holds the environment variable names a template sets, per container.
type templateEnv struct {
	Spec struct {
		InitContainers []templateContainer `json:"initContainers"`
		Containers     []templateContainer `json:"containers"`
	} `json:"spec"`
}

type templateContainer struct {
	Name string `json:"name"`
	Env  []struct {
		Name  string `json:"name"`
		Patch string `json:"$patch"`
	} `json:"env"`
}

// overrideDuplicateEnv gives every entry of a variable the template sets the
// template's value. A setting with the same name as a variable the operator
// sets lists the name twice, and a strategic merge patch changes only the first
// entry, while Kubernetes uses the last one, so the template wouldn't win.
func overrideDuplicateEnv(merged []corev1.Container, patched []templateContainer) {
	for _, p := range patched {
		m := findIn(merged, p.Name)
		if m < 0 {
			continue
		}
		env := merged[m].Env
		for _, e := range p.Env {
			if e.Patch != "" {
				continue
			}
			first := slices.IndexFunc(env, func(v corev1.EnvVar) bool { return v.Name == e.Name })
			if first < 0 {
				continue
			}
			for i := first + 1; i < len(env); i++ {
				if env[i].Name == e.Name {
					env[i] = *env[first].DeepCopy()
				}
			}
		}
	}
}

// keepEnvOrder puts the environment variables of each merged container back in
// the order of the matching base container. A strategic merge patch groups
// variables that share a name, such as a setting that overrides a variable the
// operator sets, which can move a variable ahead of one it references with
// $(NAME). Variables the template added go after the base's, in their order.
func keepEnvOrder(merged, base []corev1.Container) {
	for i := range merged {
		b := findIn(base, merged[i].Name)
		if b < 0 {
			continue
		}

		remaining := slices.Clone(merged[i].Env)
		ordered := make([]corev1.EnvVar, 0, len(remaining))
		for _, want := range base[b].Env {
			at := slices.IndexFunc(remaining, func(env corev1.EnvVar) bool { return env.Name == want.Name })
			if at < 0 {
				continue
			}
			ordered = append(ordered, remaining[at])
			remaining = slices.Delete(remaining, at, at+1)
		}
		merged[i].Env = append(ordered, remaining...)
	}
}

func restore(merged, base *corev1.PodTemplateSpec, protected Protected) {
	for _, key := range protected.Labels {
		value, ok := base.Labels[key]
		if !ok {
			continue
		}
		if merged.Labels == nil {
			merged.Labels = map[string]string{}
		}
		merged.Labels[key] = value
	}

	merged.Spec.Volumes = restoreItems(merged.Spec.Volumes, base.Spec.Volumes, func(v corev1.Volume) bool {
		return slices.Contains(protected.Volumes, v.Name)
	})

	for name, p := range protected.Containers {
		if c := findIn(base.Spec.InitContainers, name); c >= 0 {
			merged.Spec.InitContainers = restoreContainer(merged.Spec.InitContainers, base.Spec.InitContainers, c, p)
		} else if c := findIn(base.Spec.Containers, name); c >= 0 {
			merged.Spec.Containers = restoreContainer(merged.Spec.Containers, base.Spec.Containers, c, p)
		}
	}
}

// restoreContainer restores the protected parts of base[index] in merged, or
// the whole container if merged no longer has it.
func restoreContainer(merged, base []corev1.Container, index int, p ProtectedContainer) []corev1.Container {
	want := base[index]

	m := findIn(merged, want.Name)
	if m < 0 {
		return slices.Insert(merged, min(index, len(merged)), *want.DeepCopy())
	}

	got := &merged[m]
	got.Ports = restoreItems(got.Ports, want.Ports, func(port corev1.ContainerPort) bool {
		return slices.Contains(p.Ports, port.Name)
	})

	var paths []string
	for _, mount := range want.VolumeMounts {
		if slices.Contains(p.VolumeMounts, mount.Name) {
			paths = append(paths, mount.MountPath)
		}
	}
	got.VolumeMounts = restoreItems(got.VolumeMounts, want.VolumeMounts, func(mount corev1.VolumeMount) bool {
		return slices.Contains(p.VolumeMounts, mount.Name) || slices.Contains(paths, mount.MountPath)
	})

	got.Env = restoreItems(got.Env, want.Env, func(env corev1.EnvVar) bool {
		return slices.Contains(p.Env, env.Name)
	})

	return merged
}

// restoreItems replaces the items of merged that match isProtected with the
// matching items of base. They're put where the first protected item was, so
// that environment variables that reference each other keep their order.
func restoreItems[T any](merged, base []T, isProtected func(T) bool) []T {
	want := slices.DeleteFunc(slices.Clone(base), func(item T) bool { return !isProtected(item) })
	got := slices.DeleteFunc(slices.Clone(merged), func(item T) bool { return !isProtected(item) })
	if reflect.DeepEqual(want, got) {
		return merged
	}

	at := slices.IndexFunc(merged, isProtected)
	if at < 0 {
		at = min(max(slices.IndexFunc(base, isProtected), 0), len(merged))
	}
	rest := slices.DeleteFunc(slices.Clone(merged[at:]), isProtected)
	return slices.Concat(merged[:at], want, rest)
}

func findIn(containers []corev1.Container, name string) int {
	return slices.IndexFunc(containers, func(c corev1.Container) bool { return c.Name == name })
}
