---
title: API reference
weight: 1
---

All five kinds are in the API group `prefect.io`, version `v1`, and are
namespaced.

| Kind | Plural | Short name | Field reference |
| --- | --- | --- | --- |
| `PrefectServer` | `prefectservers` | `ps` | [PrefectServer.md](https://github.com/PrefectHQ/prefect-operator/blob/main/PrefectServer.md) |
| `PrefectWorkPool` | `prefectworkpools` | `pwp` | [PrefectWorkPool.md](https://github.com/PrefectHQ/prefect-operator/blob/main/PrefectWorkPool.md) |
| `PrefectWorkQueue` | `prefectworkqueues` | | [PrefectWorkQueue.md](https://github.com/PrefectHQ/prefect-operator/blob/main/PrefectWorkQueue.md) |
| `PrefectDeployment` | `prefectdeployments` | | [PrefectDeployment.md](https://github.com/PrefectHQ/prefect-operator/blob/main/PrefectDeployment.md) |
| `PrefectAutomation` | `prefectautomations` | | [PrefectAutomation.md](https://github.com/PrefectHQ/prefect-operator/blob/main/PrefectAutomation.md) |

The field descriptions come from the CRD schemas, so `kubectl explain` shows the
same text for the version installed in your cluster:

```sh
kubectl explain prefectworkpool.spec
kubectl explain prefectserver.spec.postgres --recursive
```

## Printer columns

`kubectl get` shows these columns:

| Kind | Columns |
| --- | --- |
| `PrefectServer` | `VERSION`, `READY` |
| `PrefectWorkPool` | `TYPE`, `VERSION`, `READY`, `DESIRED WORKERS`, `READY WORKERS` |
