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

A template is a partial `PodTemplateSpec` (`metadata` and `spec`), applied as a
[strategic merge patch](https://kubernetes.io/docs/tasks/manage-kubernetes-objects/update-api-object-kubectl-patch/#use-a-strategic-merge-patch-to-update-a-deployment).
Lists of containers are matched by name, so you can change one setting of a
container without restating the rest of it. The container names above are
stable. Patch directives such as `$patch: delete` work too.

The operator applies the other fields first and the template last, so the
template wins. The exception is the fields the operator depends on, which it
puts back if a template overrides them:

- the pod labels the Deployment selects on
- the `api` port of the `prefect-server` container
- the `prefect-data` volume and its mounts
- the environment variables the operator derives from the resource, such as
  the database settings and the worker's `PREFECT_API_URL`. Variables from
  `settings` are not protected.

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

Pod template annotations that other tools add to a Deployment, such as the one
`kubectl rollout restart` sets, are left alone. The operator records which
annotations it set in the Deployment's `prefect.io/pod-template-annotations`
annotation, so that it only removes those when they're deleted from the
template.

The migration Job's name comes from a hash of its spec, so changing
`migrationJobTemplate` runs the migrations again. They are safe to run again.

The template is stored as is, without a schema, so a misspelled field is only
caught when the operator applies it. It then sets the resource's
`DeploymentReconciled` (or `MigrationJobReconciled`) condition to `False` with
the reason `InvalidPodTemplate`, emits a Warning event, and leaves the running
pods as they are.

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
