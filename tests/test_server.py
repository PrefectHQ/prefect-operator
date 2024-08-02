import pytest
from prefect_operator.server import (
    PrefectPostgresDatabase,
    PrefectServer,
    PrefectSqliteDatabase,
)


def test_server_uses_context_for_namespace_and_name():
    server = PrefectServer.model_validate(
        {"version": "3.0.0rc42"},
        context={"name": "my-prefect", "namespace": "my-app"},
    )
    assert server.namespace == "my-app"
    assert server.name == "my-prefect"
    assert server.version == "3.0.0rc42"


def test_server_instantiated_directly():
    server = PrefectServer(
        name="my-prefect",
        namespace="my-app",
        version="3.0.0rc42",
    )
    assert server.namespace == "my-app"
    assert server.name == "my-prefect"
    assert server.version == "3.0.0rc42"


def environment_as_dict(container):
    return {e["name"]: e.get("value") or e.get("valueFrom") for e in container["env"]}


@pytest.fixture
def generic_server() -> PrefectServer:
    return PrefectServer(
        name="my-prefect",
        namespace="my-app",
        version="3.0.0rc42",
        settings=[
            {"name": "PREFECT_THIS", "value": "that"},
            {"name": "PREFECT_THAT", "value": "this"},
        ],
    )


def test_service(generic_server: PrefectServer):
    assert generic_server.desired_service() == {
        "apiVersion": "v1",
        "kind": "Service",
        "metadata": {"namespace": "my-app", "name": "my-prefect"},
        "spec": {
            "selector": {"app": "my-prefect"},
            "ports": [{"port": 4200, "protocol": "TCP"}],
        },
    }


def test_cannot_produce_deployment_with_no_database(generic_server: PrefectServer):
    assert generic_server.sqlite is None
    assert generic_server.postgres is None
    with pytest.raises(NotImplementedError, match="database"):
        generic_server.desired_deployment()


@pytest.fixture
def sqlite_server() -> PrefectServer:
    return PrefectServer(
        name="my-prefect",
        namespace="my-app",
        version="3.0.0rc42",
        sqlite=PrefectSqliteDatabase(
            storageClassName="the-fast-stuff",
            size="1Gi",
        ),
        settings=[
            {"name": "PREFECT_THIS", "value": "that"},
            {"name": "PREFECT_THAT", "value": "this"},
        ],
    )


def test_sqlite_server_migrates_itself(sqlite_server: PrefectServer):
    assert sqlite_server.sqlite
    assert not sqlite_server.sqlite.desired_database_migration_job(sqlite_server)

    desired_deployment = sqlite_server.desired_deployment()
    pod_template = desired_deployment["spec"]["template"]
    container = pod_template["spec"]["containers"][0]

    environment = environment_as_dict(container)
    assert environment["PREFECT_API_DATABASE_MIGRATE_ON_START"] == "True"


def test_sqlite_server_forces_recreate(sqlite_server: PrefectServer):
    desired_deployment = sqlite_server.desired_deployment()
    assert desired_deployment["kind"] == "Deployment"
    assert desired_deployment["spec"]["replicas"] == 1
    assert desired_deployment["spec"]["strategy"] == {"type": "Recreate"}


def test_sqlite_server_adds_volume(sqlite_server: PrefectServer):
    assert sqlite_server.sqlite
    desired_pvc = sqlite_server.sqlite.desired_persistent_volume_claim(sqlite_server)

    assert desired_pvc
    assert desired_pvc["kind"] == "PersistentVolumeClaim"
    assert desired_pvc["metadata"]["name"] == "my-prefect-database"
    assert desired_pvc["spec"]["storageClassName"] == "the-fast-stuff"
    assert desired_pvc["spec"]["resources"]["requests"]["storage"] == "1Gi"

    desired_deployment = sqlite_server.desired_deployment()
    pod_template = desired_deployment["spec"]["template"]
    container = pod_template["spec"]["containers"][0]
    assert pod_template["spec"]["volumes"] == [
        {
            "name": "database",
            "persistentVolumeClaim": {"claimName": "my-prefect-database"},
        }
    ]
    assert container["volumeMounts"] == [
        {
            "name": "database",
            "mountPath": "/var/lib/prefect/",
        }
    ]

    environment = environment_as_dict(container)
    assert environment["PREFECT_HOME"] == "/var/lib/prefect/"
    assert (
        environment["PREFECT_API_DATABASE_CONNECTION_URL"]
        == "sqlite+aiosqlite:////var/lib/prefect/prefect.db"
    )


