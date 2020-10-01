liqo
==========
A Helm chart for Liqo

Current chart version is `0.1.0`



## Chart Requirements

| Repository | Name | Version |
|------------|------|---------|
| file://subcharts/advertisementOperator/ | advertisementOperator | 0.1.0 |
| file://subcharts/discoveryOperator/ | discoveryOperator | 0.1.0 |
| file://subcharts/networkModule/ | networkModule | 0.1.0 |
| file://subcharts/peeringRequestOperator/ | peeringRequestOperator | 0.1.0 |
| file://subcharts/schedulingNodeOperator/ | schedulingNodeOperator | 0.1.0 |
| file://subcharts/tunnelEndpointCreator/ | tunnelEndpointCreator | 0.1.0 |
| file://subcharts/liqoDash/ | liqoDash | 0.1.0 |

## Chart Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| advertisementOperator.advController.foreignClusterID | string | `"cluster-2"` |  |
| advertisementOperator.advController.image.pullPolicy | string | `"IfNotPresent"` |  |
| advertisementOperator.advController.image.repository | string | `"liqo/advertisement-operator"` |  |
| advertisementOperator.broadcaster.image.pullPolicy | string | `"IfNotPresent"` |  |
| advertisementOperator.broadcaster.image.repository | string | `"liqo/advertisement-broadcaster"` |  |
| advertisementOperator.enabled | bool | `true` |  |
| discoveryOperator.image.pullPolicy | string | `"IfNotPresent"` |  |
| discoveryOperator.image.repository | string | `"liqo/discovery"` |  |
| discoveryOperator.enabled | bool | `true` |  |
| configmap.clusterID | string | `"cluster-1"` |  |
| configmap.gatewayIP | string | `"10.251.0.1"` |  |
| configmap.gatewayPrivateIP | string | `"10.244.2.47"` |  |
| configmap.podCIDR | string | `"10.244.0.0/16"` |  |
| configmap.serviceCIDR | string | `"10.96.0.0/12"` |  |
| global.configmapName | string | `"liqo-configmap"` |  |
| networkModule.enabled | bool | `true` |  |
| networkModule.routeOperator.image.pullPolicy | string | `"IfNotPresent"` |  |
| networkModule.routeOperator.image.repository | string | `"liqo/liqonet"` |  |
| networkModule.tunnelEndpointOperator.image.pullPolicy | string | `"IfNotPresent"` |  |
| networkModule.tunnelEndpointOperator.image.repository | string | `"liqo/liqonet"` |  |
| peeringRequestOperator.image.pullPolicy | string | `"IfNotPresent"` |  |
| peeringRequestOperator.image.repository | string | `"liqo/peering-request-operator"` |  |
| peeringRequestOperator.enabled | bool | `true` |  |
| schedulingNodeOperator.enabled | bool | `true` |  |
| schedulingNodeOperator.image.pullPolicy | string | `"IfNotPresent"` |  |
| schedulingNodeOperator.image.repository | string | `"liqo/schedulingnode-operator"` |  |
| tunnelEndpointCreator.enabled | bool | `true` |  |
| tunnelEndpointCreator.image.pullPolicy | string | `"IfNotPresent"` |  |
| tunnelEndpointCreator.image.repository | string | `"liqo/liqonet"` |  |
| liqoDash.enabled | bool | `true` |  |
| liqoDash.image.pullPolicy | string | `"IfNotPresent"` |  |
| liqoDash.image.repository | string | `"liqo/dashboard"` |  |
