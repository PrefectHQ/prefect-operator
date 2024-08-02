from contextlib import contextmanager
from typing import Any, ClassVar, Generator

import httpx
import kopf
import kubernetes
from pydantic import BaseModel, Field

from . import DEFAULT_PREFECT_VERSION
from .resources import NamedResource


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
    kind: ClassVar[str] = "PrefectWorkPool"
    plural: ClassVar[str] = "prefectworkpools"
    singular: ClassVar[str] = "prefectworkpool"

    # TODO: can we get the version from the server version at runtime?
    version: str = Field(DEFAULT_PREFECT_VERSION)
    server: PrefectServerReference
    workers: int = Field(1)

    @property
    def work_pool_name(self) -> str:
        return f"{self.namespace}:{self.name}"

    def desired_deployment(self) -> dict[str, Any]:
        container_template = {
            "name": "prefect-worker",
            "image": f"prefecthq/prefect:{self.version}-python3.12-kubernetes",
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
