import time
from typing import Any, ClassVar, Optional

import kopf
import kubernetes
from pydantic import BaseModel, Field

from . import DEFAULT_PREFECT_VERSION
from .resources import CustomResource, NamedResource


class PrefectSqliteDatabase(BaseModel):
    storageClassName: str
    size: str

    @property
    def is_file_based(self) -> bool:
        return True

    def desired_persistent_volume_claim(
        self, server: "PrefectServer"
    ) -> dict[str, Any] | None:
        return {
            "apiVersion": "v1",
            "kind": "PersistentVolumeClaim",
            "metadata": {
                "namespace": server.namespace,
                "name": f"{server.name}-database",
            },
            "spec": {
                "storageClassName": self.storageClassName,
                "accessModes": ["ReadWriteOnce"],
                "resources": {"requests": {"storage": self.size}},
            },
        }

    def configure_prefect_server(
        self,
        server: "PrefectServer",
        prefect_server_workload_spec: dict[str, Any],
        prefect_server_container: dict[str, Any],
    ) -> None:
        prefect_server_workload_spec["replicas"] = 1
        prefect_server_workload_spec["strategy"] = {"type": "Recreate"}

        prefect_server_container["env"].extend(
            [
                {
                    "name": "PREFECT_API_DATABASE_MIGRATE_ON_START",
                    "value": "true",
                },
                {
                    "name": "PREFECT_API_DATABASE_CONNECTION_URL",
                    "value": "sqlite+aiosqlite:////var/lib/prefect/prefect.db",
                },
            ]
        )
        prefect_server_container["volumeMounts"] = [
            {
                "name": "database",
                "mountPath": "/var/lib/prefect/",
            }
        ]
        prefect_server_workload_spec["template"]["spec"]["volumes"] = [
            {
                "name": "database",
                "persistentVolumeClaim": {"claimName": f"{server.name}-database"},
            }
        ]

    def desired_database_migration_job(
        self, server: "PrefectServer"
    ) -> dict[str, Any] | None:
        return None


class SecretKeyReference(BaseModel):
    name: str
    key: str


class PrefectPostgresDatabase(BaseModel):
    host: str
    port: int
    user: str
    passwordSecretKeyRef: SecretKeyReference
    database: str

    @property
    def is_file_based(self) -> bool:
        return False

    def desired_persistent_volume_claim(
        self, server: "PrefectServer"
    ) -> dict[str, Any] | None:
        return None

    def configure_prefect_server(
        self,
        server: "PrefectServer",
        prefect_server_workload_spec: dict[str, Any],
        prefect_server_container: dict[str, Any],
    ) -> None:
        prefect_server_container["env"].extend(
            [
                {
                    "name": "PREFECT_API_DATABASE_CONNECTION_URL",
                    "value": (
                        "postgresql+asyncpg://"
                        f"{ self.user }:${{PREFECT_API_DATABASE_PASSWORD}}"
                        "@"
                        f"{ self.host }:{ self.port }"
                        "/"
                        f"{self.database}"
                    ),
                },
                {
                    "name": "PREFECT_API_DATABASE_PASSWORD",
                    "valueFrom": {
                        "secretKeyRef": {
                            "name": self.passwordSecretKeyRef.name,
                            "key": self.passwordSecretKeyRef.key,
                        }
                    },
                },
                {
                    "name": "PREFECT_API_DATABASE_MIGRATE_ON_START",
                    "value": "false",
                },
            ]
        )

    def desired_database_migration_job(self, server: "PrefectServer") -> dict[str, Any]:
        return {
            "apiVersion": "batch/v1",
            "kind": "Job",
            "metadata": {
                "namespace": server.namespace,
                "name": f"{server.name}-migrate",
            },
            "spec": {
                "template": {
                    "metadata": {"labels": {"app": server.name}},
                    "spec": {
                        "containers": [
                            {
                                "name": "migrate",
                                "image": f"prefecthq/prefect:{server.version}-python3.12",
                                "env": [
                                    s.as_environment_variable() for s in server.settings
                                ],
                                "command": [
                                    "prefect",
                                    "server",
                                    "database",
                                    "upgrade",
                                    "--yes",
                                ],
                            }
                        ],
                        "restartPolicy": "OnFailure",
                    },
                },
            },
        }


class PrefectSetting(BaseModel):
    name: str
    value: str

    def as_environment_variable(self) -> dict[str, str]:
        return {"name": self.name, "value": self.value}


