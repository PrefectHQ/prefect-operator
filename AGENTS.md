# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Development Commands

### Prerequisites Setup

```bash
mise trust
mise install     # Installs all dependencies including go, docker, kubectl, etc.
```

### Core Development

```bash
make help        # Show all available make targets
make build       # Build manager binary
make run         # Run controller locally
make test        # Run unit tests with coverage
make test-e2e    # Run end-to-end tests
make watch       # Watch tests continuously
make lint        # Run golangci-lint and yamllint
make lint-fix    # Run linters with auto-fix
```

Once the project has been built at least once, you can also run `ginkgo`
directly to execute individual tests. See the @Makefile for how to run it.

### Code Generation

```bash
make manifests   # Generate CRDs, webhooks, ClusterRoles
make generate    # Generate DeepCopy methods
make docs        # Generate CRD documentation (PrefectServer.md, PrefectWorkPool.md)
```

### Kubernetes Operations

```bash
make install     # Install CRDs to cluster
make uninstall   # Remove CRDs from cluster
make deploy      # Deploy operator via Helm
make undeploy    # Remove operator deployment
kubectl apply -k deploy/samples/  # Apply sample resources
```

### Docker Operations

```bash
make docker-build IMG=<registry>/prefect-operator:tag
make docker-push IMG=<registry>/prefect-operator:tag
```

## Architecture

This is a Kubernetes operator built with Kubebuilder that manages Prefect infrastructure components.

### Core Custom Resources

- **PrefectServer**: Manages Prefect server deployments with configurable storage backends (ephemeral, SQLite, PostgreSQL) and optional Redis messaging
- **PrefectWorkPool**: Manages Prefect work pools for different execution environments (Kubernetes, process-based, external)
- **PrefectDeployment**: Manages Prefect deployments (not Kubernetes ones) of Python flows

### Key Components

- **Controllers** (`internal/controller/`): Reconciliation logic for CRDs
- **API Types** (`api/v1/`): CRD definitions and helper methods
- **Utils** (`internal/utils/`): Hash utilities for detecting resource changes
- **Conditions** (`internal/conditions/`): Status condition management
- **Constants** (`internal/constants/`): Shared constants

### Deployment Architecture

- Main controller runs as a deployment in cluster
- Uses controller-runtime for Kubernetes API interactions
- Supports namespace-scoped operation via `WATCH_NAMESPACES` env var
- Health checks on `:8081`, metrics on `:8080`
- Leader election enabled for HA deployments

### Storage Backend Configuration

PrefectServer supports three storage modes:

1. **Ephemeral**: In-memory SQLite (no persistence)
2. **SQLite**: Persistent SQLite with PVC storage
3. **PostgreSQL**: External PostgreSQL database with connection details

Each backend generates appropriate environment variables for the Prefect server container.

### Testing Strategy

- Unit tests use Ginkgo/Gomega with envtest for Kubernetes API simulation
- E2E tests require actual Kubernetes cluster
- Coverage includes API types, controllers, and utilities
- Tests run against Kubernetes v1.29.0 via envtest

The typically manual testing approach is to run a local cluster like `minikube`
and run the `prefect-operator` locally pointing to that cluster. That means
that for many testing scenarios, we can't assume that the operator is running
in the same cluster it is operating on. Use port-forwarding where appropriate
to support these cases.

### Local Testing

For local testing and development, first start a local Kubernetes cluster:

```bash
minikube start  # Start local Kubernetes cluster (if minikube is available)
```

Then run the operator locally:

```bash
make install run  # Install CRDs and run operator locally (long-running process)
```

This command should be run in a background shell as it's a long-lived process that watches and reconciles Prefect resources. The operator will:
- Install/update CRDs to the cluster
- Start the controller manager locally
- Watch for changes to PrefectServer, PrefectWorkPool, and PrefectDeployment resources
- Use port-forwarding to connect to in-cluster Prefect servers when needed

#### Testing with Sample Resources

Apply sample resources from `deploy/samples/` to test different scenarios:

```bash
# Apply a complete end-to-end example with all schedule types
kubectl apply -f deploy/samples/deployment_end-to-end.yaml

# Or test individual components
kubectl apply -f deploy/samples/v1_prefectserver_ephemeral.yaml
kubectl apply -f deploy/samples/v1_prefectworkpool_kubernetes.yaml

# List available sample configurations
ls deploy/samples/
```

#### Accessing the Prefect API

When testing with in-cluster Prefect servers, you can port-forward to access the API directly:

```bash
kubectl port-forward svc/prefect-ephemeral 4200:4200
# Prefect API now available at http://localhost:4200/api
```

This allows you to inspect deployments, schedules, and other resources created by the operator:
```bash
# View all deployments
curl -X POST http://localhost:4200/api/deployments/filter -H "Content-Type: application/json" -d '{}'

# View deployment schedules
curl -X POST http://localhost:4200/api/deployments/filter -H "Content-Type: application/json" -d '{}' | jq '.[] | {name, schedules}'
```

### Code Generation Workflow

The operator uses controller-gen for:

- CRD generation from Go types → `deploy/charts/prefect-operator/crds/`
- DeepCopy method generation for all API types
- RBAC manifest generation

Always run `make manifests generate` after modifying API types in `api/v1/`.
