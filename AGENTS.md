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

### Code Generation Workflow
The operator uses controller-gen for:
- CRD generation from Go types → `deploy/charts/prefect-operator/crds/`
- DeepCopy method generation for all API types
- RBAC manifest generation

Always run `make manifests generate` after modifying API types in `api/v1/`.