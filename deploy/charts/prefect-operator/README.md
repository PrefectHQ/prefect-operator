# prefect-operator

## Installing the Chart

### Prerequisites

1. Add the Prefect Helm repository to your Helm client:

    ```bash
    helm repo add prefect-operator https://prefecthq.github.io/prefect-operator
    helm repo update
    ```

2. Create a new namespace in your Kubernetes cluster to deploy the Prefect Operator in:

    ```bash
    kubectl create namespace prefect-system
    ```

### [Optional] Verify the Chart

We use a PGP key to sign the Helm chart as recommended by Helm.

If you would like to verify the charts signature before installing it, you can do so by following the instructions below:

```bash
# Pull Prefects public PGP key from keybase
curl https://keybase.io/prefecthq/pgp_keys.asc | gpg --dearmor > .gnupg/pubring.gpg
# Run `helm fetch` to validate the chart
helm fetch --verify prefect-operator/prefect-operator --version 2024.9.15203739 --keyring .gnupg/pubring.gpg
```

### Install the Chart

1. Install the Prefect Operator using Helm

    ```bash
    helm install prefect-operator prefect-operator/prefect-operator --namespace=prefect-system -f values.yaml
    ```

2. Verify the deployment

    Check the status of your Prefect Operatr deployment:

    ```bash
    kubectl get pods -n prefect-system

    NAME                                READY   STATUS    RESTARTS       AGE
    prefect-operator-69874bdc54-lc9vk   2/2     Running   0              25m
    ```

    You should see the Prefect Operator pod running

## Upgrading the Chart

`helm upgrade` updates the operator's CRDs along with the rest of the chart.

Only one release can own the CRDs. If you run more than one release of the chart in a cluster, set `crds.install=false` on all but one of them.

### Upgrading from a chart that installed CRDs from `crds/`

Earlier versions of the chart installed the CRDs from its `crds/` directory. Helm never upgrades those, and it doesn't record them as part of the release. The first upgrade to this version fails with an error that a CRD "exists and cannot be imported into the current release".

With Helm 3.17 or later, pass `--take-ownership` on that one upgrade:

```bash
helm upgrade prefect-operator prefect-operator/prefect-operator --namespace=prefect-system -f values.yaml --take-ownership
```

With an earlier Helm, mark the CRDs as belonging to the release first, then upgrade as usual:

```bash
for crd in prefectservers prefectworkpools prefectworkqueues prefectdeployments prefectautomations; do
  kubectl label crd "$crd.prefect.io" app.kubernetes.io/managed-by=Helm --overwrite
  kubectl annotate crd "$crd.prefect.io" --overwrite \
    meta.helm.sh/release-name=prefect-operator \
    meta.helm.sh/release-namespace=prefect-system
done
```

Later upgrades need neither step.

## Uninstalling the Chart

To uninstall/delete the prefect-operator deployment:

```bash
helm delete prefect-operator -n prefect-system
```

The command removes the Kubernetes components associated with the chart and deletes the release. It leaves the CRDs, and the Prefect resources stored in them, in place.

To remove the CRDs too, delete the Prefect resources before you uninstall the chart, so the operator can run their finalizers. That changes Prefect the same way deleting each resource does. Then delete the CRDs:

```bash
kubectl delete prefectautomations,prefectdeployments,prefectworkqueues,prefectworkpools,prefectservers --all --all-namespaces
helm delete prefect-operator -n prefect-system
kubectl delete crd prefectautomations.prefect.io prefectdeployments.prefect.io prefectworkqueues.prefect.io prefectworkpools.prefect.io prefectservers.prefect.io
```

## FAQ
tbd

## Maintainers

| Name | Email | Url |
| ---- | ------ | --- |
| jamiezieziula | <jamie@prefect.io> |  |
| jimid27 | <jimi@prefect.io> |  |
| parkedwards | <edward@prefect.io> |  |
| mitchnielsen | <mitchell@prefect.io> |  |

## Requirements

