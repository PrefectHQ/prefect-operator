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

// Package prefecttest runs a fake Prefect API for tests.
//
// The fake serves the routes the operator calls, with the status codes and
// JSON bodies of a Prefect server, and keeps its state in memory. It follows
// the server rules that the operator depends on:
//
//   - A missing object is a 404 with a {"detail": ...} body.
//   - A deployment needs its work pool, and POST /deployments/ updates the
//     deployment with the same flow and name instead of creating a second one.
//   - A new work pool starts NOT_READY with a queue named "default".
//   - PATCH applies only the fields in the request.
//   - Updates answer 204 with no body.
//
// The fake stores objects as JSON maps keyed by Prefect's field names. It does
// not share the client's Go types, so a wrong JSON tag in the client fails a
// test instead of making a round trip that hides it.
//
// The fake serves the API under /api, as a Prefect server does. Tests choose
// behavior per route, named by its method and path pattern below /api as in
// "PATCH /work_pools/{name}": Fail makes a route answer with an error status,
// and Calls counts the requests it received.
package prefecttest

import (
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// object is a Prefect API object as JSON: field names to decoded values.
type object = map[string]any

// Server is a fake Prefect API. Point a client at its APIURL.
type Server struct {
	*httptest.Server

	// APIURL is the base URL of the API: the server's URL followed by /api.
	APIURL string

	mu       sync.Mutex
	apiKey   string
	routes   []string
	calls    map[string]int
	failures map[string]int

	flows       map[string]object // by ID
	deployments map[string]object // by ID
	workPools   map[string]object // by name
	workQueues  map[string]object // by ID
	automations map[string]object // by ID
}

// NewServer starts a fake Prefect API. Call Close when the test ends.
func NewServer() *Server {
	s := &Server{
		calls:       map[string]int{},
		failures:    map[string]int{},
		flows:       map[string]object{},
		deployments: map[string]object{},
		workPools:   map[string]object{},
		workQueues:  map[string]object{},
		automations: map[string]object{},
	}

	mux := http.NewServeMux()
	s.handle(mux, "POST /flows/", s.createFlow)
	s.handle(mux, "GET /flows/name/{name}", s.readFlowByName)

	s.handle(mux, "POST /deployments/", s.createDeployment)
	s.handle(mux, "POST /deployments/filter", s.filterDeployments)
	s.handle(mux, "GET /deployments/{id}", s.readDeployment)
	s.handle(mux, "PATCH /deployments/{id}", s.updateDeployment)
	s.handle(mux, "DELETE /deployments/{id}", s.deleteDeployment)
	s.handle(mux, "POST /deployments/{id}/schedules", s.createSchedules)
	s.handle(mux, "PATCH /deployments/{id}/schedules/{schedule_id}", s.updateSchedule)
	s.handle(mux, "DELETE /deployments/{id}/schedules/{schedule_id}", s.deleteSchedule)

	s.handle(mux, "POST /work_pools/", s.createWorkPool)
	s.handle(mux, "GET /work_pools/{name}", s.readWorkPool)
	s.handle(mux, "PATCH /work_pools/{name}", s.updateWorkPool)
	s.handle(mux, "DELETE /work_pools/{name}", s.deleteWorkPool)
	s.handle(mux, "GET /collections/views/aggregate-worker-metadata", s.readWorkerMetadata)

	s.handle(mux, "POST /work_pools/{work_pool_name}/queues", s.createWorkQueue)
	s.handle(mux, "GET /work_pools/{work_pool_name}/queues/{name}", s.readWorkQueue)
	s.handle(mux, "PATCH /work_pools/{work_pool_name}/queues/{name}", s.updateWorkQueue)
	s.handle(mux, "DELETE /work_pools/{work_pool_name}/queues/{name}", s.deleteWorkQueue)

	s.handle(mux, "POST /automations/", s.createAutomation)
	s.handle(mux, "GET /automations/{id}", s.readAutomation)
	s.handle(mux, "PUT /automations/{id}", s.replaceAutomation)
	s.handle(mux, "DELETE /automations/{id}", s.deleteAutomation)

	// A request the fake has no route for is a bug in the client or a gap in
	// the fake. Answer 501 so it fails loudly instead of reading as a 404.
	noRoute := func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotImplemented,
			fmt.Sprintf("prefecttest: no route for %s %s", r.Method, r.URL.EscapedPath()))
	}
	mux.HandleFunc("/", noRoute)

	root := http.NewServeMux()
	root.Handle("/api/", http.StripPrefix("/api", mux))
	root.HandleFunc("/", noRoute)

	s.Server = httptest.NewServer(root)
	s.APIURL = s.URL + "/api"
	return s
}

