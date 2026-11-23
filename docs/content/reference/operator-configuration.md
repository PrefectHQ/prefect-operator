---
title: Operator configuration
weight: 2
---

## Flags

| Flag | Default | Description |
| --- | --- | --- |
| `--default-resync-interval` | `5m` | How often to re-check each Prefect API resource against Prefect, unless the resource sets `spec.interval`. |
| `--reconcile-timeout` | `2m` | How long one reconcile may run before its context is canceled and it is retried. `0` means no limit. |
| `--cluster-domain` | | The DNS domain of the cluster, which the URLs of in-cluster Prefect servers end with. When empty, the operator reads it from the search domains in `/etc/resolv.conf`, and uses `cluster.local` if they do not include it. |
| `--leader-elect` | `false` | Elect a leader, so that only one replica reconciles. The release manifest and the chart set it. The lease is named `fd9f6399.prefect.io`. |
| `--metrics-bind-address` | `:8080` | The address of the Prometheus metrics endpoint. |
| `--health-probe-bind-address` | `:8081` | The address of `/healthz` and `/readyz`. |
| `--enable-http2` | `false` | Serve the metrics endpoint over HTTP/2. |
| `--zap-log-level` | `info` | `debug`, `info`, `error`, or an integer above 0 for more verbose debug logs. |
| `--zap-encoder` | `json` | `json` or `console`. |

## Environment variables

| Variable | Description |
| --- | --- |
| `WATCH_NAMESPACES` | A comma-separated list of namespaces to watch. When it's unset, the operator watches all namespaces. |

`WATCH_NAMESPACES` limits what the operator reconciles. It doesn't limit its
RBAC, which is a ClusterRole in both the release manifest and the chart.

## Helm values

The chart sets the flags from these values:

| Value | Flag |
| --- | --- |
| `operator.defaultResyncInterval` | `--default-resync-interval` |
| `operator.clusterDomain` | `--cluster-domain` |
| `operator.logLevel` | `--zap-log-level` |
| `operator.logFormat` | `--zap-encoder` |

Set environment variables such as `WATCH_NAMESPACES` with
`operator.extraEnvVars`:

```yaml
operator:
  extraEnvVars:
    - name: WATCH_NAMESPACES
      value: data-eng,analytics
```

The [chart README](https://github.com/PrefectHQ/prefect-operator/blob/main/deploy/charts/prefect-operator/README.md)
lists every value.

## Without Helm

Edit the operator Deployment's `args` and `env` in the release manifest before
applying it, or patch it afterwards:

```sh
kubectl set env deployment/prefect-operator WATCH_NAMESPACES=data-eng,analytics
```
