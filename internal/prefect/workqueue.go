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
	"context"
	"errors"
	"fmt"
	"net/http"
)

// ErrWorkQueueNotFound is returned when a work queue no longer exists in
// Prefect (e.g. it was deleted out-of-band). Callers can use errors.Is to
// distinguish this from other API errors and recreate rather than loop.
var ErrWorkQueueNotFound = errors.New("work queue not found")

// ErrWorkPoolNotFound is returned when the pool a work queue belongs to does
// not exist in Prefect. Callers can use errors.Is to requeue until the pool is
// created rather than loop on a generic error.
var ErrWorkPoolNotFound = errors.New("work pool not found")

// Clearable work queue fields, by their CRD JSON names. Used as the vocabulary
// of UpdateWorkQueue's clearFields.
const (
	WorkQueueFieldConcurrencyLimit = "concurrencyLimit"
	WorkQueueFieldPriority         = "priority"
	WorkQueueFieldDescription      = "description"
	WorkQueueFieldIsPaused         = "isPaused"
)

// All methods use the pool-scoped routes (/work_pools/{pool}/queues[/{name}]),
// never the legacy /work_queues/{id} family. The two are NOT equivalent: the
// pool-scoped update renormalizes queue priorities across the pool whenever
// priority changes (models/workers.py bulk_update_work_queue_priorities) and
// the pool-scoped delete rejects deleting the pool's default queue cleanly,
// while the /work_queues/{id} routes write/delete the raw row with neither.

// WorkQueueSpec is the create/update payload for a work queue.
//
// Every optional field is a pointer with omitempty ON PURPOSE: Prefect's
// update paths apply the payload with model_dump(exclude_unset=True), so an
// omitted field is left untouched — that is what makes declared-fields-only
// management possible. WorkQueueUpdate.is_paused in particular is a plain
// bool defaulting to false on the server, so a non-pointer field here would
// silently unpause the queue on every PATCH. Do not switch these to value
// types.
type WorkQueueSpec struct {
	Name             string  `json:"name"`
	Description      *string `json:"description,omitempty"`
	IsPaused         *bool   `json:"is_paused,omitempty"`
	ConcurrencyLimit *int32  `json:"concurrency_limit,omitempty"`
	Priority         *int32  `json:"priority,omitempty"`
}

// WorkQueue is a work queue as returned by the Prefect API.
type WorkQueue struct {
	ID               string  `json:"id"`
	Name             string  `json:"name"`
	Description      *string `json:"description,omitempty"`
	IsPaused         *bool   `json:"is_paused,omitempty"`
	ConcurrencyLimit *int32  `json:"concurrency_limit,omitempty"`
	Priority         *int32  `json:"priority,omitempty"`
	WorkPoolName     string  `json:"work_pool_name,omitempty"`
}

// GetWorkQueue retrieves a work queue within a pool by name via
// GET /work_pools/{pool}/queues/{name}. Returns (nil, nil) when the queue does
// not exist, matching GetWorkPool's contract. A missing pool also 404s, so
// (nil, nil) only means "nothing to adopt" — creation disambiguates the two.
func (c *Client) GetWorkQueue(ctx context.Context, workPoolName, name string) (*WorkQueue, error) {
	var result WorkQueue
	err := c.do(ctx, http.MethodGet, c.endpoint("/work_pools/%s/queues/%s", workPoolName, name), nil, &result)
	if isNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// CreateWorkQueue creates a work queue via POST /work_pools/{pool}/queues.
// A 404 can only mean the pool itself is missing and is returned as
// ErrWorkPoolNotFound so callers can wait for the pool instead of erroring.
func (c *Client) CreateWorkQueue(ctx context.Context, workPoolName string, queue *WorkQueueSpec) (*WorkQueue, error) {
	var result WorkQueue
	err := c.do(ctx, http.MethodPost, c.endpoint("/work_pools/%s/queues", workPoolName), queue, &result)
	if isNotFound(err) {
		return nil, fmt.Errorf("work pool %q: %w", workPoolName, ErrWorkPoolNotFound)
	}
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// UpdateWorkQueue updates a work queue via PATCH /work_pools/{pool}/queues/{name}.
//
// The payload carries only the fields set on the spec — Prefect applies it
// with exclude_unset semantics, leaving omitted fields untouched. Fields named
// in clearFields (CRD JSON names) are reset to their create-time defaults with
// an explicit value: null for concurrencyLimit (nullable column), "" for
// description (NOT NULL column, defaults to ""), false for isPaused. Priority
// has no default (Prefect keeps pool priorities unique and sequential) and
// cannot be cleared. The queue name is the route key and is never sent in the
// payload, so this can never rename.
func (c *Client) UpdateWorkQueue(ctx context.Context, workPoolName, name string, queue *WorkQueueSpec, clearFields []string) error {
	payload := map[string]any{}
	if queue.Description != nil {
		payload["description"] = *queue.Description
	}
	if queue.IsPaused != nil {
		payload["is_paused"] = *queue.IsPaused
	}
	if queue.ConcurrencyLimit != nil {
		payload["concurrency_limit"] = *queue.ConcurrencyLimit
	}
	if queue.Priority != nil {
		payload["priority"] = *queue.Priority
	}
	for _, f := range clearFields {
		switch f {
		case WorkQueueFieldConcurrencyLimit:
			payload["concurrency_limit"] = nil
		case WorkQueueFieldDescription:
			payload["description"] = ""
		case WorkQueueFieldIsPaused:
			payload["is_paused"] = false
		}
	}

	err := c.do(ctx, http.MethodPatch, c.endpoint("/work_pools/%s/queues/%s", workPoolName, name), payload, nil)
	// A 404 means the queue was deleted out-of-band; signal recreate.
	if isNotFound(err) {
		return fmt.Errorf("work queue %q in pool %q: %w", name, workPoolName, ErrWorkQueueNotFound)
	}
	return err
}

// DeleteWorkQueue deletes a work queue via DELETE /work_pools/{pool}/queues/{name},
// which rejects deleting the pool's default queue (as a 500 — the server's
// ValueError isn't mapped to a 4xx) and renormalizes the remaining queue
// priorities; the legacy /work_queues/{id} route does neither.
func (c *Client) DeleteWorkQueue(ctx context.Context, workPoolName, name string) error {
	err := c.do(ctx, http.MethodDelete, c.endpoint("/work_pools/%s/queues/%s", workPoolName, name), nil, nil)
	// Treat 404 as already-deleted (idempotent cleanup).
	if isNotFound(err) {
		return nil
	}
	return err
}
