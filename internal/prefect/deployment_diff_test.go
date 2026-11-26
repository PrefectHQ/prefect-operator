package prefect

import "testing"

func strPtr(s string) *string { return &s }
func intPtr(i int) *int       { return &i }
func boolPtr(b bool) *bool    { return &b }

func matchingRemoteAndDesired() (*Deployment, *DeploymentSpec) {
	remote := &Deployment{
		ID:                     "dep-1",
		Name:                   "etl",
		FlowID:                 "flow-1",
		Paused:                 false,
		Tags:                   []string{"adsb", "etl"},
		Parameters:             map[string]any{"lookback": float64(5)},
		JobVariables:           map[string]any{"env": map[string]any{"FOO": "bar-1"}},
		WorkPoolName:           strPtr("kubernetes-pipelines-pool"),
		WorkQueueName:          strPtr("default"),
		Entrypoint:             strPtr("app/main.py:main"),
		GlobalConcurrencyLimit: &GlobalConcurrencyLimit{Limit: 3},
	}
	desired := &DeploymentSpec{
		Name:             "etl",
		FlowID:           "flow-1",
		Paused:           boolPtr(false),
		Tags:             []string{"etl", "adsb"}, // order differs on purpose
		Parameters:       map[string]any{"lookback": float64(5)},
		JobVariables:     map[string]any{"env": map[string]any{"FOO": "bar-1"}},
		WorkPoolName:     strPtr("kubernetes-pipelines-pool"),
		WorkQueueName:    strPtr("default"),
		Entrypoint:       strPtr("app/main.py:main"),
		ConcurrencyLimit: intPtr(3),
	}
	return remote, desired
}

func withConcurrencyOptions(remote *Deployment, desired *DeploymentSpec) {
	remote.ConcurrencyOptions = &ConcurrencyOptions{CollisionStrategy: "CANCEL_NEW", GracePeriodSeconds: intPtr(120)}
	desired.ConcurrencyOptions = &ConcurrencyOptions{CollisionStrategy: "CANCEL_NEW", GracePeriodSeconds: intPtr(120)}
}

