package prefect

import "reflect"

// DeploymentUpToDate reports whether the remote deployment already matches
// every operator-managed field of the desired spec, so the update call (which
// deletes the deployment's future auto-scheduled runs) can be skipped.
// Fields the spec omits are not sent in the update and are ignored here.
func DeploymentUpToDate(remote *Deployment, desired *DeploymentSpec) bool {
	if remote == nil || desired == nil {
		return false
	}
	if remote.Name != desired.Name || remote.FlowID != desired.FlowID {
		return false
	}
	if !ptrMatches(desired.Description, remote.Description) {
		return false
	}
	if !ptrMatches(desired.Version, remote.Version) {
		return false
	}
	if !ptrMatches(desired.WorkPoolName, remote.WorkPoolName) {
		return false
	}
	if !ptrMatches(desired.WorkQueueName, remote.WorkQueueName) {
		return false
	}
	if !ptrMatches(desired.Entrypoint, remote.Entrypoint) {
		return false
	}
	if !ptrMatches(desired.Path, remote.Path) {
		return false
	}
	if desired.Paused != nil && *desired.Paused != remote.Paused {
		return false
	}
	if desired.EnforceParameterSchema != nil && *desired.EnforceParameterSchema != remote.EnforceParameterSchema {
		return false
	}
	if !concurrencyLimitMatches(remote, desired.ConcurrencyLimit) {
		return false
	}
	if len(desired.Tags) > 0 && !sameStringSet(desired.Tags, remote.Tags) {
		return false
	}
	if desired.Parameters != nil && !reflect.DeepEqual(desired.Parameters, remote.Parameters) {
		return false
	}
	if desired.JobVariables != nil && !reflect.DeepEqual(desired.JobVariables, remote.JobVariables) {
		return false
	}
	if desired.ParameterOpenAPISchema != nil && !reflect.DeepEqual(desired.ParameterOpenAPISchema, remote.ParameterOpenAPISchema) {
		return false
	}
	if desired.PullSteps != nil && !reflect.DeepEqual(desired.PullSteps, remote.PullSteps) {
		return false
	}
	return true
}

// ptrMatches treats a nil desired value as "not managed" and skips it.
func ptrMatches[T comparable](desired, remote *T) bool {
	if desired == nil {
		return true
	}
	return remote != nil && *remote == *desired
}

// concurrencyLimitMatches reads the remote limit from
// global_concurrency_limit.limit; the top-level field is deprecated and always null.
func concurrencyLimitMatches(remote *Deployment, desired *int) bool {
	if desired == nil {
		return true
	}
	if remote.GlobalConcurrencyLimit != nil {
		return remote.GlobalConcurrencyLimit.Limit == *desired
	}
	return remote.ConcurrencyLimit != nil && *remote.ConcurrencyLimit == *desired
}

func sameStringSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	counts := make(map[string]int, len(a))
	for _, s := range a {
		counts[s]++
	}
	for _, s := range b {
		counts[s]--
		if counts[s] < 0 {
			return false
		}
	}
	return true
}
