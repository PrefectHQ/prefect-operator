---
title: Quickstart
weight: 2
---

This runs a Prefect server, a worker, and a scheduled flow in one namespace.
It takes about five minutes, most of it pulling the Prefect image.

## 1. Start a Prefect server

```yaml {filename="server.yaml"}
apiVersion: prefect.io/v1
kind: PrefectServer
metadata:
  name: prefect
spec: {}
```

```sh
kubectl apply -f server.yaml
kubectl get prefectserver prefect --watch
```

An empty spec gives an ephemeral server: its database is SQLite in an
`emptyDir`, so it loses everything when the pod restarts. Wait for `READY` to
become `true`:

```text
NAME      VERSION   READY
prefect   3.9.4     true
```

The operator created a Deployment and a Service, both named `prefect`, and owns
them:

```sh
kubectl get deployment,service -l prefect.io/server=prefect
```

## 2. Add a work pool and a worker

A work pool is a named queue of flow runs in Prefect. A worker is a process
that polls one work pool and runs what it finds. The `process` type runs each
flow as a subprocess of the worker, which keeps this example to one pod.

```yaml {filename="work-pool.yaml"}
apiVersion: prefect.io/v1
kind: PrefectWorkPool
metadata:
  name: process-pool
spec:
  type: process
  server:
    name: prefect
  workers: 1
```

```sh
kubectl apply -f work-pool.yaml
kubectl get prefectworkpool process-pool
```

```text
NAME           TYPE      VERSION   READY   DESIRED WORKERS   READY WORKERS
process-pool   process   3.9.4     true    1                 1
```

## 3. Deploy a flow

A Prefect deployment tells Prefect where a flow's code is and when to run it.
This one clones a public repository and runs `hello` from
`flows/hello_world.py` every minute:

```yaml {filename="deployment.yaml"}
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
    parameters:
      name: Kubernetes
    schedules:
      - slug: every-minute
        cron: "* * * * *"
```

```sh
kubectl apply -f deployment.yaml
kubectl get prefectdeployment hello -o jsonpath='{.status.conditions[?(@.type=="Synced")]}{"\n"}'
```

Within a minute or two, the worker's logs show the flow run:

```sh
kubectl logs deployment/process-pool -f
```

## 4. Open the Prefect UI

```sh
kubectl port-forward service/prefect 4200:4200
```

Open <http://localhost:4200>. The deployment `hello` is under **Deployments**,
and its runs are under **Runs**.

## Clean up

Delete the server last:

```sh
kubectl delete -f deployment.yaml -f work-pool.yaml
kubectl delete -f server.yaml
```

The first command waits while the operator deletes the deployment and the work
pool from Prefect. Deleting the `PrefectServer` then deletes its Deployment and
Service through their owner references.

If you delete the server at the same time, the other two can't reach it to
delete their objects. They stay in `Terminating` for an hour, until the
operator gives up. See [Deletion](../../concepts/how-it-works/#deletion).

## Next steps

- Keep the server's data: [Server storage](../../guides/server-storage/).
- Run each flow in its own pod: set the work pool's `type` to `kubernetes`. See
  [Prefect for Kubernetes users](../../concepts/prefect-for-kubernetes-users/#work-pool-types).
- Use Prefect Cloud instead of an in-cluster server:
  [Connect to Prefect Cloud](../../guides/prefect-cloud/).
