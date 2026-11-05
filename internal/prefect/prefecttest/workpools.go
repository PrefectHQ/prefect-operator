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
	"fmt"
	"net/http"
	"slices"
	"strings"
)

// DefaultBaseJobTemplate returns the base job template that the fake's worker
// metadata gives the "kubernetes" worker type.
func DefaultBaseJobTemplate() map[string]any {
	return map[string]any{
		"foo":  "bar",
		"quux": true,
		"boz":  []any{"baz", "bot", "biz"},
	}
}

// Prefect reserves work pool names that start with "prefect".
func reservedPoolName(w http.ResponseWriter, name string) bool {
	if strings.HasPrefix(strings.ToLower(name), "prefect") {
		writeError(w, http.StatusForbidden, "Work pools starting with 'Prefect' are reserved for internal use.")
		return true
	}
	return false
}

// poolStatus returns the status Prefect gives a work pool with no workers.
func poolStatus(pool object) any {
	if pool["type"] == "prefect-agent" {
		return nil
	}
	if pool["is_paused"] == true {
		return "PAUSED"
	}
	return "NOT_READY"
}

// POST /work_pools/ creates a work pool and its default queue.
func (s *Server) createWorkPool(w http.ResponseWriter, r *http.Request) {
	var body object
	if !decode(w, r, &body) {
		return
	}
	name, _ := body["name"].(string)
	if name == "" {
		writeError(w, http.StatusUnprocessableEntity, "name: field required")
		return
	}
	if reservedPoolName(w, name) {
		return
	}
	if _, ok := s.workPools[name]; ok {
		writeError(w, http.StatusConflict, "A work pool with this name already exists.")
		return
	}

	pool := object{
		"id":                newID(),
		"created":           now(),
		"type":              "prefect-agent",
		"description":       nil,
		"is_paused":         false,
		"concurrency_limit": nil,
		"base_job_template": object{},
	}
	merge(pool, body)
	pool["status"] = poolStatus(pool)
	s.workPools[name] = pool

	queue := s.addWorkQueue(pool, object{"name": "default", "description": "The work pool's default queue."})
	pool["default_queue_id"] = queue["id"]
	writeJSON(w, http.StatusCreated, pool)
}

// lookupWorkPool returns the work pool named by the path, or answers 404.
func (s *Server) lookupWorkPool(w http.ResponseWriter, name string) (object, bool) {
	pool, ok := s.workPools[name]
	if !ok {
		writeError(w, http.StatusNotFound, fmt.Sprintf("Work pool %q not found.", name))
	}
	return pool, ok
}

func (s *Server) readWorkPool(w http.ResponseWriter, r *http.Request) {
	if pool, ok := s.lookupWorkPool(w, r.PathValue("name")); ok {
		writeJSON(w, http.StatusOK, pool)
	}
}

func (s *Server) updateWorkPool(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if reservedPoolName(w, name) {
		return
	}
	pool, ok := s.lookupWorkPool(w, name)
	if !ok {
		return
	}
	var body object
	if !decode(w, r, &body) {
		return
	}
	if !rejectFields(w, body, "name", "type") {
		return
	}
	merge(pool, body)
	if _, ok := body["is_paused"]; ok {
		pool["status"] = poolStatus(pool)
	}
	w.WriteHeader(http.StatusNoContent)
}

// DELETE /work_pools/{name} deletes the pool and its queues.
func (s *Server) deleteWorkPool(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if reservedPoolName(w, name) {
		return
	}
	pool, ok := s.lookupWorkPool(w, name)
	if !ok {
		return
	}
	for id, queue := range s.workQueues {
		if queue["work_pool_id"] == pool["id"] {
			delete(s.workQueues, id)
		}
	}
	delete(s.workPools, name)
	w.WriteHeader(http.StatusNoContent)
}

