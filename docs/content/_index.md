---
title: Prefect Operator
cascade:
  type: docs
---

The Prefect Operator runs [Prefect](https://docs.prefect.io) on Kubernetes and
manages Prefect objects as custom resources. This manifest starts a Prefect
server, a work pool with one worker, and a deployment that runs a flow every
hour:

```yaml
apiVersion: prefect.io/v1
kind: PrefectServer
metadata:
  name: prefect
spec: {}
---
apiVersion: prefect.io/v1
kind: PrefectWorkPool
metadata:
  name: process-pool
spec:
  type: process
  server:
    name: prefect
  workers: 1
---
apiVersion: prefect.io/v1
kind: PrefectDeployment
metadata:
  name: hello
spec:
  server:
    name: prefect
  workPool:
    name: process-pool
  deployment:
    entrypoint: flows/hello_world.py:hello
    pullSteps:
      - prefect.deployments.steps.git_clone:
          repository: https://github.com/PrefectHQ/examples.git
          branch: main
    schedules:
      - slug: hourly
        cron: "0 * * * *"
```

## Custom resources

| Kind | Short name | What the operator does with it |
| --- | --- | --- |
| `PrefectServer` | `ps` | Runs the Prefect API server as a Deployment and Service, with a PVC or a migration Job depending on the database. |
| `PrefectWorkPool` | `pwp` | Creates the work pool in Prefect and runs its workers as a Deployment. |
| `PrefectWorkQueue` | | Creates or adopts a work queue in a work pool and manages its concurrency limit, priority, and pause state. |
| `PrefectDeployment` | | Creates a Prefect deployment: a flow's entrypoint, code source, parameters, and schedules. It creates no Kubernetes objects. |
| `PrefectAutomation` | | Creates a Prefect automation: a trigger and the actions it runs. It creates no Kubernetes objects. |

The work pool, work queue, deployment, and automation resources can point at
a `PrefectServer` in the cluster, a self-hosted Prefect server elsewhere, or a
Prefect Cloud workspace.

## Where to start

- New to Prefect: read [Prefect for Kubernetes users](concepts/prefect-for-kubernetes-users/)
  first. Several Prefect terms collide with Kubernetes terms.
- To try it: [install the operator](getting-started/install/), then follow the
  [quickstart](getting-started/quickstart/).
- Before running it in production: read [How the operator works](concepts/how-it-works/)
  and [Permissions](guides/permissions/).
