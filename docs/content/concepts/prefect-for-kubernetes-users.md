---
title: Prefect for Kubernetes users
weight: 1
---

Prefect schedules and runs Python workflows. This page covers the Prefect
terms that the custom resources use. The
[Prefect docs](https://docs.prefect.io/v3/concepts) cover the rest.

## Terms

| Prefect term | What it is | Closest Kubernetes idea |
| --- | --- | --- |
| Flow | A Python function decorated with `@flow`. | The program in a container image. |
| Flow run | One execution of a flow. | A Job's pod. |
| Server | The Prefect API and UI, backed by a database. Prefect Cloud is a hosted server. | The API server and etcd. |
| Deployment | A flow's code location, default parameters, schedules, and work pool. **Not a Kubernetes Deployment.** | A CronJob, without the pod template. |
| Work pool | A named queue of scheduled flow runs, plus a job template that describes the infrastructure for each run. | A scheduler's queue. |
| Work queue | A subdivision of a work pool with its own priority and concurrency limit. | A priority class with a quota. |
| Worker | A long-running process that polls one work pool and starts each flow run on the pool's infrastructure. | A controller that turns queue entries into pods. |
| Automation | A trigger on Prefect events or metrics, and the actions it runs, such as pausing a deployment or sending a notification. | An alerting rule with a webhook. |

The name collision matters in `kubectl` output. A `PrefectDeployment` creates
a Prefect deployment through the Prefect API. It creates no Kubernetes
Deployment. A `PrefectServer` and a `PrefectWorkPool` each create a Kubernetes
Deployment.

## How a flow run reaches a pod

1. A schedule on a deployment, or a person in the UI, creates a flow run in
   the server. The run is scheduled on the deployment's work pool.
2. A worker polling that work pool picks up the run.
3. The worker starts the run on the pool's infrastructure, filling in the
   pool's base job template with the deployment's `jobVariables`.
4. The flow run's process pulls the flow code, using the deployment's
   `pullSteps`, and runs the flow. It reports state back to the server.

The worker and the flow run both talk to the Prefect API over HTTP. Neither
needs to reach the Kubernetes API, except for a `kubernetes` worker creating
Jobs.

## Work pool types

`spec.type` on a `PrefectWorkPool` decides where flow runs execute.

| Type | Where each flow run runs | What the worker needs |
| --- | --- | --- |
| `process` | A subprocess in the worker's pod. | Nothing beyond its pod. Flow runs share the worker's CPU, memory, and image. |
| `kubernetes` | A Kubernetes Job that the worker creates. | A ServiceAccount, set with `spec.serviceAccountName`, that may create and watch Jobs and Pods in the namespace the job template names. |

For `kubernetes`, the operator uses the `prefecthq/prefect:<version>-python3.12-kubernetes`
image for the worker. The operator does not create the worker's ServiceAccount
or its RBAC. A minimal Role for a worker that runs Jobs in its own namespace:

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: prefect-worker
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: prefect-worker
rules:
  - apiGroups: ["batch"]
    resources: ["jobs"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
  - apiGroups: [""]
    resources: ["pods", "pods/log", "events"]
    verbs: ["get", "list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: prefect-worker
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: prefect-worker
subjects:
  - kind: ServiceAccount
    name: prefect-worker
```

Check the Role against the [Prefect Kubernetes worker docs](https://docs.prefect.io/integrations/prefect-kubernetes)
for the Prefect version you run.

## Job templates and job variables

A work pool's base job template is a JSON document with two parts. `variables`
is a JSON schema of the settings a deployment may change, such as `image` or
`namespace`. `job_configuration` is the infrastructure spec, with
`{{ variable }}` placeholders. For a `kubernetes` pool, `job_configuration`
holds a Job manifest.

`spec.baseJobTemplate` on a `PrefectWorkPool` picks the template:

| Field | Template |
| --- | --- |
| unset | The worker type's default, which the operator reads from the Prefect server. |
| `value` | An inline JSON object. |
| `configMap` | A key in a ConfigMap in the work pool's namespace, holding the template as a JSON string. The operator watches the ConfigMap and syncs the work pool again when it changes. |
| `patches` | [RFC 6902](https://datatracker.ietf.org/doc/html/rfc6902) JSON patches applied to whichever template the other fields pick, including the default. |

For example, to give every flow run of a `kubernetes` pool a default memory
request without writing out the whole default template:

```yaml
spec:
  type: kubernetes
  baseJobTemplate:
    patches:
      - op: add
        path: /job_configuration/job_manifest/spec/template/spec/containers/0/resources
        value:
          requests:
            memory: 512Mi
```

A deployment's `spec.deployment.jobVariables` fills in the variables. A
deployment can change only what the template exposes as a variable.

This makes the work pool's template the place to set limits on flow-run pods.
Anyone who can write a `PrefectDeployment` can choose any value the template's
variables allow.
