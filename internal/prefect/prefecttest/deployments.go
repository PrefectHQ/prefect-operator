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

package prefecttest

import (
	"cmp"
	"fmt"
	"maps"
	"net/http"
	"slices"
)

// POST /flows/ returns the flow with the requested name, creating it if it
// does not exist.
func (s *Server) createFlow(w http.ResponseWriter, r *http.Request) {
	var body object
	if !decode(w, r, &body) {
		return
	}
	name, _ := body["name"].(string)
	if name == "" {
		writeError(w, http.StatusUnprocessableEntity, "name: field required")
		return
	}
	for _, flow := range s.flows {
		if flow["name"] == name {
			writeJSON(w, http.StatusOK, flow)
			return
		}
	}

	flow := object{
		"id":      newID(),
		"created": now(),
		"name":    name,
		"tags":    []any{},
		"labels":  object{},
	}
	merge(flow, body)
	s.flows[flow["id"].(string)] = flow
	writeJSON(w, http.StatusCreated, flow)
}

func (s *Server) readFlowByName(w http.ResponseWriter, r *http.Request) {
	for _, flow := range s.flows {
		if flow["name"] == r.PathValue("name") {
			writeJSON(w, http.StatusOK, flow)
			return
		}
	}
	writeError(w, http.StatusNotFound, "Flow not found")
}

// POST /deployments/ creates a deployment, or updates the one with the same
// flow and name. Either way the request's schedules replace all of the
// deployment's schedules.
func (s *Server) createDeployment(w http.ResponseWriter, r *http.Request) {
	var body object
	if !decode(w, r, &body) {
		return
	}
	name, _ := body["name"].(string)
	flowID, _ := body["flow_id"].(string)
	if name == "" || flowID == "" {
		writeError(w, http.StatusUnprocessableEntity, "name and flow_id: field required")
		return
	}
	if !s.resolveWorkQueue(w, body) {
		return
	}

	status := http.StatusOK
	deployment := s.findDeployment(flowID, name)
	if deployment == nil {
		status = http.StatusCreated
		deployment = newDeployment()
		s.deployments[deployment["id"].(string)] = deployment
	}
	schedules, _ := body["schedules"].([]any)
	delete(body, "schedules")
	setDeploymentFields(deployment, body)

	deployment["schedules"] = []any{}
	for _, in := range schedules {
		in, _ := in.(map[string]any)
		appendSchedule(deployment, newSchedule(deployment["id"].(string), in))
	}
	writeJSON(w, status, deployment)
}

func newDeployment() object {
	return object{
		"id":                       newID(),
		"created":                  now(),
		"version":                  nil,
		"description":              nil,
		"paused":                   false,
		"tags":                     []any{},
		"parameters":               object{},
		"job_variables":            object{},
		"work_queue_name":          nil,
		"work_pool_name":           nil,
		"status":                   "NOT_READY",
		"schedules":                []any{},
		"concurrency_limit":        nil,
		"global_concurrency_limit": nil,
		"entrypoint":               nil,
		"path":                     nil,
		"pull_steps":               nil,
		"parameter_openapi_schema": nil,
		"enforce_parameter_schema": true,
	}
}

func (s *Server) findDeployment(flowID, name string) object {
	for _, deployment := range s.deployments {
		if deployment["flow_id"] == flowID && deployment["name"] == name {
			return deployment
		}
	}
	return nil
}

// resolveWorkQueue applies Prefect's work pool rules to a deployment create
// or update. The work pool must exist. A named queue is created in the pool
// if it does not exist, and with no queue named the deployment uses the
// pool's default queue. It answers 404 and returns false for a missing pool.
func (s *Server) resolveWorkQueue(w http.ResponseWriter, body object) bool {
	poolName, _ := body["work_pool_name"].(string)
	if poolName == "" {
		return true
	}
	pool, ok := s.workPools[poolName]
	if !ok {
		writeError(w, http.StatusNotFound, fmt.Sprintf("Work pool %q not found.", poolName))
		return false
	}
	queueName, _ := body["work_queue_name"].(string)
	if queueName == "" {
		body["work_queue_name"] = s.workQueues[pool["default_queue_id"].(string)]["name"]
		return true
	}
	if s.findWorkQueue(pool, queueName) == nil {
		s.addWorkQueue(pool, object{"name": queueName})
	}
	return true
}

// setDeploymentFields sets the fields of a create or update request on a
// deployment. Prefect stores a concurrency limit as a global concurrency
// limit, and always answers null for the deprecated concurrency_limit field.
func setDeploymentFields(deployment, body object) {
	if limit, ok := body["concurrency_limit"]; ok {
		delete(body, "concurrency_limit")
		if limit == nil {
			deployment["global_concurrency_limit"] = nil
		} else {
			deployment["global_concurrency_limit"] = object{
				"id":     newID(),
				"name":   "deployment:" + deployment["id"].(string),
				"limit":  limit,
				"active": true,
			}
		}
	}
	merge(deployment, body)
}

// lookupDeployment returns the deployment named by the {id} path parameter,
// or answers 404.
func (s *Server) lookupDeployment(w http.ResponseWriter, r *http.Request) (object, bool) {
	id := r.PathValue("id")
	deployment, ok := s.deployments[id]
	if !isID(id) || !ok {
		writeError(w, http.StatusNotFound, "Deployment not found.")
		return nil, false
	}
	return deployment, true
}

func (s *Server) readDeployment(w http.ResponseWriter, r *http.Request) {
	if deployment, ok := s.lookupDeployment(w, r); ok {
		writeJSON(w, http.StatusOK, deployment)
	}
}

