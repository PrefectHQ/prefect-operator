# Prefect operator for Kubernetes

## Development

After cloning, create and activate a Python virtual environment, the run `make`.
On subsequent pulls, or when changing dependencies (in `requirements.in`),
`make` will bring the environment up to the latest.

Run the development version of the operator on your host with

```shell
kopf run prefect_operator.py
```

The examples refer to a Kubernetes storage class named `standard`, which comes
with `minikube` by default.  You may need to adjust this for your cluster if you
are using a different stack or have changed your storage setup.

## Development and prototyping with `minikube`

First, you should have `minikube`
[installed](https://minikube.sigs.k8s.io/docs/start/) or an equivalent local
Kubernetes cluster installed and configured.

Make sure `minikube` is the cluster you're configured to connect to;

```shell
kubectl config current-context
```

You should see `minikube` as the output.

To deploy the example system on `postgres`:

```shell
./deploy-example postgres
```

This will deploy the manifests in `examples/postgres`, including the namespace
`pop-pg`, a PostgreSQL database server, a `PrefectServer` and a
`PrefectWorkPool` using that server.
