---
title: How the operator works
weight: 2
---

The custom resources fall into two groups, and the operator reconciles each
group differently.

| Group | Kinds | The operator writes to |
| --- | --- | --- |
| Workload resources | `PrefectServer`, `PrefectWorkPool` | The Kubernetes API: Deployments, Services, PVCs, Jobs. |
| Prefect API resources | `PrefectWorkPool`, `PrefectWorkQueue`, `PrefectDeployment`, `PrefectAutomation` | The Prefect API, over HTTP. |

`PrefectWorkPool` is in both groups. It creates a work pool in Prefect and runs
the pool's workers in Kubernetes.

## Objects the operator creates in Kubernetes

| Resource | Object | Name | Created when |
| --- | --- | --- | --- |
| `PrefectServer` | Deployment | `<name>` | Always. |
| `PrefectServer` | Service | `<name>`, port 4200 named `api` | Always. |
| `PrefectServer` | PersistentVolumeClaim | `<name>-data` | `spec.sqlite` is set. |
| `PrefectServer` | Job | `<name>-migration-<hash>` | `spec.postgres` is set. See [Server storage](../../guides/server-storage/#postgresql). |
| `PrefectWorkPool` | Deployment | `<name>` | Always. `spec.workers` sets its replicas. |

Each object has an owner reference to its resource, with `controller: true`.
Kubernetes garbage collection deletes the objects when you delete the
resource. That includes the SQLite PVC, and with it the server's database. A
`PrefectServer`'s objects are deleted only after its finalizer is removed. See
[Deleting a server](#deleting-a-server).

The operator overwrites changes made directly to these objects on its next
reconcile. To change them, change the resource, or patch the pod template with
[`deploymentTemplate`](../../guides/pod-templates/).

Labels on the objects:

| Label | On |
| --- | --- |
| `prefect.io/server: <name>` | Every object a `PrefectServer` creates. |
| `app: prefect-server` | The server Deployment and its pods. |
| `prefect.io/worker: <name>` | The worker Deployment and its pods. |

`spec.deploymentLabels`, `spec.serviceLabels`, and `spec.migrationJobLabels`
add more.

## Connecting to Prefect

Every resource except `PrefectServer` has a `spec.server` that says which
Prefect API to use:

```yaml
# A PrefectServer in the cluster. namespace defaults to the resource's own.
server:
  name: prefect
  namespace: prefect
```

```yaml
# Prefect Cloud, or any Prefect server outside the cluster.
server:
  remoteApiUrl: https://api.prefect.cloud
  accountId: <account-id>
  workspaceId: <workspace-id>
  apiKey:
    valueFrom:
      secretKeyRef:
        name: prefect-api-key
        key: api-key
```

For an in-cluster server, the API URL is
`http://<name>.<namespace>.svc.cluster.local.:4200/api`, or the same with your
[cluster domain](../../reference/operator-configuration/) if it isn't
`cluster.local`.
For a remote one, it's `remoteApiUrl` with `/api` appended if it's missing,
then `/accounts/<accountId>/workspaces/<workspaceId>` when both IDs are set.
The operator and the workers use the same URL. A Secret or ConfigMap holding
an API key must name that URL in an annotation. See
[Connect to Prefect Cloud](../../guides/prefect-cloud/).

The dot at the end of an in-cluster host makes a resolver look up that exact
name, without the search domains in `/etc/resolv.conf`. The dot has two other
effects:

- `NO_PROXY` entries such as `.cluster.local` don't match the host. If a worker
  or flow run pod has `HTTP_PROXY`, add `.svc.cluster.local.`, with the dot, to
  its `NO_PROXY`. Without that entry, the pod sends its API requests to the
  proxy in plain HTTP, with any credentials they have. The operator itself
  doesn't use a proxy for an in-cluster server.
- Istio sidecars before Istio 1.27 don't route the host to the Service. With
  strict mutual TLS, requests from a worker or flow run pod with one fail.

When the operator runs outside the cluster, as it does during development, it
reaches an in-cluster server through `kubectl port-forward`.

## Names in Prefect

| Resource | Name of the Prefect object |
| --- | --- |
| `PrefectWorkPool` | `metadata.name` |
| `PrefectWorkQueue` | `spec.name`, in the work pool `spec.workPoolName` |
| `PrefectDeployment` | `metadata.name`, under the flow named in `spec.deployment.entrypoint` after the `:`, or `spec.deployment.flow_name` |
| `PrefectAutomation` | `spec.name` |

Work pool names are global within a Prefect server or workspace. Two
`PrefectWorkPool` resources with the same name in different namespaces manage
the same work pool, and each one overwrites the other's settings.

## Syncing with Prefect

The operator syncs a Prefect API resource when any of these is true:

- It has no Prefect ID in `status.id` yet.
- Its spec changed since the last sync.
- Its resync interval has passed since `status.lastSyncTime`.

The resync interval is the resource's `spec.interval`, or the operator's
`--default-resync-interval` flag when that's unset. The default is 5 minutes,
and values below 10 seconds are raised to 10 seconds. The operator adds up to
10% of random jitter so that resources don't all sync at once.

A resync corrects drift. If someone edits a managed object in the Prefect UI,
the next resync writes the spec back. If someone deletes it in Prefect, the
next resync creates it again.

Some Prefect objects lose state on every update, so the operator reads the
object first and skips the update when nothing changed:

- Updating a deployment deletes its future scheduled runs, which Prefect then
  schedules again.
- Updating an automation resets the state of a proactive trigger.

### Dependencies between resources

A `PrefectDeployment` or `PrefectWorkQueue` that names a work pool waits until
the work pool exists in Prefect. Its `Synced` condition is `False` with the
reason `WorkPoolNotFound`, and it checks again every 30 seconds. When a
`PrefectWorkPool` with that name first syncs, the operator reconciles the
waiting resources right away.

A `PrefectAutomation` whose actions name a deployment waits the same way, with
the reason `DeploymentNotFound`.

You can apply all the resources at once, in any order.

## Adoption

A Prefect object can exist before the resource that manages it. For example,
Prefect creates a work queue implicitly when a deployment names one.

| Resource | If the object already exists |
| --- | --- |
| `PrefectWorkPool` | Adopts it, and sets `status.adopted: true`. |
| `PrefectWorkQueue` | Adopts it, and sets `status.adopted: true`. |
| `PrefectDeployment` | Takes it over. Prefect treats creating a deployment with an existing flow and name as an update. `status.adopted` is not recorded. |
| `PrefectAutomation` | Creates a second automation with the same name. Automation names are not unique in Prefect. |

The operator decides adoption on the first sync and doesn't change it later.

## Deletion

Every resource has a finalizer:

| Resource | Finalizer |
| --- | --- |
| `PrefectServer` | `prefect.io/server-cleanup` |
| `PrefectWorkPool` | `prefect.io/work-pool-cleanup` |
| `PrefectWorkQueue` | `prefect.io/workqueue-cleanup` |
| `PrefectDeployment` | `prefect.io/deployment-cleanup` |
| `PrefectAutomation` | `prefect.io/automation-cleanup` |

When you delete a Prefect API resource, the operator deletes its object in
Prefect and then removes the finalizer. It leaves adopted work pools and work
queues in place, because something else created them and may still depend on
them. A `PrefectDeployment` deletes the deployment it synced, even one it took
over.

If the delete in Prefect fails, the operator keeps the finalizer, records a
`DeleteFailed` Warning event, and retries. The first retry comes after one
second, and each wait after that is about twice as long, up to 30 seconds.
One hour after the resource's deletion timestamp, it removes the finalizer
anyway. The event then says that the object may still exist in Prefect.

In a namespace that is being deleted, the operator doesn't wait an hour when
the delete needs an object that the namespace's deletion already removed: the
Secret or ConfigMap with the API key or auth string, or the Service of the
in-cluster server. Nothing can create that object again, so after the first
failed delete the operator removes the finalizer. Kubernetes doesn't accept
events in a namespace that is being deleted, so the operator's log is the only
record. It names the missing object in its `deleted` field.

To remove a resource without touching Prefect, remove its finalizer:

```sh
kubectl patch prefectdeployment hello --type=merge -p '{"metadata":{"finalizers":null}}'
kubectl delete prefectdeployment hello
```

### Deleting a server

Delete a server together with the resources that use it:

```sh
kubectl delete -f server.yaml -f work-pool.yaml -f deployment.yaml
```

A `PrefectServer` keeps its finalizer while a `PrefectWorkPool`,
`PrefectWorkQueue`, `PrefectDeployment`, or `PrefectAutomation` names it in
`spec.server`. That includes resources in other namespaces that the operator
watches. It leaves out resources that also set `remoteApiUrl`, because they use
that URL. The server's Deployment and Service keep running, so those resources
can still delete their objects from it. When the last one is gone, the
operator removes the finalizer, and garbage collection deletes the Deployment
and Service.

Deleting the server's namespace doesn't keep the server running. Kubernetes
deletes the namespace's Deployments, Services, and Secrets directly, so the
operator removes the server's finalizer without waiting. A resource that hasn't
deleted its object from the server by then can't reach it. The operator removes
the finalizer of a resource in that namespace, as described in
[Deletion](#deletion), and its object stays in the server's database. A resource
in another namespace isn't deleted, and fails to sync. If you delete it after
the server's namespace is gone, it stays in `Terminating` for up to an hour. With an ephemeral or SQLite server, the
database is deleted with the namespace. With PostgreSQL, delete the resources
before the namespace to remove their objects from Prefect.

If you delete only the server, it waits until you delete the others, with no
time limit. `kubectl delete` blocks until then, unless you pass
`--wait=false`. The `DeletionBlocked` condition names up to five of the
resources, and a `DependentsExist` Warning event repeats its message:

```sh
kubectl get prefectserver prefect -o jsonpath='{.status.conditions[?(@.type=="DeletionBlocked")].message}'
```

```text
2 resources use this server and must be deleted first: PrefectDeployment default/hello, PrefectWorkPool default/process-pool
```

To delete the server without waiting, remove its finalizer. The resources
that use it then can't reach it, and they stay in `Terminating` until the
operator gives up after an hour:

```sh
kubectl patch prefectserver prefect --type=merge -p '{"metadata":{"finalizers":null}}'
```

The server waits only with background cascading deletion, which is the
default for `kubectl delete`. With `--cascade=foreground`, garbage collection
deletes the Deployment and Service before the server, whatever finalizers the
server has. The chart's [admission policies](../../guides/permissions/#admission-policies)
refuse a foreground delete of a `PrefectServer`, even one that nothing uses.

Argo CD uses foreground deletion by default. On each Application that includes
a server, set the sync option for prunes, and the finalizer for deleting the
Application itself:

```yaml
metadata:
  finalizers:
    - resources-finalizer.argocd.argoproj.io/background
spec:
  syncPolicy:
    syncOptions:
      - PrunePropagationPolicy=background
```

Garbage collection also uses foreground deletion, for a server that has an
owner reference to an object deleted that way. The policy refuses it, and the
owner stays in `Terminating` until you delete the server yourself.

## Status

Every resource reports [conditions](https://kubernetes.io/docs/concepts/overview/working-with-objects/object-management/#conditions)
in `status.conditions`. `kubectl describe` shows them, along with the events
the operator records.

| Resource | Condition | Meaning |
| --- | --- | --- |
| Prefect API resources | `Synced` | The last sync with Prefect succeeded. When `False`, the reason and message say why. |
| Prefect API resources | `Ready` | The object exists in Prefect and matches the spec. |
| `PrefectServer` | `DeploymentReconciled`, `ServiceReconciled` | The operator created or updated the object. `DeploymentReconciled` is `False` with the reason `UnsupportedVersion` when `spec.auth` is set and the image is a Prefect release that [doesn't support it](../../guides/server-auth/#prefect-version), or the reason `InvalidAuth` when the Secret has no valid auth string or the server's `settings` [set another one](../../guides/server-auth/#rules). |
| `PrefectServer` | `AuthSupported` | `True` when the server's [Prefect version](../../guides/server-auth/#prefect-version) supports `spec.auth`, and `False` when it doesn't. `Unknown` when the operator can't tell the version of the image. Without a valid auth string, it is neither `True` nor `Unknown`. |
| `PrefectServer` | `PersistentVolumeClaimReconciled`, `MigrationJobReconciled` | As above. The reason ends in `NotRequired` when the storage backend doesn't use the object. |
| `PrefectServer` | `DeletionBlocked` | The server is being deleted and waits for the resources that use it. The message names them. See [Deleting a server](#deleting-a-server). |
| `PrefectServer` | `SharedStateConfigured` | `False` when more than one replica runs without Redis lease storage. See [Server storage](../../guides/server-storage/#more-than-one-replica). |
| `PrefectServer`, `PrefectWorkPool` | `DeploymentTemplateApplied`, `MigrationJobTemplateApplied` | The pod template patch was applied. See [Pod templates](../../guides/pod-templates/). |
| `PrefectWorkPool` | `DeploymentReconciled` | The operator created or updated the worker Deployment. |
| `PrefectWorkPool` | `SettingsApplied` | Present only when the operator left some `settings` out of the worker, because they set a variable that `spec.server` sets, such as `prefect_api_url`. The message names them. |

`Synced` reasons you may see:

| Reason | Cause |
| --- | --- |
| `PrefectClientError` | The operator couldn't build a client: the Secret or ConfigMap with the API key or auth string is missing, lacks the `prefect.io/allowed-api-urls` annotation, or doesn't allow the URL. |
| `SyncError` | The Prefect API returned an error. The message includes it. |
| `WorkPoolNotFound`, `DeploymentNotFound` | A dependency doesn't exist in Prefect yet. See [Dependencies](#dependencies-between-resources). |
| `ConversionError` | The spec couldn't be turned into a Prefect API request, for example because of an invalid schedule. |
| `InvalidSpec` | The spec breaks a rule that the CRD's validation didn't catch, such as an automation without a trigger. |
| `BaseJobTemplateError` | The work pool's base job template ConfigMap or key doesn't exist. |
| `Recreating` | The object was deleted in Prefect during the sync. The next sync, one second later, creates it again. |
