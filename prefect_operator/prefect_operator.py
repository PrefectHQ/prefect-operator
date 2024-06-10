from contextlib import contextmanager
from typing import Any, Generator, Optional, Self

import httpx
import kopf
import kubernetes
from pydantic import BaseModel, Field, PrivateAttr, ValidationInfo, model_validator


class NamedResource(BaseModel):
    _name: str = PrivateAttr()

    @property
    def name(self) -> str:
        return self._name

    _namespace: str = PrivateAttr()

    @property
    def namespace(self) -> str:
        return self._namespace

    @model_validator(mode="after")
    def set_name_and_namespace(self, validation_info: ValidationInfo) -> Self:
        self._name = validation_info.context["name"]
        self._namespace = validation_info.context["namespace"]
        return self


class PrefectSqliteDatabase(BaseModel):
    storageClassName: str
    size: str

    def configure_prefect_server(
        self,
        prefect_server_stateful_set: dict[str, Any],
        prefect_server_container: dict[str, Any],
    ) -> None:
        prefect_server_container["volumeMounts"] = [
            {
                "name": "database",
                "mountPath": "/var/lib/prefect/",
            }
        ]
        prefect_server_stateful_set["volumeClaimTemplates"] = [
            {
                "metadata": {"name": "database"},
                "spec": {
                    "accessModes": ["ReadWriteOnce"],
                    "storageClassName": self.storageClassName,
                    "resources": {"requests": {"storage": self.size}},
                },
            }
        ]


class SecretKeyReference(BaseModel):
    name: str
    key: str


class PrefectPostgresDatabase(BaseModel):
    host: str
    port: int
    user: str
    passwordSecretKeyRef: SecretKeyReference
    database: str

    def configure_prefect_server(
        self,
        prefect_server_stateful_set: dict[str, Any],
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
            ]
        )


class PrefectSetting(BaseModel):
    name: str
    value: str

    def as_environment_variable(self) -> dict[str, str]:
        return {"name": self.name, "value": self.value}


class PrefectServer(NamedResource):
    sqlite: Optional[PrefectSqliteDatabase] = Field(None)
    postgres: Optional[PrefectPostgresDatabase] = Field(None)
    settings: list[PrefectSetting] = Field([])

    def desired_stateful_set(self) -> dict[str, Any]:
        container_template = {
            "name": "prefect-server",
            "image": "prefecthq/prefect:3.0.0rc2-python3.12",
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

        stateful_set_spec = {
            "replicas": 1,
            "selector": {"matchLabels": {"app": self.name}},
            "template": pod_template,
        }

        database = self.postgres or self.sqlite
        if not database:
            raise NotImplementedError("No database defined")

        database.configure_prefect_server(stateful_set_spec, container_template)

        return {
            "apiVersion": "apps/v1",
            "kind": "StatefulSet",
            "metadata": {"namespace": self.namespace, "name": self.name},
            "spec": stateful_set_spec,
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

    api = kubernetes.client.AppsV1Api()
    desired_stateful_set = server.desired_stateful_set()

    try:
        api.create_namespaced_stateful_set(
            server.namespace,
            desired_stateful_set,
        )
        logger.info("Created stateful set %s", name)
    except kubernetes.client.ApiException as e:
        if e.status != 409:
            raise

        api.replace_namespaced_stateful_set(
            desired_stateful_set["metadata"]["name"],
            server.namespace,
            desired_stateful_set,
        )
        logger.info("Updated stateful set %s", name)

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

    api = kubernetes.client.AppsV1Api()
    try:
        api.delete_namespaced_stateful_set(name, namespace)
        logger.info("Deleted stateful set %s", name)
    except kubernetes.client.ApiException as e:
        if e.status == 404:
            logger.info("Stateful set %s not found", name)
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


class PrefectServerReference(BaseModel):
    namespace: str = Field("")
    name: str

    @property
    def as_environment_variable(self) -> dict[str, Any]:
        return {"name": "PREFECT_API_URL", "value": self.in_cluster_api_url}

    @property
    def in_cluster_api_url(self) -> str:
        return f"http://{self.name}.{self.namespace}.svc:4200/api"

    @contextmanager
    def client(self) -> Generator[httpx.Client, None, None]:
        with httpx.Client(base_url=self.in_cluster_api_url) as c:
            yield c


class PrefectWorkPool(NamedResource):
    server: PrefectServerReference
    workers: int = Field(1)

    @property
    def work_pool_name(self) -> str:
        return f"{self.namespace}:{self.name}"

    def desired_deployment(self) -> dict[str, Any]:
        container_template = {
            "name": "prefect-worker",
            "image": "prefecthq/prefect:3.0.0rc2-python3.12-kubernetes",
            "env": [
                self.server.as_environment_variable,
            ],
            "command": [
                "bash",
                "-c",
                (
                    "prefect worker start --type kubernetes "
                    f"--pool '{ self.work_pool_name }' "
                    f'--name "{ self.namespace }:${{HOSTNAME}}"'
                ),
            ],
        }

        pod_template: dict[str, Any] = {
            "metadata": {"labels": {"app": self.name}},
            "spec": {
                "containers": [container_template],
            },
        }

        deployment_spec = {
            "replicas": self.workers,
            "selector": {"matchLabels": {"app": self.name}},
            "template": pod_template,
        }

        return {
            "apiVersion": "apps/v1",
            "kind": "Deployment",
            "metadata": {"namespace": self.namespace, "name": self.name},
            "spec": deployment_spec,
        }


@kopf.on.resume("prefect.io", "v3", "prefectworkpool")
@kopf.on.create("prefect.io", "v3", "prefectworkpool")
@kopf.on.update("prefect.io", "v3", "prefectworkpool")
def reconcile_work_pool(
    namespace: str, name: str, spec: dict[str, Any], logger: kopf.Logger, **_
):
    work_pool = PrefectWorkPool.model_validate(
        spec, context={"name": name, "namespace": namespace}
    )
    print(repr(work_pool))

    api = kubernetes.client.AppsV1Api()
    desired_deployment = work_pool.desired_deployment()

    try:
        api.create_namespaced_deployment(
            work_pool.namespace,
            desired_deployment,
        )
        logger.info("Created deployment %s", name)
    except kubernetes.client.ApiException as e:
        if e.status != 409:
            raise

        api.replace_namespaced_deployment(
            desired_deployment["metadata"]["name"],
            work_pool.namespace,
            desired_deployment,
        )
        logger.info("Updated deployment %s", name)


@kopf.on.delete("prefect.io", "v3", "prefectworkpool")
def delete_work_pool(
    namespace: str, name: str, spec: dict[str, Any], logger: kopf.Logger, **_
):
    work_pool = PrefectWorkPool.model_validate(
        spec, context={"name": name, "namespace": namespace}
    )
    print(repr(work_pool))

    api = kubernetes.client.AppsV1Api()
    try:
        api.delete_namespaced_deployment(name, namespace)
        logger.info("Deleted deployment %s", name)
    except kubernetes.client.ApiException as e:
        if e.status == 404:
            logger.info("deployment %s not found", name)
        else:
            raise
