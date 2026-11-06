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
