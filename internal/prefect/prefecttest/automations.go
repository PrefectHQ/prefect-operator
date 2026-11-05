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
	"net/http"
)

// newAutomation returns an automation with Prefect's defaults for the fields
// that body leaves out. Prefect gives the trigger a new ID each time it
// stores the automation.
func newAutomation(id, created string, body object) object {
	automation := object{
		"id":                 id,
		"created":            created,
		"description":        "",
		"enabled":            true,
		"trigger":            nil,
		"actions":            []any{},
		"actions_on_trigger": []any{},
		"actions_on_resolve": []any{},
	}
	merge(automation, body)
	if trigger, ok := automation["trigger"].(map[string]any); ok {
		trigger["id"] = newID()
	}
	return automation
}

func (s *Server) createAutomation(w http.ResponseWriter, r *http.Request) {
	var body object
	if !decode(w, r, &body) {
		return
	}
	if name, _ := body["name"].(string); name == "" {
		writeError(w, http.StatusUnprocessableEntity, "name: field required")
		return
	}
	automation := newAutomation(newID(), now(), body)
	s.automations[automation["id"].(string)] = automation
	writeJSON(w, http.StatusCreated, automation)
}

// lookupAutomation returns the automation named by the {id} path parameter,
// or answers 404.
func (s *Server) lookupAutomation(w http.ResponseWriter, r *http.Request) (object, bool) {
	id := r.PathValue("id")
	automation, ok := s.automations[id]
	if !isID(id) || !ok {
		writeError(w, http.StatusNotFound, "Automation not found")
		return nil, false
	}
	return automation, true
}

func (s *Server) readAutomation(w http.ResponseWriter, r *http.Request) {
	if automation, ok := s.lookupAutomation(w, r); ok {
		writeJSON(w, http.StatusOK, automation)
	}
}

// PUT /automations/{id} replaces the automation: fields missing from the
// request take their defaults.
func (s *Server) replaceAutomation(w http.ResponseWriter, r *http.Request) {
	existing, ok := s.lookupAutomation(w, r)
	if !ok {
		return
	}
	var body object
	if !decode(w, r, &body) {
		return
	}
	id := existing["id"].(string)
	s.automations[id] = newAutomation(id, existing["created"].(string), body)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) deleteAutomation(w http.ResponseWriter, r *http.Request) {
	if automation, ok := s.lookupAutomation(w, r); ok {
		delete(s.automations, automation["id"].(string))
		w.WriteHeader(http.StatusNoContent)
	}
}
