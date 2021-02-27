## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| advertisement.broadcasterImageName | string | `"liqo/advertisement-broadcaster"` | broadcaster image repository |
| advertisement.config.ingoingConfig.acceptPolicy | string | `"AutoAcceptMax"` |  |
| advertisement.config.ingoingConfig.maxAcceptableAdvertisement | int | `5` |  |
| advertisement.config.keepaliveRetryTime | int | `20` |  |
| advertisement.config.keepaliveThreshold | int | `3` |  |
| advertisement.config.outgoingConfig.enableBroadcaster | bool | `true` |  |
| advertisement.config.outgoingConfig.resourceSharingPercentage | int | `30` |  |
| advertisement.imageName | string | `"liqo/advertisement-operator"` | advertisement image repository |
| advertisement.pod.annotations | object | `{}` | advertisement pod annotations |
| advertisement.pod.labels | object | `{}` | advertisement pod labels |
| apiServer.address | string | `""` | remote API server IP address |
| apiServer.port | string | `""` | remote API server port |
| auth.config.allowEmptyToken | bool | `false` | enable the authentication with an empty token. NB: use it only for testing installations |
| auth.imageName | string | `"liqo/auth-service"` | auth image repository |
| auth.ingress.annotations | object | `{}` | auth ingress annotations |
| auth.ingress.class | string | `""` |  |
| auth.ingress.enable | bool | `false` |  |
| auth.ingress.host | string | `""` |  |
| auth.ingress.port | string | `""` |  |
| auth.initContainer.imageName | string | `"nginx:1.19"` | auth init container image repository |
| auth.pod.annotations | object | `{}` | auth pod annotations |
| auth.pod.labels | object | `{}` | auth pod labels |
| auth.service.annotations | object | `{}` | auth service annotations |
| auth.service.type | string | `"NodePort"` | auth service type |
| auth.tls | bool | `true` | enable TLS for the Authentication Service Pod |
| crdReplicator.imageName | string | `"liqo/crd-replicator"` | crdReplicator image repository |
| crdReplicator.pod.annotations | object | `{}` | crdReplicator pod annotations |
| crdReplicator.pod.labels | object | `{}` | crdReplicator pod labels |
| discovery.config.autojoin | bool | `true` |  |
| discovery.config.autojoinUntrusted | bool | `true` |  |
| discovery.config.clusterName | string | `""` |  |
| discovery.config.domain | string | `"local."` |  |
| discovery.config.enableAdvertisement | bool | `true` |  |
| discovery.config.enableDiscovery | bool | `true` |  |
| discovery.config.name | string | `"MyLiqo"` |  |
| discovery.config.port | int | `6443` |  |
| discovery.config.service | string | `"_liqo_api._tcp"` |  |
| discovery.config.ttl | int | `90` |  |
| discovery.imageName | string | `"liqo/discovery"` | discovery image repository |
| discovery.pod.annotations | object | `{}` | discovery pod annotations |
| discovery.pod.labels | object | `{}` | discovery pod labels |
| fullnameOverride | string | `""` | full liqo name override |
| gateway.imageName | string | `"liqo/liqonet"` | gateway image repository |
| gateway.pod.annotations | object | `{}` | gateway pod annotations |
| gateway.pod.labels | object | `{}` | gateway pod labels |
| gateway.service.annotations | object | `{}` |  |
| gateway.service.type | string | `"NodePort"` | If you plan to use liqo over the Internet consider to change this field to "LoadBalancer". More generally, if your cluster nodes are not directly reachable by the cluster to whom you are peering then change it to "LoadBalancer" |
| nameOverride | string | `""` | liqo name override |
| networkManager.config.GKEProvider | bool | `false` | set this field to true if you are deploying liqo in GKE cluster |
| networkManager.config.podCIDR | string | `""` | The subnet used by the cluster for the pods, in CIDR notation. At the moment the internal IPAM used by liqo only supports podCIDRs with netmask /16 (255.255.0.0). |
| networkManager.config.reservedSubnets | list | `[]` | Usually the IPs used for the pods in k8s clusters belong to private subnets. In order to prevent IP conflicting between locally used private subnets in your infrastructure and private subnets belonging to remote clusters you need tell liqo the subnets used in your cluster. E.g if your cluster nodes belong to the 192.168.2.0/24 subnet then you should add that subnet to the reservedSubnets. PodCIDR and serviceCIDR used in the local cluster are automatically added to the reserved list. |
| networkManager.config.serviceCIDR | string | `""` | the subnet used by the cluster for the services, in CIDR notation |
| networkManager.imageName | string | `"liqo/liqonet"` | networkManager image repository |
| networkManager.pod.annotations | object | `{}` | networkManager pod annotations |
| networkManager.pod.labels | object | `{}` | networkManager pod labels |
| peeringRequest.imageName | string | `"liqo/peering-request-operator"` | peeringRequest image repository |
| peeringRequest.pod.annotations | object | `{}` | peering request pod annotations |
| peeringRequest.pod.labels | object | `{}` | peering request pod labels |
| pullPolicy | string | `"IfNotPresent"` | pullPolicy for liqo pods |
| route.imageName | string | `"liqo/liqonet"` | route image repository |
| route.pod.annotations | object | `{}` | route pod annotations |
| route.pod.labels | object | `{}` | route pod labels |
| tag | string | `""` | images' tag |
| virtualKubelet.imageName | string | `"liqo/virtual-kubelet"` | virtual kubelet image repository |
| virtualKubelet.initContainer.imageName | string | `"liqo/init-vkubelet"` | virtual kubelet init container image repository |
| webhook.imageName | string | `"liqo/liqo-webhook"` | webhook image repository |
| webhook.initContainer.imageName | string | `"liqo/webhook-configuration"` | webhook init container image repository |
| webhook.mutatingWebhookConfiguration.annotations | object | `{}` | mutatingWebhookConfiguration annotations |
| webhook.mutatingWebhookConfiguration.namespaceSelector | object | `{"liqo.io/enabled":"true"}` | mutatingWebhookConfiguration namespace selector |
| webhook.pod.annotations | object | `{}` | webhook pod annotations |
| webhook.pod.labels | object | `{}` | webhook pod labels |
| webhook.service.annotations | object | `{}` | webhook service annotations |
| webhook.service.type | string | `"ClusterIP"` | webhook service type |