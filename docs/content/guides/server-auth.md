---
title: Require authentication
weight: 2
---

A `PrefectServer` accepts API requests from anything that can reach it, unless
you set `spec.auth`. With it, the server requires an auth string,
`username:password`, as HTTP basic authentication on every request except its
health checks. Each resource that uses the server names a Secret with the same
auth string.

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: prefect-auth
  annotations:
    prefect.io/allowed-api-urls: http://prefect.data-eng.svc.cluster.local:4200
stringData:
  auth-string: admin:<password>
---
apiVersion: prefect.io/v1
kind: PrefectServer
metadata:
  name: prefect
  namespace: data-eng
spec:
  auth:
    secretKeyRef:
      name: prefect-auth
      key: auth-string
---
apiVersion: prefect.io/v1
kind: PrefectWorkPool
metadata:
  name: process-pool
  namespace: data-eng
spec:
  type: process
  workers: 1
  server:
    name: prefect
    auth:
      secretKeyRef:
        name: prefect-auth
        key: auth-string
```

The operator sets `PREFECT_SERVER_API_AUTH_STRING` on the server from the
Secret, and `PREFECT_API_AUTH_STRING` on each work pool's worker. A resource
without `spec.server.auth` gets `Synced` `False` with the message
`API request failed with status 401`.

## When to turn it on

If you isolate namespaces with NetworkPolicy, set `spec.auth` on every server.
Otherwise the operator can reach the server on behalf of a resource in any
namespace it watches. A resource can name the server in `spec.server.name` and
`spec.server.namespace`, or with its Service URL in `remoteApiUrl`.

Without NetworkPolicy, any pod in the cluster can already call a server that
has no auth.

## Turn it on for a running server

A server without auth ignores the auth string, so set it on the clients first:

1. Create the Secret in the namespace of the server. Create it in the namespace
   of each resource that uses the server too, with the
   `prefect.io/allowed-api-urls` annotation, and in the namespace that the flow
   run Jobs of `kubernetes` work pools run in.
2. Set `spec.server.auth` on each of those resources. The operator replaces the
   workers, which then send the auth string. On `process` work pools, that
   interrupts the flow runs in progress. Set the auth string on other clients
   too, such as workers you run yourself, scripts, and CI jobs.
3. Wait for flow runs that started before step 2 to finish. They don't have the
   auth string, and their API requests get a 401 after the next step.
4. Set `spec.auth` on the server. Check that `AuthSupported` is `True`, and that
   the new server pod is ready.

## Prefect version

`spec.auth` requires Prefect 3.6.22 or later. Earlier servers ignore the auth
string, or don't check it on every request. When the image is a
`prefecthq/prefect` release before 3.6.22, the operator doesn't change the
server's Deployment, and sets `DeploymentReconciled` to `False` with the reason
`UnsupportedVersion`. A server that already runs keeps running, without auth.

The operator can't tell the version of other images, of tags such as
`3-latest` and `3.6-python3.12`, or of an image with a digest. It deploys them,
and sets the server's `AuthSupported` condition to `Unknown`. The condition is
absent while the server's version is supported or unknown and its Secret or
`settings` are invalid. To get the version of a running server:

```sh
kubectl -n data-eng exec deployment/prefect -- prefect --version
```

## Flow runs in Kubernetes Jobs

On a work pool of type `kubernetes`, the operator names the auth Secret in the
worker settings
`PREFECT_INTEGRATIONS_KUBERNETES_WORKER_API_AUTH_STRING_SECRET_NAME` and
`..._SECRET_KEY`. Each Job then reads `PREFECT_API_AUTH_STRING` from the
Secret. This requires `prefect-kubernetes` 0.6.3 or later in the worker's
image, which the Prefect 3.4.10 and later images have. An earlier worker puts
the auth string in the Job's manifest.

The Secret must exist in the namespace the Jobs run in, or their pods don't
start. Prefect's default base job template runs Jobs in the `default`
namespace. Set `namespace` in the work pool's base job template to the work
pool's namespace, or copy the Secret to the namespace you use.

To pass the auth string in the Job's manifest as plain text instead, set the
first setting to an empty string. Anyone who can view the Jobs or pods can then
read it:

```yaml
spec:
  settings:
    - name: PREFECT_INTEGRATIONS_KUBERNETES_WORKER_API_AUTH_STRING_SECRET_NAME
      value: ""
```

Flow code can read the auth string either way.

## Flow runs in other work pools

The operator sets no such settings for other types of work pool. A worker
passes the auth string to the flow runs it starts in its own way. For example,
a Docker worker puts it in the container's environment, where anyone who can
inspect the container can read it. Before you use `spec.server.auth` with
another type, see Prefect's documentation for its worker.

## Use a server from another namespace

A resource reads the auth string from a Secret in its own namespace. To let
resources in another namespace use the server, copy the Secret there. Copy the
`prefect.io/allowed-api-urls` annotation too, which must list the server's
in-cluster URL:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: prefect-auth
  namespace: ml
  annotations:
    prefect.io/allowed-api-urls: http://prefect.data-eng.svc.cluster.local:4200
stringData:
  auth-string: admin:<password>
```

Resources in `ml` then set `spec.server.namespace: data-eng` and
`spec.server.auth` as above.

## Rules

- The operator sends the auth string only to an API URL that the Secret's
  `prefect.io/allowed-api-urls` annotation lists, and doesn't follow redirects.
  See [the annotation](../prefect-cloud/#the-prefectioallowed-api-urls-annotation).
- `secretKeyRef.optional` must not be `true`.
- The auth string must be ASCII, have a username and a password, and not start
  or end with whitespace, such as a newline. Otherwise, or without the Secret,
  the operator doesn't change the server's Deployment, sets
  `DeploymentReconciled` to `False` with the reason `InvalidAuth`, and retries.
- The server's `settings` must not set `PREFECT_SERVER_API_AUTH_STRING`. To move
  an auth string from `settings` to `spec.auth`, remove the setting in the same
  change. Prefect also reads the name in another case, such as
  `prefect_server_api_auth_string`. If `settings` sets that, the operator sets
  the same `InvalidAuth` condition until you remove it.
- The operator doesn't watch the Secret. If you empty the key later, the server
  runs with an empty auth string from its next restart.
- Set `spec.server.apiKey` or `spec.server.auth`, not both.
- The operator sends the auth string over plain HTTP to an in-cluster server.
  If your pod network isn't encrypted, for example by a service mesh with
  mutual TLS, other workloads on the network path can read it.
- With the chart's admission policies, writing a `PrefectWorkQueue`,
  `PrefectDeployment`, or `PrefectAutomation` that names an auth Secret
  requires `get` on that Secret. See [Permissions](../permissions/).
- To change the auth string, update every copy of the Secret. The server and
  workers read it when their pods start, so restart them:
  `kubectl rollout restart deployment/prefect deployment/process-pool`.
