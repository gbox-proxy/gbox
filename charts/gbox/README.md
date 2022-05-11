# GBox Chart for Kubernetes

![Version: 1.0.3](https://img.shields.io/badge/Version-1.0.3-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: v1.0.5](https://img.shields.io/badge/AppVersion-v1.0.5-informational?style=flat-square)

GBox Helm chart for Kubernetes. GBox is a reverse proxy in front of any GraphQL server for caching, securing and monitoring.

## Installing the Chart

To install the chart with the release name `my-release`, run the following commands:

    helm repo add gbox https://gbox-proxy.github.io/gbox
    helm install my-release gbox/gbox

## Requirements

| Repository | Name | Version |
|------------|------|---------|
| https://charts.bitnami.com/bitnami | redis | 16.8.9 |

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| adminAuth.enabled | bool | `true` | Whether enable basic auth when interact with GraphQL admin endpoint. |
| adminAuth.password | string | "gbox" | Basic auth password. |
| adminAuth.username | string | `"gbox"` | Basic auth username. |
| affinity | object | `{}` | [Affinity](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#affinity-and-anti-affinity) configuration. See the [API reference](https://kubernetes.io/docs/reference/kubernetes-api/workload-resources/pod-v1/#scheduling) for details. |
| autoscaling | object | Disabled by default. | Autoscaling by resources |
| caching.autoInvalidateCache | string | `"true"` | Whether auto invalidate cached data through mutation results or not. |
| caching.debugHeaders | string | `"false"` | Whether add debug headers to query operations responses or not. |
| caching.enabled | bool | `true` | Whether enable caching or not. |
| caching.rules | string | Will cache all query results, see [values.yaml](values.yaml). | Caching rules configuration. |
| caching.storeDsn | string | See [values.yaml](values.yaml). | By default, this chart use Redis to storing cached data, if you want to use your external Redis server, remember to disable internal Redis sub-chart. |
| caching.typeKeys | string | `""` | Specific type keys configuration, by default `id` is key of all types. |
| caching.varies | string | `""` | Caching varies configuration. |
| complexity.enabled | bool | `true` | Whether enable filter query complexity or not. |
| complexity.maxComplexity | int | `60` | The maximum number of Node requests that might be needed to execute the query. |
| complexity.maxDepth | int | `15` | Max query depth. |
| complexity.nodeCountLimit | int | `60` | The maximum number of Nodes a query may return. |
| disabledIntrospection | bool | `false` | Whether disable introspection queries or not. |
| disabledPlaygrounds | bool | `false` | Whether disable playgrounds or not. |
| extraDirectives | string | `""` | GBox extra directives, useful in cases you may want to add CORS config and/or http headers when fetch schema from upstream. |
| fetchSchemaInterval | string | `"10m"` | Interval times to introspect upstream schema definition. |
| fullnameOverride | string | `""` | A name to substitute for the full names of resources. |
| globalDirectives | string | `""` | Caddy [global directives](https://caddyserver.com/docs/caddyfile/options). |
| image.pullPolicy | string | `"IfNotPresent"` | [Image pull policy](https://kubernetes.io/docs/concepts/containers/images/#updating-images) for updating already existing images on a node. |
| image.repository | string | `"gboxproxy/gbox"` | Name of the image repository to pull the container image from. |
| image.tag | string | `""` | Overrides the image tag whose default is the chart appVersion. |
| imagePullSecrets | list | `[]` | Reference to one or more secrets to be used when [pulling images](https://kubernetes.io/docs/tasks/configure-pod-container/pull-image-private-registry/#create-a-pod-that-uses-your-secret) (from private registries). |
| ingress.annotations | object | `{}` | Annotations to be added to the ingress. |
| ingress.className | string | `""` | Ingress [class name](https://kubernetes.io/docs/concepts/services-networking/ingress/#ingress-class). |
| ingress.enabled | bool | `false` | Enable [ingress](https://kubernetes.io/docs/concepts/services-networking/ingress/). |
| ingress.hosts[0].host | string | `"chart-example.local"` |  |
| ingress.hosts[0].paths[0].path | string | `"/"` |  |
| ingress.hosts[0].paths[0].pathType | string | `"ImplementationSpecific"` |  |
| ingress.tls | list | See [values.yaml](values.yaml). | Ingress TLS configuration. |
| metrics.enabled | bool | `true` | Whether enable Prometheus metric endpoint or not |
| metrics.path | string | `"/metrics"` | Url path of metric endpoint. |
| nameOverride | string | `""` | A name in place of the chart name for `app:` labels. |
| nodeSelector | object | `{}` | [Node selector](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#nodeselector) configuration. |
| podAnnotations | object | `{}` | Annotations to be added to pods. |
| podSecurityContext | object | `{}` | Pod [security context](https://kubernetes.io/docs/tasks/configure-pod-container/security-context/#set-the-security-context-for-a-pod). See the [API reference](https://kubernetes.io/docs/reference/kubernetes-api/workload-resources/pod-v1/#security-context) for details. |
| redis.architecture | string | `"standalone"` | Set Redis architecture standalone or replication. |
| redis.auth.password | string | `"!ChangeMe!"` |  |
| redis.enabled | bool | `true` | Whether enable Redis sub-chart or not. |
| replicaCount | int | `1` | The number of replicas (pods) to launch |
| resources | object | No requests or limits. | Container resource [requests and limits](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/). See the [API reference](https://kubernetes.io/docs/reference/kubernetes-api/workload-resources/pod-v1/#resources) for details. |
| reverseProxyDirectives | string | `""` | Reverse proxy [directives](https://caddyserver.com/docs/caddyfile/directives/reverse_proxy). |
| securityContext | object | `{}` | Container [security context](https://kubernetes.io/docs/tasks/configure-pod-container/security-context/#set-the-security-context-for-a-container). See the [API reference](https://kubernetes.io/docs/reference/kubernetes-api/workload-resources/pod-v1/#security-context-1) for details. |
| service.port | int | `80` | Service port. |
| service.type | string | `"ClusterIP"` | Kubernetes [service type](https://kubernetes.io/docs/concepts/services-networking/service/#publishing-services-service-types). |
| serviceAccount.annotations | object | `{}` | Annotations to add to the service account |
| serviceAccount.create | bool | `true` | Specifies whether a service account should be created |
| serviceAccount.name | string | `""` | The name of the service account to use. If not set and create is true, a name is generated using the fullname template |
| tolerations | list | `[]` | [Tolerations](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/) for node taints. See the [API reference](https://kubernetes.io/docs/reference/kubernetes-api/workload-resources/pod-v1/#scheduling) for details. |
| upstream | string | `""` | Your upstream GraphQL server url. |
