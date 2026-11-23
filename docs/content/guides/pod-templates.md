---
title: Pod templates
weight: 3
---

Most pod settings, such as resources, node selectors, tolerations, and extra
containers, have their own fields. For anything else, patch the pods the
operator builds with a pod template. This one runs the server as a non-root
user and gives it more time before the first readiness check:

```yaml
apiVersion: prefect.io/v1
kind: PrefectServer
metadata:
  name: prefect
spec:
  postgres:
    # ...
  deploymentTemplate:
    spec:
      securityContext:
        runAsNonRoot: true
        runAsUser: 1001
        fsGroup: 1001
      containers:
        - name: prefect-server
          readinessProbe:
            initialDelaySeconds: 30
  migrationJobTemplate:
    spec:
      securityContext:
        runAsNonRoot: true
        runAsUser: 1001
```

| Resource | Field | Pods | Container names |
| --- | --- | --- | --- |
| `PrefectServer` | `deploymentTemplate` | The server Deployment | `prefect-server`, `wait-for-database` (init) |
| `PrefectServer` | `migrationJobTemplate` | The PostgreSQL migration Job | `prefect-server-migration`, `wait-for-database` (init) |
| `PrefectWorkPool` | `deploymentTemplate` | The worker Deployment | `prefect-worker` |

## How the patch applies

A template is a partial pod template, with `metadata` and `spec` like a
Deployment's `spec.template`. The operator applies it as a
[strategic merge patch](https://kubernetes.io/docs/tasks/manage-kubernetes-objects/update-api-object-kubectl-patch/#use-a-strategic-merge-patch-to-update-a-deployment).
Lists of containers are matched by name, so you can change one setting of a
container without restating the rest of it. The container names above are
stable. Patch directives such as `$patch: delete` work too.

A patch merges into what the operator built, so a probe you patch keeps the
operator's handler. To swap the handler, for example from `httpGet` to
`tcpSocket`, replace the whole probe with `$patch: replace`:

```yaml
deploymentTemplate:
  spec:
    containers:
      - name: prefect-server
        readinessProbe:
          $patch: replace
          tcpSocket:
            port: 4200
          periodSeconds: 10
```

Setting `httpGet: null` instead works only with `kubectl apply --server-side`,
because a client-side `kubectl apply` drops the `null`.

Lists of strings, such as `args` and `command`, are replaced, not merged. On
`prefect-worker`, overriding `args` replaces the whole list, including
`--pool <name>`, so include every argument the worker needs.

## Fields the operator keeps

The operator applies the other fields first and the template last, so the
template wins. The exception is the fields the operator depends on. It puts
these back if a template overrides them:

- the pod labels the Deployment selects on
- the `api` port of the `prefect-server` container
- the `prefect-data` volume and its mounts
- the environment variables the operator derives from the resource, such as
  the database settings and the worker's `PREFECT_API_URL`. Variables from
  `settings` are not protected.

When it puts a field back, the resource's `DeploymentTemplateApplied` (or
`MigrationJobTemplateApplied`) condition has the reason `ProtectedFieldsIgnored`.
Its message lists the fields that kept the operator's values, such as
`spec.containers[prefect-worker].env[PREFECT_API_URL]`. The operator also
records a Warning event when that list changes. Otherwise the reason is
`TemplateApplied`.

The condition describes the pod template the operator rendered. It doesn't
confirm that running pods use it. Check `DeploymentReconciled` and
`MigrationJobReconciled` for reconciliation progress and errors. The condition
is removed when there is no template, or when rendering fails. On a
`PrefectServer`, a rendering failure removes both template conditions.

## Labels and annotations from other tools

The operator leaves alone pod template labels and annotations that other tools
add to a Deployment, such as the annotation that `kubectl rollout restart`
sets. It records the ones it set in the Deployment's
`prefect.io/pod-template-labels` and `prefect.io/pod-template-annotations`
annotations, and removes only those when they're deleted from the template.

## Invalid templates

The template is stored without a schema, so the API server doesn't catch a
misspelled field. The operator catches it when it applies the template. It
then sets `DeploymentReconciled` (or `MigrationJobReconciled`) to `False` with
the reason `InvalidPodTemplate`, records a Warning event, and leaves the
running pods as they are. It tries again when the template changes. A template
that adds a container with the name of an existing one is rejected the same
way.

The migration Job's name comes from a hash of its spec, so changing
`migrationJobTemplate` runs the migrations again.
