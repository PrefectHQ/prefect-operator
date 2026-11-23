---
title: Permissions
weight: 5
---

## What the operator can do

The operator runs with a ClusterRole. Its rules are generated from the
controllers and are in
[`files/role.yaml`](https://github.com/PrefectHQ/prefect-operator/blob/main/deploy/charts/prefect-operator/files/role.yaml).
In every namespace it watches, it can:

- create, update, and delete Deployments, Services, and PVCs
- create and delete Jobs
- read ConfigMaps, and read Secrets by name
- read Pods
- read Namespaces by name, to check whether one is being deleted
- create Events
- manage the five Prefect custom resources and their status

It reads Secrets by name only, with no `list` or `watch`: the ones that a
resource's `spec.server.apiKey.valueFrom` names. Database and Redis passwords
are passed to pods as `secretKeyRef` environment variables, so the operator
never reads them.

To limit it to some namespaces, set the `WATCH_NAMESPACES` environment variable
on the operator to a comma-separated list. See
[Operator configuration](../../reference/operator-configuration/).

## What writing a resource lets someone do

The operator acts with its own ServiceAccount, so a resource can make it do
things the person who wrote the resource can't do directly:

| Resource | Lets the writer |
| --- | --- |
| `PrefectServer`, `PrefectWorkPool` | Run any container, with any ServiceAccount, in the resource's namespace, through `extraContainers`, `settings`, `serviceAccountName`, and `deploymentTemplate`. |
| `PrefectWorkQueue`, `PrefectDeployment`, `PrefectAutomation` | Send the spec to Prefect with the API key in any Secret or ConfigMap in the namespace that has the `prefect.io/allowed-api-urls` annotation. |
| `PrefectDeployment` | Run flow code on the work pool's workers, with any job settings the work pool's base job template exposes. |
| `PrefectWorkPool`, `PrefectWorkQueue`, `PrefectDeployment`, `PrefectAutomation` | Send requests from the operator's pod to any http or https URL in `spec.server.remoteApiUrl`, and read up to 4 KiB of an error response in the `Synced` condition. |

The operator can reach every in-cluster `PrefectServer`, so a resource in one
namespace can use a server in another. To control which namespaces can use a
server, [require authentication](../server-auth/) on it.

The operator doesn't connect to link-local addresses, such as the cloud
instance metadata address `169.254.169.254`, even through a host name that
resolves to one. If the operator uses an HTTP proxy, the proxy resolves host
names, so block those addresses in the proxy too. To limit which other hosts the
operator can reach, apply a NetworkPolicy to its pod.

## Admission policies

The chart installs two ValidatingAdmissionPolicies that close the first two
rows. Writing a resource then also requires the RBAC for what the operator does
with it:

| Resource | The requester also needs |
| --- | --- |
| `PrefectServer`, `PrefectWorkPool` | `create` on `deployments` in the resource's namespace. |
| `PrefectWorkQueue`, `PrefectDeployment`, `PrefectAutomation` | `get` on the Secret or ConfigMap that `spec.server.apiKey.valueFrom` names. |

When a request fails a policy, the API server rejects it with a message such as:

```text
the operator creates a Deployment from each PrefectWorkPool, so writing its spec
requires permission to create Deployments in namespace data-eng
```

The policies don't check a request that leaves the spec unchanged, such as the
operator adding its finalizer. They check new requests only, so resources that
already exist aren't checked until someone changes them.

The policies need Kubernetes 1.30 or later. The chart value
`admissionPolicy.enabled` controls whether the chart installs them. Without
them, the `prefect.io/allowed-api-urls` annotation still applies.

## Roles for people

The chart creates an editor and a viewer ClusterRole for each custom resource,
for admins to bind to people. The operator doesn't use them.

| ClusterRole | Grants |
| --- | --- |
| `prefect-server:editor` | Full access to `prefectservers`. |
| `prefect-server:viewer` | `get`, `list`, `watch` on `prefectservers`. |

The others are `prefect-workpool`, `prefect-workqueue`, `prefect-deployment`,
and `prefect-automation`, each with `:editor` and `:viewer`. The editor roles grant only the
Prefect resources, so with the admission policies enabled, also grant the
permissions in the table above. For example, a team that manages its own work
pools in `data-eng`:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: data-eng-prefect
  namespace: data-eng
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: prefect-workpool:editor
subjects:
  - kind: Group
    name: data-eng
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: create-deployments
  namespace: data-eng
rules:
  - apiGroups: ["apps"]
    resources: ["deployments"]
    verbs: ["create"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: data-eng-create-deployments
  namespace: data-eng
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: create-deployments
subjects:
  - kind: Group
    name: data-eng
```
