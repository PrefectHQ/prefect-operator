---
title: Server storage
weight: 1
---

A `PrefectServer` stores its state in a database. Set at most one of
`spec.ephemeral`, `spec.sqlite`, and `spec.postgres`. With none set, the
server is ephemeral.

| Backend | Database | Survives a pod restart | Replicas | Deployment strategy |
| --- | --- | --- | --- | --- |
| `ephemeral` | SQLite in an `emptyDir` | No | 1 | `RollingUpdate` |
| `sqlite` | SQLite on a PVC | Yes | 1 | `Recreate` |
| `postgres` | An external PostgreSQL | Yes | `spec.replicas`, default 1 | `RollingUpdate` |

## Ephemeral

```yaml
apiVersion: prefect.io/v1
kind: PrefectServer
metadata:
  name: prefect
spec:
  ephemeral: {}
```

Use it for trying things out and for tests. The server creates its schema on
start.

## SQLite

```yaml
apiVersion: prefect.io/v1
kind: PrefectServer
metadata:
  name: prefect
spec:
  sqlite:
    storageClassName: standard
    size: 1Gi
```

The operator creates a `ReadWriteOnce` PVC named `<name>-data` and mounts it
at `/var/lib/prefect/`. The Deployment uses the
`Recreate` strategy, because two pods can't share the volume.

The PVC has an owner reference to the `PrefectServer`. Deleting the
`PrefectServer` deletes the PVC, and the database with it, unless the
StorageClass's reclaim policy is `Retain`.

## PostgreSQL

```yaml
apiVersion: prefect.io/v1
kind: PrefectServer
metadata:
  name: prefect
spec:
  postgres:
    host: postgres.databases.svc
    port: 5432
    database: prefect
    userFrom:
      secretKeyRef:
        name: prefect-db
        key: username
    passwordFrom:
      secretKeyRef:
        name: prefect-db
        key: password
```

Each connection setting (`host`, `port`, `user`, `password`, `database`) has a
`...From` variant that takes an `EnvVarSource`, so any of them can come from a
Secret or ConfigMap. The operator passes them to the server as
`PREFECT_API_DATABASE_*` environment variables.

The operator runs schema migrations in a Job before it rolls out the server:

1. It creates a Job named `<name>-migration-<hash>`, where the hash is of the
   Job's spec. The Job runs `prefect server database upgrade --yes`.
2. It checks the Job every 20 seconds. Until the Job completes, it doesn't
   create or update the server Deployment, and `MigrationJobReconciled` is
   `False`.
3. When the Job completes, it rolls out the Deployment.

Changing the image, the database settings, or `migrationJobTemplate` changes
the hash and runs a new Job. A failed Job holds the rollout, and the condition
message includes the Job's failure message. Kubernetes deletes the Job one
hour after it finishes, through `ttlSecondsAfterFinished`. The next reconcile
then creates it again. The migrations are safe to run again.

Both the Job and the server pods have an init container, `wait-for-database`,
that runs `pg_isready` until the database accepts connections. It uses
`postgres:16-alpine` unless you set `spec.postgres.waitForDatabaseImage`.

## Redis

Set `spec.redis` to use Redis for the server's messaging broker and cache:

```yaml
spec:
  postgres:
    # ...
  redis:
    host: redis.databases.svc
    port: 6379
    passwordFrom:
      secretKeyRef:
        name: prefect-redis
        key: password
```

The Prefect image must include the `prefect-redis` package.

## More than one replica

```yaml
spec:
  replicas: 3
  postgres:
    # ...
  redis:
    host: redis.databases.svc
    port: 6379
    leaseStorage: true
```

The CRD rejects `replicas` above 1 unless `postgres` is set and `sqlite` is not.

Prefect keeps concurrency limit leases in each replica's memory by default. With
more than one replica, each replica enforces limits on its own, so the
cluster-wide limit is not enforced. `leaseStorage: true` stores leases in Redis.
It needs a Prefect image whose `prefect-redis` package includes the
`lease_storage` module. Without it, the server's `SharedStateConfigured`
condition is `False` with the reason `InMemoryLeaseStorage`.
