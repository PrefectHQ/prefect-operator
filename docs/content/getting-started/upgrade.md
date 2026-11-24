---
title: Upgrade
weight: 3
---

Read the notes below for each release between yours and the new one, then
upgrade the operator. Each
[GitHub release](https://github.com/PrefectHQ/prefect-operator/releases) also
lists every change it includes.

## Upgrade the operator

With the release manifest, apply the new release's manifest. It updates the
CRDs too:

```sh
VERSION=v1.0.0
kubectl apply --server-side \
  -f https://github.com/PrefectHQ/prefect-operator/releases/download/$VERSION/prefect-operator.yaml
```

With Helm, find the chart version whose `APP VERSION` is the release, and
upgrade to it:

```sh
helm repo update
helm search repo prefect-operator/prefect-operator --versions | head
helm upgrade prefect-operator prefect-operator/prefect-operator \
  --namespace prefect-system --version <chart-version>
```

## v1.0

### Before you upgrade

**Annotate the Secrets and ConfigMaps that hold API keys.** The operator reads
an API key only from an object with the `prefect.io/allowed-api-urls`
annotation, which lists the API URLs the key may be sent to. Until the object
has it, a resource that uses the key has `Synced` `False` with the reason
`PrefectClientError`. Deleting the resource keeps it in `Terminating` for an
hour, and then leaves its object in Prefect. To list the objects that need the
annotation:

```sh
kubectl get prefectworkpools,prefectworkqueues,prefectdeployments,prefectautomations -A \
  -o jsonpath='{range .items[*]}{.metadata.namespace} secret/{.spec.server.apiKey.valueFrom.secretKeyRef.name} configmap/{.spec.server.apiKey.valueFrom.configMapKeyRef.name}{"\n"}{end}' |
  awk '{ for (i = 2; i <= 3; i++) if ($i !~ /\/$/) print $1, $i }' | sort -u
```

```text
data-eng secret/prefect-api-key
ml configmap/prefect-api-key
```

Then annotate each one with the URL its resources use:

```sh
kubectl -n data-eng annotate secret/prefect-api-key \
  prefect.io/allowed-api-urls=https://api.prefect.cloud/api/accounts/<account-id>/workspaces/<workspace-id>
```

See [the annotation](../../guides/prefect-cloud/#the-prefectioallowed-api-urls-annotation)
for which URLs an entry matches.

**Grant the RBAC that the admission policies check.** The release manifest
and the chart now install
[admission policies](../../guides/permissions/#admission-policies). Writing a
`PrefectServer` or `PrefectWorkPool` then requires `create` on `deployments`
in its namespace. Writing a `PrefectWorkQueue`, `PrefectDeployment`, or
`PrefectAutomation` requires `get` on the Secret or ConfigMap that it reads
the API key or auth string from. The policies check new writes only, so existing resources
keep working until someone changes them. With Helm, set
`admissionPolicy.enabled=false` to leave the policies out.

**Set a version on servers and work pools that must not change.** The default
Prefect version is now 3.9.4. It used to be 3.9.1. The pods of a server or work
pool that sets neither `version` nor `image` are replaced. An ephemeral server
loses its database, and a PostgreSQL server runs its migrations. To keep the old
version, set it first:

```sh
kubectl -n data-eng patch prefectserver prefect --type=merge -p '{"spec":{"version":"3.9.1"}}'
```

**Remove API settings from work pools.** A work pool's `settings` can't set
`PREFECT_API_URL`, `PREFECT_API_KEY`, or `PREFECT_API_AUTH_STRING` any more. On
an existing work pool, the operator leaves them out of the worker, in any letter
case, and names them in the `SettingsApplied` condition. The worker then uses the
URL from `spec.server`. The CRD rejects new writes that set them. To list the
work pools that set one:

```sh
kubectl get prefectworkpools -A \
  -o jsonpath='{range .items[*]}{.metadata.namespace}/{.metadata.name} {.spec.settings[*].name}{"\n"}{end}' |
  grep -iE ' PREFECT_API_(URL|KEY|AUTH_STRING)( |$)'
```

Set `spec.server.remoteApiUrl` in place of `PREFECT_API_URL`, and
`spec.server.apiKey` in place of `PREFECT_API_KEY`. The operator uses them too,
so it must be able to reach that URL.

**With `rbac.operator.create=false`, update the operator's role.** It needs
`get` on `secrets`, or a resource that reads an API key or auth string from a
Secret doesn't sync. It also needs `get` on `namespaces`, and `create` and
`patch` on `events` in the `events.k8s.io` API group. Without those two,
a deleted namespace can stay in `Terminating`, and the operator records no
events.

**Run Kubernetes 1.30 or later.** The policies need it, and the chart refuses
to install on earlier versions.

**With Argo CD, use background deletion.** The policies refuse a foreground
delete of a `PrefectServer`, and Argo CD uses foreground deletion by default.
On each Application that includes a server, set the
`PrunePropagationPolicy=background` sync option. If it has the finalizer
`resources-finalizer.argocd.argoproj.io`, change that to
`resources-finalizer.argocd.argoproj.io/background`. See
[Deleting a server](../../concepts/how-it-works/#deleting-a-server).

**With Helm, let the release take over the CRDs.** The chart now upgrades the
CRDs, and the first upgrade fails with an error that a CRD "exists and cannot
be imported into the current release". With Helm 3.17 or later, pass
`--take-ownership` on that upgrade. For an earlier Helm, see
[the chart README](https://github.com/PrefectHQ/prefect-operator/blob/main/deploy/charts/prefect-operator/README.md#upgrading-from-a-chart-that-installed-crds-from-crds).
If you run more than one release of the chart in a cluster, set
`crds.install=false` on all but one.

The chart also creates editor and viewer ClusterRoles for
`PrefectWorkQueue`, `PrefectDeployment`, and `PrefectAutomation`, such as
`prefect-workqueue:editor`. If a ClusterRole with one of those names already
exists outside the release, the upgrade fails. With `--take-ownership`, or with
the release manifest, it replaces that ClusterRole's rules. To keep yours with
Helm, set the chart value for that role to `false`, such as
`rbac.userRoles.prefectWorkQueue.editor.create`.

**Check worker and flow run pods that use an HTTP proxy or Istio.** The host in
an in-cluster server's URL now ends with a dot. If the pods have `HTTP_PROXY`,
add `.svc.cluster.local.`, with the dot, to their `NO_PROXY`. Without that
entry, they send API requests to the proxy in plain HTTP, including the API key
or auth string. If the pods have Istio sidecars, upgrade to Istio 1.27 or
later. See
[Connecting to Prefect](../../concepts/how-it-works/#connecting-to-prefect).

### What changes

- The URL of an in-cluster server ends with the cluster domain and a dot, as in
  `http://prefect.data-eng.svc.cluster.local.:4200/api`. It used to end at
  `.svc`. The worker pods of each work pool that names a server in
  `spec.server.name` are replaced once. On `process` work pools, that interrupts
  the flow runs in progress. The operator reads the domain from its
  pod's DNS search domains. Set
  [`operator.clusterDomain`](../../reference/operator-configuration/) if the
  log line "URLs of in-cluster Prefect servers use this cluster domain" has the
  wrong one.
- `spec.server.name` and `spec.server.namespace` must be DNS labels, which have
  no upper-case letters. An existing resource with another value gets `Synced`
  `False`.
- A `PrefectServer` gets the finalizer `prefect.io/server-cleanup`. Deleting
  the server waits until no resource names it in `spec.server`, unless its
  namespace is being deleted. See
  [Deleting a server](../../concepts/how-it-works/#deleting-a-server). Servers
  stay in `Terminating` if you remove the operator before them. See
  [Uninstall](../install/#uninstall).
- With PostgreSQL, the operator doesn't roll out a server whose migration Job
  failed. It used to roll out the server anyway. See
  [Server storage](../../guides/server-storage/#postgresql).
- When the delete in Prefect fails, the operator keeps the resource's
  finalizer and retries for up to an hour, with a `DeleteFailed` event. It used
  to remove the finalizer at once, and leave the object in Prefect.
- Deleting a `PrefectWorkPool` leaves the work pool in Prefect when the
  resource adopted it, which `status.adopted` records. Work pools that an
  earlier operator synced are recorded as created, so deleting them still
  deletes the pool. To keep one, mark it adopted first:

  ```sh
  kubectl patch prefectworkpool <name> --subresource=status --type=merge -p '{"status":{"adopted":true}}'
  ```

- `remoteApiUrl` must be an http or https URL with a host, and no user
  information, query, or fragment. An existing resource with another URL gets
  `Synced` `False`. The operator doesn't connect to link-local addresses, such
  as `169.254.169.254`.
- A resource that sets both `apiKey.value` and `apiKey.valueFrom` uses
  `valueFrom`. The CRD rejects new writes that set both.
- Removing a field from a `PrefectDeployment` schedule, such as `timezone`,
  now removes it in Prefect on the next sync.
- A resource that waits for a work pool or deployment syncs as soon as the
  `PrefectWorkPool` or `PrefectDeployment` it names syncs. For an object
  created outside the operator, it checks every 30 seconds, where it used to
  check every 5.
- The operator logs JSON at `info` level. It used to log console text at
  `debug` level. Set the chart values `operator.logFormat: console` and
  `operator.logLevel: debug` to get the old output.

### New

- [Server authentication](../../guides/server-auth/): `PrefectServer.spec.auth`
  and `spec.server.auth`. It requires Prefect 3.6.22 or later.
- [Pod templates](../../guides/pod-templates/): `deploymentTemplate` and
  `migrationJobTemplate`.
- Warning events when a sync or a delete fails.
- Editor and viewer ClusterRoles for `PrefectWorkQueue`,
  `PrefectDeployment`, and `PrefectAutomation`. See
  [Roles for people](../../guides/permissions/#roles-for-people).

## Downgrade

v1.0 and later add the finalizer `prefect.io/server-cleanup` to each
`PrefectServer`. Earlier releases don't remove it, so after a downgrade to one
of them, a deleted server stays in `Terminating`, and so does a namespace that
has one. After you downgrade, remove the finalizer from each server:

```sh
kubectl get prefectservers -A -o jsonpath='{range .items[*]}{.metadata.namespace} {.metadata.name}{"\n"}{end}' |
  while read -r namespace name; do
    kubectl -n "$namespace" patch prefectserver "$name" --type=merge -p '{"metadata":{"finalizers":null}}'
  done
```

Do this after the downgrade, because a later release adds the finalizer back
while it runs.
