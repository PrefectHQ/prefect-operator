import typing

import kubernetes
from httpcore import SOCKET_OPTION, ConnectionPool, NetworkBackend, NetworkStream
from httpcore._backends.sync import SyncStream
from httpx import Client, HTTPTransport, Limits, create_ssl_context
from httpx._config import DEFAULT_LIMITS
from httpx._types import CertTypes, ProxyTypes, VerifyTypes
from kubernetes import config
from kubernetes.stream import portforward
from kubernetes.stream.ws_client import PortForward


class KubernetesPortForwardBackend(NetworkBackend):
    def __init__(self) -> None:
        config.load_kube_config()

        self._api = kubernetes.client.CoreV1Api()

    def connect_tcp(
        self,
        host: str,
        port: int,
        timeout: typing.Optional[float] = None,
        local_address: typing.Optional[str] = None,
        socket_options: typing.Optional[typing.Iterable[SOCKET_OPTION]] = None,
    ) -> NetworkStream:
        try:
            name, namespace, kind, *_ = host.split(".")
        except ValueError:
            raise NotImplementedError(f"Unsupported hostname: {host}")

        if kind == "svc":
            try:
                service = self._api.read_namespaced_service(name, namespace)
            except kubernetes.client.rest.ApiException as e:
                if e.status == 404:
                    raise NotImplementedError(
                        f"Service {name!r} not found in namespace {namespace!r}"
                    )
                raise

            selector = service.spec.selector

            pods = self._api.list_namespaced_pod(
                namespace=namespace,
                label_selector=" ".join(f"{k}={v}" for k, v in selector.items()),
            )
            for pod in pods.items:
                if pod.status.phase == "Running":
                    name = pod.metadata.name
                    break
            else:
                raise NotImplementedError(
                    f"No running pod found matching the service selector: {selector}"
                )
        elif kind != "pod":
            raise NotImplementedError(f"Unsupported hostname: {host}")

        forward: PortForward = portforward(
            self._api.connect_get_namespaced_pod_portforward,
            namespace=namespace,
            name=name,
            ports=f"{port}",
        )

        socket: PortForward._Port._Socket = forward.socket(port)

        return SyncStream(socket)


class KubernetesPortForwardTransport(HTTPTransport):
    def __init__(
        self,
        verify: VerifyTypes = True,
        cert: CertTypes | None = None,
        http1: bool = True,
        http2: bool = False,
        limits: Limits = DEFAULT_LIMITS,
        trust_env: bool = True,
        proxy: ProxyTypes | None = None,
        uds: str | None = None,
        local_address: str | None = None,
        retries: int = 0,
        socket_options: typing.Iterable[SOCKET_OPTION] | None = None,
    ) -> None:
        super().__init__(
            verify=verify,
            cert=cert,
            http1=http1,
            http2=http2,
            limits=limits,
            trust_env=trust_env,
            proxy=proxy,
            uds=uds,
            local_address=local_address,
            retries=retries,
            socket_options=socket_options,
        )
        ssl_context = create_ssl_context(verify=verify, cert=cert, trust_env=trust_env)
        self._pool = ConnectionPool(
            ssl_context=ssl_context,
            max_connections=limits.max_connections,
            max_keepalive_connections=limits.max_keepalive_connections,
            keepalive_expiry=limits.keepalive_expiry,
            http1=http1,
            http2=http2,
            uds=uds,
            local_address=local_address,
            retries=retries,
            socket_options=socket_options,
            network_backend=KubernetesPortForwardBackend(),
        )


def main():
    config.load_kube_config()

    with Client(transport=KubernetesPortForwardTransport()) as client:
        response = client.get("http://my-server.pod-pg.svc:4200/api/health")
        print(response)


if __name__ == "__main__":
    main()