// GET /collections/views/aggregate-worker-metadata lists worker types by
// collection. The fake knows one: "kubernetes", with DefaultBaseJobTemplate.
func (s *Server) readWorkerMetadata(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, object{
		"prefect-kubernetes": object{
			"kubernetes": object{
				"type":                           "kubernetes",
				"display_name":                   "Kubernetes",
				"default_base_job_configuration": DefaultBaseJobTemplate(),
			},
		},
	})
}

func (s *Server) findWorkQueue(pool object, name string) object {
	for _, queue := range s.workQueues {
		if queue["work_pool_id"] == pool["id"] && queue["name"] == name {
			return queue
		}
	}
	return nil
}

// addWorkQueue creates a queue in pool. A queue created without a priority
// takes the lowest priority that no other queue in the pool holds.
func (s *Server) addWorkQueue(pool, body object) object {
	queue := object{
		"id":                newID(),
		"created":           now(),
		"description":       "",
		"is_paused":         false,
		"concurrency_limit": nil,
		"priority":          nil,
		"work_pool_id":      pool["id"],
		"work_pool_name":    pool["name"],
	}
	merge(queue, body)
	if queue["priority"] == nil {
		queue["priority"] = s.freePriority(pool)
	}
	s.workQueues[queue["id"].(string)] = queue
	return queue
}

func (s *Server) freePriority(pool object) float64 {
	var taken []float64
	for _, queue := range s.workQueues {
		if queue["work_pool_id"] == pool["id"] {
			if p, ok := queue["priority"].(float64); ok {
				taken = append(taken, p)
			}
		}
	}
	for p := 1.0; ; p++ {
		if !slices.Contains(taken, p) {
			return p
		}
	}
}

func (s *Server) createWorkQueue(w http.ResponseWriter, r *http.Request) {
	pool, ok := s.lookupWorkPool(w, r.PathValue("work_pool_name"))
	if !ok {
		return
	}
	var body object
	if !decode(w, r, &body) {
		return
	}
	name, _ := body["name"].(string)
	if name == "" {
		writeError(w, http.StatusUnprocessableEntity, "name: field required")
		return
	}
	if s.findWorkQueue(pool, name) != nil {
		writeError(w, http.StatusConflict, "A work queue with this name already exists.")
		return
	}
	writeJSON(w, http.StatusCreated, s.addWorkQueue(pool, body))
}

// lookupWorkQueue returns the pool and queue named by the path, or answers
// 404 when either is missing.
func (s *Server) lookupWorkQueue(w http.ResponseWriter, r *http.Request) (object, object, bool) {
	pool, ok := s.lookupWorkPool(w, r.PathValue("work_pool_name"))
	if !ok {
		return nil, nil, false
	}
	queue := s.findWorkQueue(pool, r.PathValue("name"))
	if queue == nil {
		writeError(w, http.StatusNotFound, "Work queue not found.")
		return nil, nil, false
	}
	return pool, queue, true
}

func (s *Server) readWorkQueue(w http.ResponseWriter, r *http.Request) {
	if _, queue, ok := s.lookupWorkQueue(w, r); ok {
		writeJSON(w, http.StatusOK, queue)
	}
}

func (s *Server) updateWorkQueue(w http.ResponseWriter, r *http.Request) {
	_, queue, ok := s.lookupWorkQueue(w, r)
	if !ok {
		return
	}
	var body object
	if !decode(w, r, &body) {
		return
	}
	merge(queue, body)
	w.WriteHeader(http.StatusNoContent)
}

// DELETE /work_pools/{work_pool_name}/queues/{name} refuses to delete the
// pool's default queue. Prefect raises a ValueError for that, which reaches
// the client as a 500.
func (s *Server) deleteWorkQueue(w http.ResponseWriter, r *http.Request) {
	pool, queue, ok := s.lookupWorkQueue(w, r)
	if !ok {
		return
	}
	if queue["id"] == pool["default_queue_id"] {
		writeJSON(w, http.StatusInternalServerError, object{"exception_message": "Internal Server Error"})
		return
	}
	delete(s.workQueues, queue["id"].(string))
	w.WriteHeader(http.StatusNoContent)
}