| Repository | Name | Version |
|------------|------|---------|
| https://charts.bitnami.com/bitnami | common | 2.41.0 |

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| commonAnnotations | object | `{}` | annotations to add to all deployed objects |
| commonLabels | object | `{"app.kubernetes.io/component":"operator"}` | labels to add to all deployed objects |
| crds.install | bool | `true` | install and upgrade the operator's CRDs with the chart. Uninstalling the chart leaves them in place. Set to false when the CRDs are managed outside this release, such as by another release of the chart in the same cluster |
| fullnameOverride | string | `"prefect-operator"` | fully override common.names.fullname |
| nameOverride | string | `""` | partially overrides common.names.name |
| namespaceOverride | string | `""` | fully override common.names.namespace |
| operator.affinity | object | `{}` | affinity for operator pods assignment |
| operator.containerSecurityContext.allowPrivilegeEscalation | bool | `false` | set operator containers' security context allowPrivilegeEscalation |
| operator.containerSecurityContext.capabilities | object | `{"drop":["ALL"]}` | set operator container's security context capabilities |
| operator.defaultResyncInterval | string | `""` | default interval for checking Prefect resources for out-of-band drift. Uses the operator's 5m default when empty |
| operator.extraEnvVars | list | `[]` | array with environment variables to add to operator container |
| operator.image.pullPolicy | string | `"IfNotPresent"` | operator image pull policy |
| operator.image.pullSecrets | list | `[]` | operator image pull secrets |
| operator.image.repository | string | `"prefecthq/prefect-operator"` | operator image repository |
| operator.image.tag | string | `"latest"` | operator image tag (immutable tags are recommended) |
| operator.livenessProbe.config.initialDelaySeconds | int | `15` | The number of seconds to wait before starting the first probe. |
| operator.livenessProbe.config.periodSeconds | int | `20` | The number of seconds to wait between consecutive probes. |
| operator.livenessProbe.enabled | bool | `true` |  |
| operator.logFormat | string | `"json"` | operator log format: json or console. Uses the operator's json default when empty |
| operator.logLevel | string | `"info"` | operator log level: debug, info, error, or an integer above 0 for more verbose debug logs. Uses the operator's info default when empty |
| operator.nodeSelector | object | `{}` | node labels for operator pods assignment |
| operator.podAnnotations | object | `{}` | extra annotations for operator pod |
| operator.podLabels | object | `{}` | extra labels for operator pod |
| operator.podSecurityContext.runAsNonRoot | bool | `true` | set operator pod's security context runAsNonRoot |
| operator.priorityClassName | string | `""` | priority class name to use for the operator pods; if the priority class is empty or doesn't exist, the operator pods are scheduled without a priority class |
| operator.readinessProbe.config.initialDelaySeconds | int | `5` | The number of seconds to wait before starting the first probe. |
| operator.readinessProbe.config.periodSeconds | int | `10` | The number of seconds to wait between consecutive probes. |
| operator.readinessProbe.enabled | bool | `true` |  |
| operator.replicaCount | int | `1` | number of operator replicas to deploy |
| operator.resources.limits | object | `{"cpu":"500m","memory":"128Mi"}` | the requested limits for the operator container |
| operator.resources.requests | object | `{"cpu":"10m","memory":"64Mi"}` | the requested resources for the operator container |
| operator.terminationGracePeriodSeconds | int | `10` | seconds operator pod needs to terminate gracefully |
| operator.tolerations | list | `[]` | tolerations for operator pods assignment |
| operator.topologySpreadConstraints | list | `[]` | topology spread constraints for operator pod assignment spread across your cluster among failure-domains |
| rbac.operator.create | bool | `true` | specifies whether the operator role & role binding should be created |
| rbac.userRoles.prefectAutomation.editor.create | bool | `true` | specifies whether the automation editor role should be created |
| rbac.userRoles.prefectAutomation.viewer.create | bool | `true` | specifies whether the automation viewer role should be created |
| rbac.userRoles.prefectDeployment.editor.create | bool | `true` | specifies whether the deployment editor role should be created |
| rbac.userRoles.prefectDeployment.viewer.create | bool | `true` | specifies whether the deployment viewer role should be created |
| rbac.userRoles.prefectServer.editor.create | bool | `true` | specifies whether the server editor role should be created |
| rbac.userRoles.prefectServer.viewer.create | bool | `true` | specifies whether the server viewer role should be created |
| rbac.userRoles.prefectWorkQueue.editor.create | bool | `true` | specifies whether the work queue editor role should be created |
| rbac.userRoles.prefectWorkQueue.viewer.create | bool | `true` | specifies whether the work queue viewer role should be created |
| rbac.userRoles.prefectWorkpool.editor.create | bool | `true` | specifies whether the workpool editor role should be created |
| rbac.userRoles.prefectWorkpool.viewer.create | bool | `true` | specifies whether the workpool viewer role should be created |
| serviceAccount.annotations | object | `{}` | additional service account annotations (evaluated as a template) |
| serviceAccount.create | bool | `true` | specifies whether a ServiceAccount should be created |
| serviceAccount.name | string | `""` | the name of the ServiceAccount to use. if not set and create is true, a name is generated using the common.names.fullname template |

----------------------------------------------
Autogenerated from chart metadata using [helm-docs v1.14.2](https://github.com/norwoodj/helm-docs/releases/v1.14.2)