def test_sqlite_server_adds_environment(sqlite_server: PrefectServer):
    desired_deployment = sqlite_server.desired_deployment()

    pod_template = desired_deployment["spec"]["template"]
    container = pod_template["spec"]["containers"][0]

    environment = environment_as_dict(container)
    assert environment["PREFECT_THIS"] == "that"
    assert environment["PREFECT_THAT"] == "this"


@pytest.fixture
def postgres_server() -> PrefectServer:
    return PrefectServer(
        name="my-prefect",
        namespace="my-app",
        version="3.0.0rc42",
        postgres=PrefectPostgresDatabase(
            host="my-postgres",
            port=5432,
            user="my-user",
            passwordSecretKeyRef={
                "name": "my-secrets",
                "key": "my-password",
            },
            database="my-database",
        ),
        settings=[
            {"name": "PREFECT_THIS", "value": "that"},
            {"name": "PREFECT_THAT", "value": "this"},
        ],
    )


def test_postgres_server_uses_migration_job(postgres_server: PrefectServer):
    assert postgres_server.postgres
    assert postgres_server.postgres.desired_database_migration_job(postgres_server) == {
        "apiVersion": "batch/v1",
        "kind": "Job",
        "metadata": {"namespace": "my-app", "name": "my-prefect-migrate"},
        "spec": {
            "template": {
                "spec": {
                    "containers": [
                        {
                            "name": "migrate",
                            "image": "prefecthq/prefect:3.0.0rc42-python3.12",
                            "command": [
                                "prefect",
                                "server",
                                "database",
                                "upgrade",
                                "--yes",
                            ],
                            "env": [
                                {"name": "PREFECT_THIS", "value": "that"},
                                {"name": "PREFECT_THAT", "value": "this"},
                                {
                                    "name": "PREFECT_API_DATABASE_CONNECTION_URL",
                                    "value": "postgresql+asyncpg://my-user:${PREFECT_API_DATABASE_PASSWORD}@my-postgres:5432/my-database",
                                },
                                {
                                    "name": "PREFECT_API_DATABASE_PASSWORD",
                                    "valueFrom": {
                                        "secretKeyRef": {
                                            "name": "my-secrets",
                                            "key": "my-password",
                                        }
                                    },
                                },
                                {
                                    "name": "PREFECT_API_DATABASE_MIGRATE_ON_START",
                                    "value": "False",
                                },
                            ],
                        }
                    ],
                    "restartPolicy": "OnFailure",
                }
            }
        },
    }

    desired_deployment = postgres_server.desired_deployment()
    pod_template = desired_deployment["spec"]["template"]
    container = pod_template["spec"]["containers"][0]

    environment = environment_as_dict(container)
    assert environment["PREFECT_API_DATABASE_MIGRATE_ON_START"] == "False"


def test_postgres_server_uses_default_strategy(postgres_server: PrefectServer):
    desired_deployment = postgres_server.desired_deployment()
    assert desired_deployment["kind"] == "Deployment"
    assert desired_deployment["spec"]["replicas"] == 1
    assert "strategy" not in desired_deployment["spec"]


def test_postgres_server_does_not_add_volume(postgres_server: PrefectServer):
    assert postgres_server.postgres
    assert not postgres_server.postgres.desired_persistent_volume_claim(postgres_server)

    desired_deployment = postgres_server.desired_deployment()
    pod_template = desired_deployment["spec"]["template"]
    container = pod_template["spec"]["containers"][0]
    assert "volumes" not in pod_template["spec"]
    assert "volumeMounts" not in container

    environment = environment_as_dict(container)
    assert environment["PREFECT_HOME"] == "/var/lib/prefect/"
    assert (
        environment["PREFECT_API_DATABASE_CONNECTION_URL"]
        == "postgresql+asyncpg://my-user:${PREFECT_API_DATABASE_PASSWORD}@my-postgres:5432/my-database"
    )
    assert environment["PREFECT_API_DATABASE_PASSWORD"] == {
        "secretKeyRef": {"name": "my-secrets", "key": "my-password"}
    }


def test_postgres_server_adds_environment(postgres_server: PrefectServer):
    desired_deployment = postgres_server.desired_deployment()

    pod_template = desired_deployment["spec"]["template"]
    container = pod_template["spec"]["containers"][0]

    environment = environment_as_dict(container)
    assert environment["PREFECT_THIS"] == "that"
    assert environment["PREFECT_THAT"] == "this"
