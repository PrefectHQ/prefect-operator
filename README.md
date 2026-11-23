# prefect-operator

The Prefect Operator is an application that can manage instances of Prefect.
It follows the [Kubernetes operator pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/).

## Custom Resource Definitions

### PrefectServer

The [PrefectServer CRD](./PrefectServer.md)

### PrefectWorkPool

The [PrefectWorkPool CRD](./PrefectWorkPool.md)

### PrefectWorkQueue

The [PrefectWorkQueue CRD](./PrefectWorkQueue.md)

### PrefectDeployment

The [PrefectDeployment CRD](./PrefectDeployment.md)

### PrefectAutomation

The [PrefectAutomation CRD](./PrefectAutomation.md)

## Pod template overrides

Most pod settings, such as resources, node selectors, tolerations and extra
containers, have their own fields. For anything else, you can patch the pods
the operator builds with a pod template:

| Resource          | Field                  | Pods                         | Container names                                |
|-------------------|------------------------|------------------------------|------------------------------------------------|
| `PrefectServer`   | `deploymentTemplate`   | the server Deployment        | `prefect-server`, `wait-for-database` (init)   |
| `PrefectServer`   | `migrationJobTemplate` | the PostgreSQL migration Job | `prefect-server-migration`, `wait-for-database` (init) |
| `PrefectWorkPool` | `deploymentTemplate`   | the worker Deployment        | `prefect-worker`                               |

