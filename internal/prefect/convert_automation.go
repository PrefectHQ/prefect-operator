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
	"encoding/json"
	"fmt"
	"strconv"

	"k8s.io/apimachinery/pkg/runtime"

	prefectiov1 "github.com/PrefectHQ/prefect-operator/api/v1"
)

const (
	// keyType is the discriminator key used by Prefect trigger/action payloads.
	keyType = "type"
	// keyName is the "name" key used in payloads/filters.
	keyName = "name"
)

// DeploymentNamesReferenced returns the distinct deployment names referenced by
// any action's deploymentName across all three action lists. The controller
// resolves these to IDs (via FindDeploymentByName) and passes the map to
// ConvertToAutomationSpec.
func DeploymentNamesReferenced(a *prefectiov1.PrefectAutomation) []string {
	seen := map[string]struct{}{}
	var names []string
	collect := func(actions []prefectiov1.PrefectAutomationAction) {
		for i := range actions {
			if n := actions[i].DeploymentName; n != nil && *n != "" {
				if _, ok := seen[*n]; !ok {
					seen[*n] = struct{}{}
					names = append(names, *n)
				}
			}
		}
	}
	collect(a.Spec.Actions)
	collect(a.Spec.ActionsOnTrigger)
	collect(a.Spec.ActionsOnResolve)
	return names
}

// ConvertToAutomationSpec converts a K8s PrefectAutomation into the Prefect API
// AutomationSpec payload. deploymentIDs maps a deployment name -> ID for actions
// that reference a deployment by name (resolved by the controller beforehand).
func ConvertToAutomationSpec(a *prefectiov1.PrefectAutomation, deploymentIDs map[string]string) (*AutomationSpec, error) {
	trigger, err := buildTrigger(a.Spec.Trigger)
	if err != nil {
		return nil, err
	}

	actions, err := buildActions(a.Spec.Actions, deploymentIDs)
	if err != nil {
		return nil, err
	}
	actionsOnTrigger, err := buildActions(a.Spec.ActionsOnTrigger, deploymentIDs)
	if err != nil {
		return nil, err
	}
	actionsOnResolve, err := buildActions(a.Spec.ActionsOnResolve, deploymentIDs)
	if err != nil {
		return nil, err
	}

	spec := &AutomationSpec{
		Name:             a.Spec.Name,
		Enabled:          a.Spec.Enabled,
		Trigger:          trigger,
		Actions:          actions,
		ActionsOnTrigger: actionsOnTrigger,
		ActionsOnResolve: actionsOnResolve,
	}
	if a.Spec.Description != nil {
		spec.Description = *a.Spec.Description
	}
	// Prefect requires a non-null actions list.
	if spec.Actions == nil {
		spec.Actions = []map[string]any{}
	}
	return spec, nil
}

// buildTrigger renders exactly one of the trigger union members into the API map.
func buildTrigger(t prefectiov1.PrefectAutomationTrigger) (map[string]any, error) {
	switch {
	case t.Event != nil:
		return buildEventTrigger(t.Event)
	case t.Metric != nil:
		return buildMetricTrigger(t.Metric)
	case t.Compound != nil:
		return buildCompoundTrigger(t.Compound)
	case t.Sequence != nil:
		return buildSequenceTrigger(t.Sequence)
	default:
		return nil, fmt.Errorf("no trigger type set (need one of event/metric/compound/sequence)")
	}
}

func buildEventTrigger(e *prefectiov1.PrefectEventTrigger) (map[string]any, error) {
	match, err := rawToMap(e.Match)
	if err != nil {
		return nil, fmt.Errorf("trigger.event.match: %w", err)
	}
	matchRelated, err := rawToMap(e.MatchRelated)
	if err != nil {
		return nil, fmt.Errorf("trigger.event.matchRelated: %w", err)
	}
	m := map[string]any{
		keyType:         "event",
		"posture":       e.Posture,
		"match":         match,
		"match_related": matchRelated,
		"after":         nonNilStrings(e.After),
		"expect":        nonNilStrings(e.Expect),
		"for_each":      nonNilStrings(e.ForEach),
	}
	if e.Threshold != nil {
		m["threshold"] = *e.Threshold
	}
	if e.Within != nil {
		m["within"] = *e.Within
	}
	return m, nil
}

func buildMetricTrigger(mt *prefectiov1.PrefectMetricTrigger) (map[string]any, error) {
	match, err := rawToMap(mt.Match)
	if err != nil {
		return nil, fmt.Errorf("trigger.metric.match: %w", err)
	}
	matchRelated, err := rawToMap(mt.MatchRelated)
	if err != nil {
		return nil, fmt.Errorf("trigger.metric.matchRelated: %w", err)
	}
	threshold := mt.Metric.Threshold.AsApproximateFloat64()
	return map[string]any{
		keyType:         "metric",
		"match":         match,
		"match_related": matchRelated,
		"metric": map[string]any{
			keyName:      mt.Metric.Name,
			"operator":   mt.Metric.Operator,
			"threshold":  threshold,
			"range":      mt.Metric.Range,
			"firing_for": mt.Metric.FiringFor,
		},
	}, nil
}

