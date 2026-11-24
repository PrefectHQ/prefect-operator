---
title: Troubleshooting
weight: 6
---

Start with the resource's conditions and events, then the operator's logs:

```sh
kubectl describe prefectdeployment hello
kubectl logs deployment/prefect-operator -n prefect-system
```

The tables of condition types and `Synced` reasons are in
[How the operator works](../../concepts/how-it-works/#status).

## A resource is stuck in `Terminating`

The operator couldn't delete the object in Prefect and is retrying. Look for
`DeleteFailed` events:

```sh
kubectl get events --field-selector reason=DeleteFailed
```

The operator gives up and removes the finalizer one hour after the deletion
started. When the resource's namespace is being deleted and the Secret,
ConfigMap, or server Service that the delete needs is gone, it gives up after
the first failed delete. To remove the finalizer sooner, and leave the object
in Prefect, see [Deletion](../../concepts/how-it-works/#deletion).

If the operator isn't running, nothing removes the finalizer. Start the
operator again, or remove the finalizer by hand.

## A `PrefectServer` is stuck in `Terminating`

The server waits until no resource names it in `spec.server`, with no time
limit. Its `DeletionBlocked` condition says which resources those are:

```sh
kubectl get prefectserver prefect -o jsonpath='{.status.conditions[?(@.type=="DeletionBlocked")].message}'
```

```text
1 resource uses this server and must be deleted first: PrefectWorkPool default/process-pool
```

Delete those resources. The operator deletes the server after the last one is
gone. See [Deleting a server](../../concepts/how-it-works/#deleting-a-server).

## Deleting a `PrefectServer` is refused

```text
ValidatingAdmissionPolicy 'prefect-operator-server-deletion' with binding 'prefect-operator-server-deletion' denied request: foreground deletion deletes a PrefectServer's Deployment and Service before the resources that use the server can delete their objects from it; ...
```

The chart's admission policies refuse foreground deletes of servers. Delete
the server without `--cascade=foreground`. If Argo CD's prune fails with this
message, set the `PrunePropagationPolicy=background` sync option on the
Application.

If deleting an Argo CD Application fails with it, delete the server with
`kubectl delete prefectserver`, and Argo CD finishes deleting the rest. Kubernetes
doesn't allow a new finalizer on the Application once its deletion has started,
so change the finalizer of other Applications that include a server beforehand.
See [Deleting a server](../../concepts/how-it-works/#deleting-a-server).

## `Synced` is `False` with `PrefectClientError`

The operator couldn't read the API key or auth string. The message says which
of these it is:

- The Secret or ConfigMap doesn't exist in the resource's namespace.
- It doesn't have the `prefect.io/allowed-api-urls` annotation.
- The annotation doesn't list the resource's API URL. The message includes the
  URL the operator computed. Compare it with the annotation, as described in
  [Connect to Prefect Cloud](../prefect-cloud/#the-prefectioallowed-api-urls-annotation).

## `Synced` is `False` with status 401

The server requires an auth string, and the resource doesn't send it or sends
the wrong one. Set `spec.server.auth` to a Secret with the server's auth
string. See [Require authentication](../server-auth/).

## Flow run pods have `CreateContainerConfigError`

A flow run on a `kubernetes` work pool with `spec.server.auth` reads the auth
string from a Secret, which must exist in the namespace its Job runs in. See
[Flow runs in Kubernetes Jobs](../server-auth/#flow-runs-in-kubernetes-jobs).

## `Synced` is `False` with `WorkPoolNotFound`

The work pool the resource names doesn't exist in Prefect yet. If a
`PrefectWorkPool` creates it, check that resource's `Synced` condition. The
names must match exactly, and work pool names are global to the Prefect server
or workspace, not per namespace.

## The worker pods aren't ready

The worker serves its health check on port 8080 at `/health`, and the
liveness probe starts after 120 seconds. Check its logs:

```sh
kubectl logs deployment/<work-pool-name> -c prefect-worker
```

Common causes are a wrong `PREFECT_API_URL`, which the operator derives from
`spec.server`, and a `kubernetes` worker whose ServiceAccount can't create Jobs.

## The server rollout doesn't start

With PostgreSQL, the operator waits for the migration Job to complete before it
updates the server Deployment. Check the `MigrationJobReconciled` condition and
the Job's pods:

```sh
kubectl get jobs -l prefect.io/server=<name>
kubectl logs job/<name>-migration-<hash> -c wait-for-database
kubectl logs job/<name>-migration-<hash> -c prefect-server-migration
```

A Job stuck in `wait-for-database` can't reach the database host.