// handle registers h for route. A route that ends in "/" matches only that
// exact path, as Prefect's routes do, and not the paths below it.
func (s *Server) handle(mux *http.ServeMux, route string, h http.HandlerFunc) {
	s.routes = append(s.routes, route)
	pattern := route
	if strings.HasSuffix(pattern, "/") {
		pattern += "{$}"
	}
	mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.calls[route]++
		if s.apiKey != "" && r.Header.Get("Authorization") != "Bearer "+s.apiKey {
			writeError(w, http.StatusUnauthorized, "Invalid authentication credentials")
			return
		}
		if status, ok := s.failures[route]; ok {
			writeError(w, status, fmt.Sprintf("prefecttest: injected failure for %s", route))
			return
		}
		h(w, r)
	})
}

// RequireAPIKey makes every route answer 401 to a request that does not
// send key as its bearer token.
func (s *Server) RequireAPIKey(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.apiKey = key
}

// Fail makes route answer every request with status until ClearFailures.
func (s *Server) Fail(route string, status int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mustBeRoute(route)
	s.failures[route] = status
}

// ClearFailures undoes every Fail.
func (s *Server) ClearFailures() {
	s.mu.Lock()
	defer s.mu.Unlock()
	clear(s.failures)
}

// Calls returns how many requests route has received.
func (s *Server) Calls(route string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mustBeRoute(route)
	return s.calls[route]
}

// mustBeRoute panics when route is not one the fake serves, so a typo in a
// test fails instead of counting zero calls.
func (s *Server) mustBeRoute(route string) {
	if !slices.Contains(s.routes, route) {
		panic(fmt.Sprintf("prefecttest: no route %q; routes are %q", route, s.routes))
	}
}

func newID() string {
	return uuid.NewString()
}

func now() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}

// isID reports whether s is a UUID. Prefect declares ID path parameters as
// UUIDs, so a request with any other ID matches no route and answers 404.
func isID(s string) bool {
	return uuid.Validate(s) == nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError answers with status and the {"detail": ...} body of a FastAPI
// HTTPException.
func writeError(w http.ResponseWriter, status int, detail string) {
	writeJSON(w, status, object{"detail": detail})
}

// decode reads the JSON request body into v. It answers 422, as Prefect does
// for a body that fails validation, and returns false when the body is not
// JSON of v's type.
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeError(w, http.StatusUnprocessableEntity, fmt.Sprintf("invalid request body: %v", err))
		return false
	}
	return true
}

// rejectFields answers 422 and returns false when body sets any of fields.
// Prefect's update schemas forbid fields that an update cannot change.
func rejectFields(w http.ResponseWriter, body object, fields ...string) bool {
	for _, f := range fields {
		if _, ok := body[f]; ok {
			writeError(w, http.StatusUnprocessableEntity, fmt.Sprintf("%s: extra inputs are not permitted", f))
			return false
		}
	}
	return true
}

// merge sets each field of body on obj, as a Prefect PATCH does: fields
// missing from body keep their values, and a null clears a field.
func merge(obj, body object) {
	maps.Copy(obj, body)
	obj["updated"] = now()
}