class PrefectServer(CustomResource, NamedResource):
    kind: ClassVar[str] = "PrefectServer"
    plural: ClassVar[str] = "prefectservers"
    singular: ClassVar[str] = "prefectserver"

    version: str = Field(DEFAULT_PREFECT_VERSION)
    sqlite: Optional[PrefectSqliteDatabase] = Field(None)
    postgres: Optional[PrefectPostgresDatabase] = Field(None)
    settings: list[PrefectSetting] = Field([])

    def desired_deployment(self) -> dict[str, Any]:
        container_template = {
            "name": "prefect-server",
            "image": f"prefecthq/prefect:{self.version}-python3.12",
            "env": [
                {
                    "name": "PREFECT_HOME",
                    "value": "/var/lib/prefect/",
                },
                *[s.as_environment_variable() for s in self.settings],
            ],
            "command": ["prefect", "server", "start", "--host", "0.0.0.0"],
            "ports": [{"containerPort": 4200}],
            "readinessProbe": {
                "httpGet": {"path": "/api/health", "port": 4200, "scheme": "HTTP"},
                "initialDelaySeconds": 10,
                "periodSeconds": 5,
                "timeoutSeconds": 5,
                "successThreshold": 1,
                "failureThreshold": 30,
            },
            "livenessProbe": {
                "httpGet": {"path": "/api/health", "port": 4200, "scheme": "HTTP"},
                "initialDelaySeconds": 120,
                "periodSeconds": 10,
                "timeoutSeconds": 5,
                "successThreshold": 1,
                "failureThreshold": 2,
            },
        }

        pod_template: dict[str, Any] = {
            "metadata": {"labels": {"app": self.name}},
            "spec": {
                "containers": [container_template],
            },
        }

        deployment_spec = {
            "replicas": 1,
            "selector": {"matchLabels": {"app": self.name}},
            "template": pod_template,
        }

        database = self.postgres or self.sqlite
        if not database:
            raise NotImplementedError("No database defined")

        database.configure_prefect_server(self, deployment_spec, container_template)

        return {
            "apiVersion": "apps/v1",
            "kind": "Deployment",
            "metadata": {"namespace": self.namespace, "name": self.name},
            "spec": deployment_spec,
        }

    def desired_service(self) -> dict[str, Any]:
        return {
            "apiVersion": "v1",
            "kind": "Service",
            "metadata": {"namespace": self.namespace, "name": self.name},
            "spec": {
                "selector": {"app": self.name},
                "ports": [{"port": 4200, "protocol": "TCP"}],
            },
        }


@kopf.on.resume("prefect.io", "v3", "prefectserver")
@kopf.on.create("prefect.io", "v3", "prefectserver")
@kopf.on.update("prefect.io", "v3", "prefectserver")
def reconcile_server(
    namespace: str, name: str, spec: dict[str, Any], logger: kopf.Logger, **_
):
    server = PrefectServer.model_validate(
        spec, context={"name": name, "namespace": namespace}
    )
    print(repr(server))

    database = server.postgres or server.sqlite
    if database:
        api = kubernetes.client.BatchV1Api()
        desired_database_migration = database.desired_database_migration_job(server)
        if desired_database_migration:
            try:
                api.delete_namespaced_job(
                    name=desired_database_migration["metadata"]["name"],
                    namespace=namespace,
                )
            except kubernetes.client.ApiException as e:
                if e.status not in (404, 409):
                    raise

            while True:
                try:
                    api.create_namespaced_job(
                        server.namespace, desired_database_migration
                    )
                    break
                except kubernetes.client.ApiException as e:
                    if e.status == 409:
                        time.sleep(1)
                        continue
                    raise

    desired_persistent_volume_claim = database.desired_persistent_volume_claim(server)
    if desired_persistent_volume_claim:
        api = kubernetes.client.CoreV1Api()
        try:
            api.create_namespaced_persistent_volume_claim(
                server.namespace,
                desired_persistent_volume_claim,
            )
            logger.info("Created persistent volume claim %s", name)
        except kubernetes.client.ApiException as e:
            if e.status != 409:
                raise

    api = kubernetes.client.AppsV1Api()
    desired_deployment = server.desired_deployment()
    try:
        api.create_namespaced_deployment(server.namespace, desired_deployment)
        logger.info("Created deployment %s", name)
    except kubernetes.client.ApiException as e:
        if e.status != 409:
            raise

        api.replace_namespaced_deployment(
            desired_deployment["metadata"]["name"],
            server.namespace,
            desired_deployment,
        )
        logger.info("Updated deployment %s", name)

    desired_service = server.desired_service()
    api = kubernetes.client.CoreV1Api()
    try:
        api.create_namespaced_service(
            server.namespace,
            desired_service,
        )
        logger.info("Created service %s", name)
    except kubernetes.client.ApiException as e:
        if e.status != 409:
            raise

        api.replace_namespaced_service(
            desired_service["metadata"]["name"],
            server.namespace,
            desired_service,
        )
        logger.info("Updated service %s", name)


@kopf.on.delete("prefect.io", "v3", "prefectserver")
def delete_server(
    namespace: str, name: str, spec: dict[str, Any], logger: kopf.Logger, **_
):
    server = PrefectServer.model_validate(
        spec, context={"name": name, "namespace": namespace}
    )
    print(repr(server))

    api = kubernetes.client.BatchV1Api()
    try:
        api.delete_namespaced_job(
            name=f"{server.name}-migrate",
            namespace=namespace,
        )
    except kubernetes.client.ApiException as e:
        if e.status not in (404, 409):
            raise

    api = kubernetes.client.AppsV1Api()
    try:
        api.delete_namespaced_deployment(name, namespace)
        logger.info("Deleted deployment %s", name)
    except kubernetes.client.ApiException as e:
        if e.status == 404:
            logger.info("Deployment %s not found", name)
        else:
            raise

    api = kubernetes.client.CoreV1Api()
    try:
        api.delete_namespaced_service(name, namespace)
        logger.info("Deleted service %s", name)
    except kubernetes.client.ApiException as e:
        if e.status == 404:
            logger.info("Service %s not found", name)
        else:
            raise