func (s *Server) updateDeployment(w http.ResponseWriter, r *http.Request) {
	deployment, ok := s.lookupDeployment(w, r)
	if !ok {
		return
	}
	var body object
	if !decode(w, r, &body) {
		return
	}
	if !rejectFields(w, body, "name", "flow_id") || !s.resolveWorkQueue(w, body) {
		return
	}
	setDeploymentFields(deployment, body)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) deleteDeployment(w http.ResponseWriter, r *http.Request) {
	if deployment, ok := s.lookupDeployment(w, r); ok {
		delete(s.deployments, deployment["id"].(string))
		w.WriteHeader(http.StatusNoContent)
	}
}

// POST /deployments/filter supports the filter the operator sends: names in
// deployments.name.any_, and a limit. Results are sorted by name.
func (s *Server) filterDeployments(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Deployments struct {
			Name struct {
				Any []string `json:"any_"`
			} `json:"name"`
		} `json:"deployments"`
		Limit int `json:"limit"`
	}
	if !decode(w, r, &body) {
		return
	}
	names := body.Deployments.Name.Any

	matches := []object{}
	for _, deployment := range s.deployments {
		name, _ := deployment["name"].(string)
		if names == nil || slices.Contains(names, name) {
			matches = append(matches, deployment)
		}
	}
	slices.SortFunc(matches, func(a, b object) int {
		return cmp.Compare(a["name"].(string), b["name"].(string))
	})
	if body.Limit > 0 && len(matches) > body.Limit {
		matches = matches[:body.Limit]
	}
	writeJSON(w, http.StatusOK, matches)
}

// newSchedule returns a deployment schedule with Prefect's defaults for the
// fields that in leaves out.
func newSchedule(deploymentID string, in object) object {
	schedule := object{
		"id":                 newID(),
		"created":            now(),
		"updated":            now(),
		"deployment_id":      deploymentID,
		"active":             true,
		"max_scheduled_runs": nil,
		"parameters":         object{},
		"slug":               nil,
	}
	for _, f := range []string{"active", "max_scheduled_runs", "parameters", "slug"} {
		if v, ok := in[f]; ok {
			schedule[f] = v
		}
	}
	spec, _ := in["schedule"].(map[string]any)
	schedule["schedule"] = normalizeSchedule(spec)
	return schedule
}

// normalizeSchedule returns the schedule as Prefect stores it. Prefect keeps
// only the fields of the schedule's type and fills in defaults: an interval
// schedule anchors at the current time in UTC, a cron schedule matches on
// day of month or day of week, and an rrule schedule is in UTC.
func normalizeSchedule(in object) object {
	pick := func(defaults object) object {
		out := object{}
		for f, def := range defaults {
			if v, ok := in[f]; ok && v != nil {
				out[f] = v
			} else {
				out[f] = def
			}
		}
		return out
	}
	switch {
	case in["interval"] != nil:
		return pick(object{"interval": nil, "anchor_date": now(), "timezone": "UTC"})
	case in["cron"] != nil:
		// Without an anchor date, a cron schedule's timezone stays null.
		return pick(object{"cron": nil, "timezone": nil, "day_or": true})
	case in["rrule"] != nil:
		return pick(object{"rrule": nil, "timezone": "UTC"})
	}
	return maps.Clone(in)
}

func appendSchedule(deployment, schedule object) {
	deployment["schedules"] = append(deployment["schedules"].([]any), schedule)
}

func (s *Server) createSchedules(w http.ResponseWriter, r *http.Request) {
	deployment, ok := s.lookupDeployment(w, r)
	if !ok {
		return
	}
	var body []object
	if !decode(w, r, &body) {
		return
	}

	// The slug is unique within a deployment, and one duplicate fails the
	// whole request.
	slugs := map[any]bool{}
	for _, existing := range deployment["schedules"].([]any) {
		slugs[existing.(object)["slug"]] = true
	}
	for _, in := range body {
		slug := in["slug"]
		if slug != nil && slugs[slug] {
			writeError(w, http.StatusConflict, "Schedule slugs must be unique within a deployment.")
			return
		}
		slugs[slug] = true
	}

	created := []object{}
	for _, in := range body {
		schedule := newSchedule(deployment["id"].(string), in)
		appendSchedule(deployment, schedule)
		created = append(created, schedule)
	}
	writeJSON(w, http.StatusCreated, created)
}

// lookupSchedule returns the deployment and the index of the schedule named
// by the path, or answers 404.
func (s *Server) lookupSchedule(w http.ResponseWriter, r *http.Request) (object, int, bool) {
	deployment, ok := s.lookupDeployment(w, r)
	if !ok {
		return nil, 0, false
	}
	i := slices.IndexFunc(deployment["schedules"].([]any), func(schedule any) bool {
		return schedule.(object)["id"] == r.PathValue("schedule_id")
	})
	if i < 0 {
		writeError(w, http.StatusNotFound, "Schedule not found.")
		return nil, 0, false
	}
	return deployment, i, true
}

func (s *Server) updateSchedule(w http.ResponseWriter, r *http.Request) {
	deployment, i, ok := s.lookupSchedule(w, r)
	if !ok {
		return
	}
	var body object
	if !decode(w, r, &body) {
		return
	}
	if spec, ok := body["schedule"].(map[string]any); ok {
		body["schedule"] = normalizeSchedule(spec)
	}
	merge(deployment["schedules"].([]any)[i].(object), body)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) deleteSchedule(w http.ResponseWriter, r *http.Request) {
	deployment, i, ok := s.lookupSchedule(w, r)
	if !ok {
		return
	}
	deployment["schedules"] = slices.Delete(deployment["schedules"].([]any), i, i+1)
	w.WriteHeader(http.StatusNoContent)
}
