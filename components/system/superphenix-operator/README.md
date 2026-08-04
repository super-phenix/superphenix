# superphenix-operator

![Version: 0.0.0](https://img.shields.io/badge/Version-0.0.0-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: 0.0.0](https://img.shields.io/badge/AppVersion-0.0.0-informational?style=flat-square)

A Helm chart for the Superphenix Operator

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| affinity | object | `{}` | Affinity rules for pod assignment. |
| clusters | object | `{}` | Superphenix clusters to be managed by this operator. You can either create the Cluster CRs by hand or use this field to define your clusters from the chart. |
| config | object | `{"argocd":{"ha":{"enabled":false},"values":{}},"clustersConfigMap":{"name":"superphenix-clusters-config"},"enableHTTP2":false,"management":{"values":{}},"syncPeriod":"5m","syncTimeout":"15m","system":{"chartName":"superphenix-system","repoURL":"ghcr.io/super-phenix/charts","version":""},"talosManager":{"chart":{"url":"ghcr.io/super-phenix/charts","version":"0.1.0"}},"telemetry":{"disabled":false},"valuesConfigMap":{"name":"superphenix-mgmt-values"}}` | Superphenix operator configuration |
| config.argocd.ha | object | `{"enabled":false}` | ArgoCD configuration. |
| config.argocd.ha.enabled | bool | `false` | Enable HA mode for ArgoCD deployment. |
| config.argocd.values | object | `{}` | Arbitrary Helm values merged under the "argocd" ConfigMap key. |
| config.clustersConfigMap | object | `{"name":"superphenix-clusters-config"}` | ConfigMap where all clusters will append their configuration. |
| config.enableHTTP2 | bool | `false` | Enable HTTP/2 for the metrics and webhook servers. |
| config.management.values | object | `{}` | Arbitrary Helm values merged under the "superphenix" ConfigMap key. |
| config.syncPeriod | string | `"5m"` | Interval at which to periodically resync sub-applications. |
| config.syncTimeout | string | `"15m"` | Duration after which an in-progress sub-application sync is considered stuck. |
| config.system | object | `{"chartName":"superphenix-system","repoURL":"ghcr.io/super-phenix/charts","version":""}` | Default system chart deployed by the cluster controller on every managed cluster. This can be overridden per cluster in the Cluster CR. |
| config.system.chartName | string | `"superphenix-system"` | Name of the system chart. |
| config.system.repoURL | string | `"ghcr.io/super-phenix/charts"` | Repository URL for the system chart. |
| config.system.version | string | `""` | Version of the system chart. If none provided, inherits from the operator chart. |
| config.talosManager | object | `{"chart":{"url":"ghcr.io/super-phenix/charts","version":"0.1.0"}}` | talos-manager chart deployed by the cluster controller for every managed cluster. |
| config.talosManager.chart.url | string | `"ghcr.io/super-phenix/charts"` | Repository URL for the talos-manager chart. |
| config.talosManager.chart.version | string | `"0.1.0"` | Version of the talos-manager chart. |
| config.telemetry | object | `{"disabled":false}` | Telemetry settings. |
| config.telemetry.disabled | bool | `false` | Disable sending anonymous telemetry. |
| config.valuesConfigMap | object | `{"name":"superphenix-mgmt-values"}` | General ConfigMap holding Helm values overrides for all management components. |
| fullnameOverride | string | `""` | String to fully override fullname template. |
| health.probeBindAddress | string | `":8081"` | The address the health probe endpoint binds to. |
| image.pullPolicy | string | `"IfNotPresent"` | Pull policy for the operator image. |
| image.repository | string | `"ghcr.io/super-phenix/superphenix-operator"` | Repository for the operator image. |
| image.tag | string | `""` | Overrides the image tag whose default is the chart appVersion. |
| imagePullSecrets | list | `[]` | Secrets for pulling the operator image. |
| leaderElection.enabled | bool | `false` | Enable leader election for the operator, necessary if running multiple replicas. |
| metrics | object | `{"bindAddress":"0","secure":true}` | Metrics and Health configuration. |
| metrics.bindAddress | string | `"0"` | The address the metric endpoint binds to. Use "0" to disable. |
| metrics.secure | bool | `true` | Whether to secure the metrics endpoint with TLS. |
| nameOverride | string | `""` | String to partially override fullname template. |
| nodeSelector | object | `{}` | Node selector for pod assignment. |
| podAnnotations | object | `{}` | Annotations for the operator pods. |
| podSecurityContext | object | `{"runAsNonRoot":true,"seccompProfile":{"type":"RuntimeDefault"}}` | Pod-level security context. |
| podSecurityContext.runAsNonRoot | bool | `true` | Whether to run as a non-root user. |
| podSecurityContext.seccompProfile.type | string | `"RuntimeDefault"` | Type of seccomp profile to use. |
| replicaCount | int | `1` | Number of replicas for the operator deployment. |
| resources | object | `{"limits":{"cpu":"500m","memory":"256Mi"},"requests":{"cpu":"10m","memory":"64Mi"}}` | Resource limits and requests for the operator container. |
| resources.limits.cpu | string | `"500m"` | CPU limit for the operator. |
| resources.limits.memory | string | `"256Mi"` | Memory limit for the operator. |
| resources.requests.cpu | string | `"10m"` | CPU request for the operator. |
| resources.requests.memory | string | `"64Mi"` | Memory request for the operator. |
| securityContext | object | `{"allowPrivilegeEscalation":false,"capabilities":{"drop":["ALL"]},"readOnlyRootFilesystem":true}` | Container-level security context. |
| securityContext.allowPrivilegeEscalation | bool | `false` | Whether to allow privilege escalation. |
| securityContext.readOnlyRootFilesystem | bool | `true` | Whether to mount the container's root filesystem as read-only. |
| serviceAccount.annotations | object | `{}` | Annotations to add to the service account. |
| serviceAccount.create | bool | `true` | Specifies whether a service account should be created. |
| serviceAccount.name | string | `""` | The name of the service account to use. If not set and create is true, a name is generated using the fullname template. |
| tolerations | list | `[]` | Tolerations for pod assignment. |

----------------------------------------------
Autogenerated from chart metadata using [helm-docs v1.14.2](https://github.com/norwoodj/helm-docs/releases/v1.14.2)
