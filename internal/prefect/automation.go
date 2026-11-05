package prefect

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// AutomationSpec is the request payload for creating/updating an automation.
// Trigger and actions are kept as generic maps so the client stays agnostic to
// the (large, evolving) Prefect trigger/action schema; the convert layer builds
// them from the typed PrefectAutomation.
type AutomationSpec struct {
	Name             string           `json:"name"`
	Description      string           `json:"description"`
	Enabled          *bool            `json:"enabled,omitempty"`
	Trigger          map[string]any   `json:"trigger"`
	Actions          []map[string]any `json:"actions"`
	ActionsOnTrigger []map[string]any `json:"actions_on_trigger,omitempty"`
	ActionsOnResolve []map[string]any `json:"actions_on_resolve,omitempty"`
}

// Automation is a Prefect automation as returned by the API.
type Automation struct {
	ID               string           `json:"id"`
	Created          time.Time        `json:"created"`
	Updated          time.Time        `json:"updated"`
	Name             string           `json:"name"`
	Description      string           `json:"description"`
	Enabled          bool             `json:"enabled"`
	Trigger          map[string]any   `json:"trigger"`
	Actions          []map[string]any `json:"actions"`
	ActionsOnTrigger []map[string]any `json:"actions_on_trigger"`
	ActionsOnResolve []map[string]any `json:"actions_on_resolve"`
}

// CreateAutomation creates a new automation via POST /automations/.
func (c *Client) CreateAutomation(ctx context.Context, automation *AutomationSpec) (*Automation, error) {
	var result Automation
	if err := c.do(ctx, http.MethodPost, c.endpoint("/automations/"), automation, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetAutomation retrieves an automation by ID via GET /automations/{id}. It
// returns (nil, nil) when the automation does not exist.
func (c *Client) GetAutomation(ctx context.Context, id string) (*Automation, error) {
	return getObject[Automation](ctx, c, c.endpoint("/automations/%s", id))
}

// UpdateAutomation updates an automation via PUT /automations/{id}. A PUT
// replaces the whole automation, so fields unset in the spec are cleared. It
// returns the automation as read back after the update.
func (c *Client) UpdateAutomation(ctx context.Context, id string, automation *AutomationSpec) (*Automation, error) {
	reqURL := c.endpoint("/automations/%s", id)
	if err := c.do(ctx, http.MethodPut, reqURL, automation, nil); err != nil {
		return nil, err
	}
	// Prefect answers the PUT with 204 and no body, so read the automation back.
	var result Automation
	if err := c.do(ctx, http.MethodGet, reqURL, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// FindDeploymentByName resolves a deployment by name (tenant-wide) via
// POST /deployments/filter. Deployment names are only unique per flow (the
// canonical reference is "{flow}/{deployment}"), so a bare name can match more
// than one deployment. When no deployment matches, the error matches
// ErrNotFound. When more than one matches, it returns a different error
// rather than resolving to an arbitrary one.
func (c *Client) FindDeploymentByName(ctx context.Context, name string) (*Deployment, error) {
	filter := map[string]any{
		"deployments": map[string]any{
			keyName: map[string]any{"any_": []string{name}},
		},
		"limit": 2,
	}
	var results []Deployment
	if err := c.do(ctx, http.MethodPost, c.endpoint("/deployments/filter"), filter, &results); err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("no deployment found with name %q: %w", name, ErrNotFound)
	}
	if len(results) > 1 {
		return nil, fmt.Errorf("deployment name %q is ambiguous: it matches multiple deployments (names are only unique per flow); use a flow-qualified reference", name)
	}
	return &results[0], nil
}

// DeleteAutomation deletes an automation via DELETE /automations/{id}.
func (c *Client) DeleteAutomation(ctx context.Context, id string) error {
	return c.deleteObject(ctx, c.endpoint("/automations/%s", id))
}
