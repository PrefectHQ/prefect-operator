---
title: Install
weight: 1
---

Each [GitHub release](https://github.com/PrefectHQ/prefect-operator/releases)
has two manifests:

| File | Contents |
| --- | --- |
| `prefect-operator.yaml` | The CRDs, and the operator's ServiceAccount, RBAC, and Deployment. Releases after v0.20.0 also include the [admission policies](../../guides/permissions/). |
| `prefect-crds.yaml` | The CRDs only. |

## With kubectl

```sh
VERSION=v0.20.0
kubectl apply --server-side \
  -f https://github.com/PrefectHQ/prefect-operator/releases/download/$VERSION/prefect-operator.yaml
```

The CRDs are large, so apply with `--server-side`. A client-side apply stores
the whole object in the `kubectl.kubernetes.io/last-applied-configuration`
annotation, which can exceed the 256 KiB annotation limit.

The manifest installs the operator into the `default` namespace. To read it
before applying it:

```sh
curl -sL https://github.com/PrefectHQ/prefect-operator/releases/download/$VERSION/prefect-operator.yaml \
  | grep -E '^kind:' | sort | uniq -c
```

## With Helm

```sh
helm repo add prefect-operator https://prefecthq.github.io/prefect-operator
helm repo update
helm install prefect-operator prefect-operator/prefect-operator \
  --namespace prefect-system --create-namespace
```

The chart's values are listed in the
[chart README](https://github.com/PrefectHQ/prefect-operator/blob/main/deploy/charts/prefect-operator/README.md),
which also covers verifying the chart's signature.

## Check that it's running

```sh
kubectl get deployment prefect-operator
kubectl get crds | grep prefect.io
```

The second command lists five CRDs: `prefectautomations`, `prefectdeployments`,
`prefectservers`, `prefectworkpools`, and `prefectworkqueues`.

## Requirements

- Kubernetes 1.30 or later for the admission policies. CI tests the chart on
  the versions listed in
  [operator-lint-and-test.yaml](https://github.com/PrefectHQ/prefect-operator/blob/main/.github/workflows/operator-lint-and-test.yaml).
- Without the admission policies, see [Permissions](../../guides/permissions/)
  for what anyone who can write the custom resources can do.

## Uninstall

Delete the custom resources first, so the operator can delete the objects it
created in Prefect. Then delete the operator:

```sh
kubectl delete prefectautomations,prefectdeployments,prefectworkqueues,prefectworkpools --all -A
kubectl delete prefectservers --all -A
kubectl delete -f https://github.com/PrefectHQ/prefect-operator/releases/download/$VERSION/prefect-operator.yaml
```

Delete the `PrefectServer` resources last. The other resources need their
server to delete their objects from it.

If you delete the operator first, the resources keep their finalizers and stay
in `Terminating`. See [Deletion](../../concepts/how-it-works/#deletion).