func TestDeploymentUpToDate(t *testing.T) {
	t.Run("matching spec is up to date", func(t *testing.T) {
		remote, desired := matchingRemoteAndDesired()
		if !DeploymentUpToDate(remote, desired) {
			t.Fatal("expected up to date")
		}
	})

	t.Run("nil remote is not up to date", func(t *testing.T) {
		_, desired := matchingRemoteAndDesired()
		if DeploymentUpToDate(nil, desired) {
			t.Fatal("expected not up to date")
		}
	})

	t.Run("desired-omitted fields are ignored", func(t *testing.T) {
		remote, desired := matchingRemoteAndDesired()
		remote.Description = strPtr("remote-only description")
		remote.Path = strPtr("/remote/path")
		desired.Paused = nil
		desired.ConcurrencyLimit = nil
		desired.Parameters = nil
		if !DeploymentUpToDate(remote, desired) {
			t.Fatal("expected up to date when desired omits fields")
		}
	})

	t.Run("deprecated top-level concurrency_limit is ignored when global object present", func(t *testing.T) {
		remote, desired := matchingRemoteAndDesired()
		remote.ConcurrencyLimit = nil // the API always returns null here
		if !DeploymentUpToDate(remote, desired) {
			t.Fatal("expected up to date via global_concurrency_limit")
		}
	})

	t.Run("matching concurrency options are up to date", func(t *testing.T) {
		remote, desired := matchingRemoteAndDesired()
		withConcurrencyOptions(remote, desired)
		if !DeploymentUpToDate(remote, desired) {
			t.Fatal("expected up to date")
		}
	})

	t.Run("concurrency options the spec omits are ignored", func(t *testing.T) {
		remote, desired := matchingRemoteAndDesired()
		withConcurrencyOptions(remote, desired)
		desired.ConcurrencyOptions = nil
		if !DeploymentUpToDate(remote, desired) {
			t.Fatal("expected up to date when desired omits concurrency options")
		}
	})

	concurrencyOptionsMutations := map[string]func(remote *Deployment, desired *DeploymentSpec){
		"remote strategy changed": func(r *Deployment, _ *DeploymentSpec) { r.ConcurrencyOptions.CollisionStrategy = "ENQUEUE" },
		"desired strategy changed": func(_ *Deployment, d *DeploymentSpec) {
			d.ConcurrencyOptions.CollisionStrategy = "ENQUEUE"
		},
		"remote grace period changed": func(r *Deployment, _ *DeploymentSpec) { r.ConcurrencyOptions.GracePeriodSeconds = intPtr(300) },
		"remote grace period unset":   func(r *Deployment, _ *DeploymentSpec) { r.ConcurrencyOptions.GracePeriodSeconds = nil },
		"desired grace period unset":  func(_ *Deployment, d *DeploymentSpec) { d.ConcurrencyOptions.GracePeriodSeconds = nil },
		"options unset remotely":      func(r *Deployment, _ *DeploymentSpec) { r.ConcurrencyOptions = nil },
	}
	for name, mutate := range concurrencyOptionsMutations {
		t.Run("concurrency options "+name+" needs update", func(t *testing.T) {
			remote, desired := matchingRemoteAndDesired()
			withConcurrencyOptions(remote, desired)
			mutate(remote, desired)
			if DeploymentUpToDate(remote, desired) {
				t.Fatal("expected update needed")
			}
		})
	}

	mutations := map[string]func(remote *Deployment, desired *DeploymentSpec){
		"name":                     func(_ *Deployment, d *DeploymentSpec) { d.Name = "other" },
		"flow id":                  func(_ *Deployment, d *DeploymentSpec) { d.FlowID = "flow-2" },
		"description":              func(_ *Deployment, d *DeploymentSpec) { d.Description = strPtr("new") },
		"version":                  func(_ *Deployment, d *DeploymentSpec) { d.Version = strPtr("v2") },
		"work pool":                func(_ *Deployment, d *DeploymentSpec) { d.WorkPoolName = strPtr("other-pool") },
		"work queue":               func(_ *Deployment, d *DeploymentSpec) { d.WorkQueueName = strPtr("other-queue") },
		"entrypoint":               func(_ *Deployment, d *DeploymentSpec) { d.Entrypoint = strPtr("app/other.py:main") },
		"path":                     func(_ *Deployment, d *DeploymentSpec) { d.Path = strPtr("/new/path") },
		"paused":                   func(_ *Deployment, d *DeploymentSpec) { d.Paused = boolPtr(true) },
		"enforce parameter schema": func(r *Deployment, d *DeploymentSpec) { d.EnforceParameterSchema = boolPtr(!r.EnforceParameterSchema) },
		"concurrency limit":        func(_ *Deployment, d *DeploymentSpec) { d.ConcurrencyLimit = intPtr(5) },
		"concurrency limit unset remotely": func(r *Deployment, _ *DeploymentSpec) {
			r.GlobalConcurrencyLimit = nil
			r.ConcurrencyLimit = nil
		},
		"tags":       func(_ *Deployment, d *DeploymentSpec) { d.Tags = []string{"adsb", "jam"} },
		"parameters": func(_ *Deployment, d *DeploymentSpec) { d.Parameters = map[string]any{"lookback": float64(6)} },
		"job variables": func(_ *Deployment, d *DeploymentSpec) {
			d.JobVariables = map[string]any{"env": map[string]any{"FOO": "baz"}}
		},
		"parameter openapi schema": func(_ *Deployment, d *DeploymentSpec) {
			d.ParameterOpenAPISchema = map[string]any{"type": "object"}
		},
		"pull steps": func(_ *Deployment, d *DeploymentSpec) {
			d.PullSteps = []map[string]any{{"prefect.deployments.steps.git_clone": map[string]any{}}}
		},
	}
	for name, mutate := range mutations {
		t.Run("changed "+name+" needs update", func(t *testing.T) {
			remote, desired := matchingRemoteAndDesired()
			mutate(remote, desired)
			if DeploymentUpToDate(remote, desired) {
				t.Fatal("expected update needed")
			}
		})
	}
}

func TestDeploymentClearsApplied(t *testing.T) {
	t.Run("a pending options clear is applied once the remote has none", func(t *testing.T) {
		remote, _ := matchingRemoteAndDesired()
		if !DeploymentClearsApplied(remote, []string{DeploymentFieldConcurrencyOptions}) {
			t.Fatal("expected clear applied")
		}
	})

	t.Run("a pending options clear is not applied while the remote keeps them", func(t *testing.T) {
		remote, _ := matchingRemoteAndDesired()
		remote.ConcurrencyOptions = &ConcurrencyOptions{CollisionStrategy: "CANCEL_NEW"}
		if DeploymentClearsApplied(remote, []string{DeploymentFieldConcurrencyOptions}) {
			t.Fatal("expected clear pending")
		}
	})

	t.Run("an options clear ignores the concurrency limit", func(t *testing.T) {
		remote, _ := matchingRemoteAndDesired()
		if !DeploymentClearsApplied(remote, []string{DeploymentFieldConcurrencyOptions}) {
			t.Fatal("expected clear applied with the limit still set")
		}
	})
}