A template is a partial pod template, with `metadata` and `spec` like a
Deployment's `spec.template`, applied as a
[strategic merge patch](https://kubernetes.io/docs/tasks/manage-kubernetes-objects/update-api-object-kubectl-patch/#use-a-strategic-merge-patch-to-update-a-deployment).
Lists of containers are matched by name, so you can change one setting of a
container without restating the rest of it. The container names above are
stable. Patch directives such as `$patch: delete` work too.

A patch merges into what the operator built, so a probe you patch keeps the
operator's handler. To swap the handler, for example from `httpGet` to
`tcpSocket`, replace the whole probe with `$patch: replace`. Setting
`httpGet: null` instead only works with `kubectl apply --server-side`, because
a plain `kubectl apply` drops the `null`.

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

Lists of strings such as `args` and `command` are replaced, not merged. On
`prefect-worker`, overriding `args` replaces the whole list, including
`--pool <name>`, so include every argument the worker needs.

The operator applies the other fields first and the template last, so the
template wins. The exception is the fields the operator depends on, which it
puts back if a template overrides them:

- the pod labels the Deployment selects on
- the `api` port of the `prefect-server` container
- the `prefect-data` volume and its mounts
- the environment variables the operator derives from the resource, such as
  the database settings and the worker's `PREFECT_API_URL`. Variables from
  `settings` are not protected.

The resource's `DeploymentTemplateApplied` (or `MigrationJobTemplateApplied`)
condition describes the desired pod template after rendering, before Kubernetes
writes or migration completion; it does not confirm that running pods use it.
Its reason is `ProtectedFieldsIgnored`,
and its message lists the fields that kept the operator's values, such as
`spec.containers[prefect-worker].env[PREFECT_API_URL]`. The operator also emits
a Warning event when that list changes. Otherwise the reason is
`TemplateApplied`. The condition is removed when there is no template or template
rendering fails. On a PrefectServer, a rendering failure clears both template
conditions because the combined render did not complete. Check `DeploymentReconciled`
and `MigrationJobReconciled` for reconciliation progress and errors.

For example, to run the server as a non-root user and give it more time before
the first readiness check:

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

Pod template labels and annotations that other tools add to a Deployment, such
as the annotation `kubectl rollout restart` sets, are left alone. The operator
records which ones it set in the Deployment's `prefect.io/pod-template-labels`
and `prefect.io/pod-template-annotations` annotations, so that it only removes
those when they're deleted from the template.

The migration Job's name comes from a hash of its spec, so changing
`migrationJobTemplate` runs the migrations again. They are safe to run again.

The template is stored as is, without a schema, so a misspelled field is only
caught when the operator applies it. It then sets the resource's
`DeploymentReconciled` (or `MigrationJobReconciled`) condition to `False` with
the reason `InvalidPodTemplate`, emits a Warning event, and leaves the running
pods as they are. It tries again when the template changes. A template that
adds a container with the name of an existing one is rejected the same way.

## API keys from Secrets and ConfigMaps

A resource can read its Prefect API key from a Secret or ConfigMap with
`spec.server.apiKey.valueFrom`. The Secret or ConfigMap must have the annotation
`prefect.io/allowed-api-urls`, a comma-separated list of the Prefect API URLs
that the key may be used for. The operator refuses a key without the annotation,
or for a server whose API URL doesn't match an entry:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: prefect-api-key
  annotations:
    prefect.io/allowed-api-urls: https://api.prefect.cloud/api/accounts/<account-id>/workspaces/<workspace-id>
stringData:
  api-key: pnu_...
```

An entry matches when its scheme and host, including any port, equal the API
URL's, and its path is the API URL's path or a parent of it. For a remote server,
the API URL is `remoteApiUrl` with `/api` appended if it's missing, followed by
`/accounts/<accountId>/workspaces/<workspaceId>` when both are set. For an
in-cluster server, it's `http://<name>.<namespace>.svc:4200/api`. The resource's
`Synced` condition names the annotation when the operator refuses a key.

Set either `apiKey.value` or `apiKey.valueFrom`, not both. The operator checks
the annotation each time it reads the key. A `PrefectWorkPool` worker reads the
key itself, through a `secretKeyRef` or `configMapKeyRef` in its Deployment. If
you remove a URL from the annotation later, the operator stops updating the
worker Deployment, but running and restarted workers still read the key. To
stop a worker from using the key, delete the `PrefectWorkPool` or rotate the
key.

## Permissions for writing Prefect resources

The operator uses its own service account to create Deployments and read API
keys. The chart installs two ValidatingAdmissionPolicies, so writing a Prefect
resource also requires the RBAC for what the operator then does with it:

| Resource | The requester also needs |
| --- | --- |
| `PrefectServer`, `PrefectWorkPool` | `create` on `deployments` in the resource's namespace. The operator builds a Deployment from the spec, including `extraContainers`, `settings`, `serviceAccountName` and `deploymentTemplate`. |
| `PrefectWorkQueue`, `PrefectDeployment`, `PrefectAutomation` | `get` on the Secret or ConfigMap that `spec.server.apiKey.valueFrom` names. The operator sends the spec to Prefect with that key. |

The chart's editor roles grant only the Prefect resources, so grant these
permissions alongside them. The policies don't check a request that leaves the
spec unchanged, such as the operator adding its finalizer. They check new requests only, so resources that already exist
aren't checked until someone changes them.

The policies need Kubernetes 1.30 or later, and `admissionPolicy.enabled`
controls whether the chart installs them. Without them, anyone who can write a
`PrefectServer` or `PrefectWorkPool` can run any container, with any service
account, in its namespace. The `prefect.io/allowed-api-urls` annotation applies
either way.

A `PrefectDeployment` runs flow code on the work pool's workers. Its
`jobVariables` fill in the work pool's base job template, so the template
decides which job settings, such as the image or service account, a deployment
can change.

## Installing

Install the operator with its Helm chart:

```sh
helm repo add prefect-operator https://prefecthq.github.io/prefect-operator
helm repo update
helm install prefect-operator prefect-operator/prefect-operator --namespace prefect-system --create-namespace
```

The [chart's README](deploy/charts/prefect-operator/README.md) covers verifying the chart's signature, the chart's values, and uninstalling.

## Development

### Prerequisites

- [mise](https://mise.jdx.dev) (`curl https://mise.run | sh`). It installs Go, kubectl, Helm, and the other tools pinned in [.mise.toml](.mise.toml):

  ```sh
  mise trust
  mise install
  ```

  The `make` targets run `mise install` for you. Python tools such as `yamllint` also need `pipx` (`pip install --user pipx`); see [jdx/mise#2536](https://github.com/jdx/mise/issues/2536).
- Docker, to build the operator image.
- A Kubernetes cluster, such as minikube. CI installs the chart on the Kubernetes versions listed in [operator-lint-and-test.yaml](.github/workflows/operator-lint-and-test.yaml).

### Run the operator from your machine

Install the CRDs into the cluster in your current kube context, and then run the operator against that cluster:

```sh
make install run
```

Installing CRDs needs cluster-wide permissions. If `make install` fails with RBAC errors, use an account with cluster-admin.

The operator reaches in-cluster Prefect servers through `kubectl port-forward`. It uses the kube context that was current when it started, so switching contexts afterwards doesn't move it to another cluster. Restart it to pick up a new context.

In another shell, apply a sample resource from [deploy/samples](deploy/samples/):

```sh
kubectl apply -f deploy/samples/v1_prefectserver_ephemeral.yaml
```

### Deploy your build to a cluster

Build the operator image and push it to a registry that your cluster can pull from:

```sh
make docker-build docker-push IMG=<some-registry>/prefect-operator:<tag>
```

`make deploy` installs the chart with the image set in its values.yaml, and ignores `IMG`. To deploy your image, build the chart's dependencies and pass the image to Helm:

```sh
make helmbuild
helm install prefect-operator deploy/charts/prefect-operator \
  --namespace prefect-system --create-namespace \
  --set operator.image.repository=<some-registry>/prefect-operator \
  --set operator.image.tag=<tag>
```

### Uninstall

Delete the resources you created, then the operator, then the CRDs:

```sh
kubectl delete -f deploy/samples/v1_prefectserver_ephemeral.yaml
make undeploy
make uninstall
```

## Contributing
**NOTE:** Run `make help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)

## License

The Prefect Operator is licensed under the Apache License, Version 2.0. See [LICENSE](LICENSE).