func buildCompoundTrigger(c *prefectiov1.PrefectCompoundTrigger) (map[string]any, error) {
	children, err := buildChildTriggers(c.Triggers)
	if err != nil {
		return nil, err
	}
	m := map[string]any{
		keyType:    "compound",
		"require":  parseRequire(c.Require),
		"triggers": children,
	}
	if c.Within != nil {
		m["within"] = *c.Within
	}
	return m, nil
}

func buildSequenceTrigger(s *prefectiov1.PrefectSequenceTrigger) (map[string]any, error) {
	children, err := buildChildTriggers(s.Triggers)
	if err != nil {
		return nil, err
	}
	m := map[string]any{
		keyType:    "sequence",
		"triggers": children,
	}
	if s.Within != nil {
		m["within"] = *s.Within
	}
	return m, nil
}

func buildChildTriggers(triggers []prefectiov1.PrefectChildTrigger) ([]map[string]any, error) {
	out := make([]map[string]any, 0, len(triggers))
	for i := range triggers {
		var (
			child map[string]any
			err   error
		)
		switch {
		case triggers[i].Event != nil:
			child, err = buildEventTrigger(triggers[i].Event)
		case triggers[i].Metric != nil:
			child, err = buildMetricTrigger(triggers[i].Metric)
		default:
			err = fmt.Errorf("child must set event or metric")
		}
		if err != nil {
			return nil, fmt.Errorf("child trigger %d: %w", i, err)
		}
		out = append(out, child)
	}
	return out, nil
}

// parseRequire renders compound `require` as "any"/"all" (string) or a number.
func parseRequire(require string) any {
	if require == "any" || require == "all" {
		return require
	}
	if n, err := strconv.Atoi(require); err == nil {
		return n
	}
	return require
}

func buildActions(actions []prefectiov1.PrefectAutomationAction, deploymentIDs map[string]string) ([]map[string]any, error) {
	if len(actions) == 0 {
		return nil, nil
	}
	out := make([]map[string]any, 0, len(actions))
	for i := range actions {
		m, err := buildAction(actions[i], deploymentIDs)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, nil
}

func buildAction(a prefectiov1.PrefectAutomationAction, deploymentIDs map[string]string) (map[string]any, error) {
	m := map[string]any{keyType: a.Type}
	putStr := func(key string, v *string) {
		if v != nil {
			m[key] = *v
		}
	}
	// Resolve the deployment reference: explicit ID wins; otherwise resolve the
	// name via the map the controller populated.
	switch {
	case a.DeploymentID != nil:
		m["deployment_id"] = *a.DeploymentID
	case a.DeploymentName != nil && *a.DeploymentName != "":
		id, ok := deploymentIDs[*a.DeploymentName]
		if !ok || id == "" {
			return nil, fmt.Errorf("deployment name %q not resolved to an ID", *a.DeploymentName)
		}
		m["deployment_id"] = id
	}
	putStr("source", a.Source)
	putStr("automation_id", a.AutomationID)
	putStr("work_pool_id", a.WorkPoolID)
	putStr("work_queue_id", a.WorkQueueID)
	putStr("schedule_id", a.ScheduleID)
	putStr("block_document_id", a.BlockDocumentID)
	putStr("state", a.State)
	putStr(keyName, a.Name)
	putStr("message", a.Message)
	putStr("subject", a.Subject)
	putStr("body", a.Body)
	putStr("payload", a.Payload)
	if a.Parameters != nil {
		params, err := rawToMap(a.Parameters)
		if err != nil {
			return nil, fmt.Errorf("action parameters: %w", err)
		}
		m["parameters"] = params
	}
	if a.JobVariables != nil {
		jv, err := rawToMap(a.JobVariables)
		if err != nil {
			return nil, fmt.Errorf("action job_variables: %w", err)
		}
		m["job_variables"] = jv
	}
	return m, nil
}

// rawToMap parses a RawExtension JSON object into a map, defaulting to {}.
func rawToMap(raw *runtime.RawExtension) (map[string]any, error) {
	if raw == nil || len(raw.Raw) == 0 {
		return map[string]any{}, nil
	}
	var m map[string]any
	if err := json.Unmarshal(raw.Raw, &m); err != nil {
		return nil, fmt.Errorf("invalid JSON object: %w", err)
	}
	return m, nil
}

func nonNilStrings(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// UpdateAutomationStatus updates the K8s PrefectAutomation status from an API Automation.
//
// LastSyncTime is stamped by the controller (not here) after a successful sync,
// so needsSync gates the next Prefect re-check by the resync interval
// (spec.interval or --default-resync-interval) rather than hitting the API on
// every reconcile.
func UpdateAutomationStatus(k8sAutomation *prefectiov1.PrefectAutomation, automation *Automation) {
	id := automation.ID
	k8sAutomation.Status.Id = &id
	k8sAutomation.Status.Ready = true
}
