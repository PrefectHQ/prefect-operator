# prefect-operator

![Version: 0.0.0](https://img.shields.io/badge/Version-0.0.0-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: 2-latest](https://img.shields.io/badge/AppVersion-2--latest-informational?style=flat-square)

Prefect Operator application bundle

**Homepage:** <https://github.com/PrefectHQ>

## Maintainers

| Name | Email | Url |
| ---- | ------ | --- |
| jamiezieziula | <jamie@prefect.io> |  |
| jimid27 | <jimi@prefect.io> |  |
| parkedwards | <edward@prefect.io> |  |
| mitchnielsen | <mitchell@prefect.io> |  |

## Source Code

* <https://github.com/PrefectHQ/prefect-operator>

## Requirements

| Repository | Name | Version |
|------------|------|---------|
| https://charts.bitnami.com/bitnami | common | 2.20.5 |

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| commonAnnotations | object | `{}` | annotations to add to all deployed objects |
| commonLabels | object | `{"app.kubernetes.io/component":"operator"}` | labels to add to all deployed objects |
| fullnameOverride | string | `"prefect-operator"` | fully override common.names.fullname |
| kubeRbacProxy.create | bool | `true` | specifies whether the kube-rbac-proxy should be deployed to the cluster |
| kubeRbacProxy.image | string | `"gcr.io/kubebuilder/kube-rbac-proxy:v0.16.0"` | the image of the kube-rbac-proxy to use |
| kubeRbacProxy.name | string | `"kube-rbac-proxy"` | the name of the kube-rbac-proxy to use |
| metrics.enabled | bool | `false` | enable the export of Prometheus metrics |
| metrics.serviceMonitor.enabled | bool | `false` | creates a Prometheus Operator ServiceMonitor (also requires `metrics.enabled` to be `true`) |
| nameOverride | string | `""` | partially overrides common.names.name |
| namespaceOverride | string | `""` | fully override common.names.namespace |
| operator.affinity | object | `{}` | affinity for operator pods assignment |
| operator.containerSecurityContext.allowPrivilegeEscalation | bool | `false` | set operator containers' security context allowPrivilegeEscalation |
| operator.containerSecurityContext.capabilities | object | `{"drop":["ALL"]}` | set operator container's security context capabilities |
| operator.extraEnvVars | list | `[]` | array with environment variables to add to operator container |
| operator.image.pullPolicy | string | `"IfNotPresent"` | operator image pull policy |
| operator.image.pullSecrets | list | `[]` | operator image pull secrets |
| operator.image.repository | string | `"prefecthq/prefect-operator"` | operator image repository |
| operator.image.tag | string | `"latest"` | operator image tag (immutable tags are recommended) |
| operator.livenessProbe.config.initialDelaySeconds | int | `15` | The number of seconds to wait before starting the first probe. |
| operator.livenessProbe.config.periodSeconds | int | `20` | The number of seconds to wait between consecutive probes. |
| operator.livenessProbe.enabled | bool | `true` |  |
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
| rbac.userRoles.prefectServer.editor.create | bool | `true` | specifies whether the server editor role should be created |
| rbac.userRoles.prefectServer.viewer.create | bool | `true` | specifies whether the server viewer role should be created |
| rbac.userRoles.prefectWorkpool.editor.create | bool | `true` | specifies whether the workpool editor role should be created |
| rbac.userRoles.prefectWorkpool.viewer.create | bool | `true` | specifies whether the workpool viewer role should be created |
| serviceAccount.annotations | object | `{}` | additional service account annotations (evaluated as a template) |
| serviceAccount.create | bool | `true` | specifies whether a ServiceAccount should be created |
| serviceAccount.name | string | `""` | the name of the ServiceAccount to use. if not set and create is true, a name is generated using the common.names.fullname template |

----------------------------------------------
Autogenerated from chart metadata using [helm-docs v1.13.1](https://github.com/norwoodj/helm-docs/releases/v1.13.1)
