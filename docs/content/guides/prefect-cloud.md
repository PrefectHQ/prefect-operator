---
title: Connect to Prefect Cloud
weight: 2
---

To manage objects in a Prefect Cloud workspace, or in a Prefect server outside
the cluster, give each resource a `remoteApiUrl` and an API key.

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: prefect-api-key
  annotations:
    prefect.io/allowed-api-urls: https://api.prefect.cloud/api/accounts/<account-id>/workspaces/<workspace-id>
stringData:
  api-key: pnu_...
---
apiVersion: prefect.io/v1
kind: PrefectWorkPool
metadata:
  name: k8s-pool
spec:
  type: kubernetes
  workers: 2
  serviceAccountName: prefect-worker
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

The account and workspace IDs are in the workspace's URL in the Prefect Cloud
UI: `https://app.prefect.cloud/account/<account-id>/workspace/<workspace-id>`.

For a self-hosted server, set `remoteApiUrl` to its API URL, such as
`https://prefect.example.com/api`, and leave out the IDs.

## The `prefect.io/allowed-api-urls` annotation

The operator reads an API key from a Secret or ConfigMap only if the object has
the annotation `prefect.io/allowed-api-urls`. Its value is a comma-separated
list of the Prefect API URLs that the key may be used for. The annotation
stops a resource in the namespace from sending the key to a URL its owner
didn't choose.

An entry matches the resource's API URL when:

- the scheme and host, including any port, are the same, ignoring a dot at the
  end of the host name, and
- the entry's path is the API URL's path, or a parent of it.

| Entry | Allows |
| --- | --- |
| `https://api.prefect.cloud/api/accounts/A/workspaces/W` | Workspace `W` only. |
| `https://api.prefect.cloud/api/accounts/A` | Every workspace in account `A`. |
| `https://api.prefect.cloud` | Any URL on `api.prefect.cloud`. |

The resource's API URL is `remoteApiUrl` with `/api` appended if it's missing,
followed by `/accounts/<accountId>/workspaces/<workspaceId>` when both are set.
For an in-cluster `PrefectServer`, it's
`http://<name>.<namespace>.svc.<cluster-domain>.:4200/api`. The cluster domain
is usually `cluster.local`. The operator logs the one it uses when it starts,
and the [`--cluster-domain` flag](../../reference/operator-configuration/) sets
it.

The operator checks the annotation each time it reads the key. Without a match,
the resource's `Synced` condition is `False` with the reason
`PrefectClientError`, and the message names the annotation.

## How workers get the key

A `PrefectWorkPool` worker reads the key itself, through a `secretKeyRef` or
`configMapKeyRef` in the worker Deployment's `PREFECT_API_KEY` variable. The
operator sets the worker's `PREFECT_API_URL` to the URL it checked, and the
CRD rejects `settings` that set either variable.

If you remove a URL from the annotation later, the operator stops updating the
worker Deployment. Running workers, and workers that restart, still read the
key. To stop a worker from using a key, delete the `PrefectWorkPool` or rotate
the key.

## Rules

- Set `apiKey.value` or `apiKey.valueFrom`, not both. `value` puts the key in
  the resource's spec in plain text, so prefer `valueFrom`.
- The Secret or ConfigMap must be in the resource's namespace.
- With the chart's admission policies, writing a `PrefectWorkQueue`,
  `PrefectDeployment`, or `PrefectAutomation` that names a Secret requires
  `get` on that Secret. See [Permissions](../permissions/).
