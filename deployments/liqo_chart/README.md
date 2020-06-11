liqo_chart
==========
A Helm chart for Liqo

Current chart version is `0.1.0`



## Chart Requirements

| Repository | Name | Version |
|------------|------|---------|
| file://subcharts/adv_chart/ | adv_chart | 0.1.0 |
| file://subcharts/networkModule_chart/ | networkModule_chart | 0.1.0 |
| file://subcharts/schedulingNodeOperator_chart/ | schedulingNodeOperator_chart | 0.1.0 |
| file://subcharts/tunnelEndpointCreator_chart/ | tunnelEndpointCreator_chart | 0.1.0 |

## Chart Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| adv_chart.advController.foreignClusterID | string | `"cluster-2"` |  |
| adv_chart.advController.image.pullPolicy | string | `"IfNotPresent"` |  |
| adv_chart.advController.image.repository | string | `"liqo/advertisement-operator"` |  |
| adv_chart.broadcaster.image.pullPolicy | string | `"IfNotPresent"` |  |
| adv_chart.broadcaster.image.repository | string | `"liqo/advertisement-broadcaster"` |  |
| adv_chart.enabled | bool | `true` |  |
| configmap.clusterID | string | `"cluster-1"` |  |
| configmap.gatewayIP | string | `"10.251.0.1"` |  |
| configmap.gatewayPrivateIP | string | `"10.244.2.47"` |  |
| configmap.podCIDR | string | `"10.244.0.0/16"` |  |
| configmap.serviceCIDR | string | `"10.96.0.0/12"` |  |
| global.configmapName | string | `"liqo-configmap"` |  |
| networkModule_chart.enabled | bool | `true` |  |
| networkModule_chart.routeOperator.image.pullPolicy | string | `"IfNotPresent"` |  |
| networkModule_chart.routeOperator.image.repository | string | `"liqo/liqonet"` |  |
| networkModule_chart.tunnelEndpointOperator.image.pullPolicy | string | `"IfNotPresent"` |  |
| networkModule_chart.tunnelEndpointOperator.image.repository | string | `"liqo/liqonet"` |  |
| schedulingNodeOperator_chart.enabled | bool | `true` |  |
| schedulingNodeOperator_chart.image.pullPolicy | string | `"IfNotPresent"` |  |
| schedulingNodeOperator_chart.image.repository | string | `"liqo/schedulingnode-operator"` |  |
| tunnelEndpointCreator_chart.enabled | bool | `true` |  |
| tunnelEndpointCreator_chart.image.pullPolicy | string | `"IfNotPresent"` |  |
| tunnelEndpointCreator_chart.image.repository | string | `"liqo/liqonet"` |  |
