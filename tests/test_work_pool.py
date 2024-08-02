from prefect_operator.work_pool import PrefectServerReference, PrefectWorkPool


def test_work_pool_uses_context_for_namespace_and_name():
    work_pool = PrefectWorkPool.model_validate(
        {
            "server": {"namespace": "my-app", "name": "my-prefect"},
            "workers": 3,
        },
        context={"name": "my-pool", "namespace": "my-app"},
    )
    assert work_pool.namespace == "my-app"
    assert work_pool.name == "my-pool"
    assert work_pool.server.namespace == "my-app"
    assert work_pool.server.name == "my-prefect"
    assert work_pool.workers == 3


def test_work_pool_instantiated_directly():
    work_pool = PrefectWorkPool(
        name="my-pool",
        namespace="my-app",
        server=PrefectServerReference(namespace="my-app", name="my-prefect"),
        workers=3,
    )
    assert work_pool.namespace == "my-app"
    assert work_pool.name == "my-pool"
    assert work_pool.server.namespace == "my-app"
    assert work_pool.server.name == "my-prefect"
    assert work_pool.workers == 3


def test_work_pool_for_in_namespace_server():
    work_pool = PrefectWorkPool(
        name="my-pool",
        namespace="my-app",
        server=PrefectServerReference(namespace="my-app", name="my-prefect"),
        workers=3,
        version="3.0.0rc42",
    )

    assert work_pool.desired_deployment() == {
        "apiVersion": "apps/v1",
        "kind": "Deployment",
        "metadata": {"namespace": "my-app", "name": "my-pool"},
        "spec": {
            "replicas": 3,
            "selector": {"matchLabels": {"app": "my-pool"}},
            "template": {
                "metadata": {"labels": {"app": "my-pool"}},
                "spec": {
                    "containers": [
                        {
                            "name": "prefect-worker",
                            "image": "prefecthq/prefect:3.0.0rc42-python3.12-kubernetes",
                            "env": [
                                {
                                    "name": "PREFECT_API_URL",
                                    "value": "http://my-prefect.my-app.svc:4200/api",
                                },
                            ],
                            "command": [
                                "bash",
                                "-c",
                                (
                                    "prefect worker start --type kubernetes "
                                    "--pool 'my-app:my-pool' "
                                    '--name "my-app:${HOSTNAME}"'
                                ),
                            ],
                        },
                    ],
                },
            },
        },
    }
