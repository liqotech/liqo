---
title: Chart values
weight: 5
---

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| advertisement.broadcasterImageName | string | `"liqo/advertisement-broadcaster"` | broadcaster image repository |
| advertisement.config.enableBroadcaster | bool | `true` | If set to false, the remote clusters will not be able to leverage your resources, but you will still be able to use theirs. |
| advertisement.config.resourceSharingPercentage | int | `30` | It defines the percentage of available cluster resources that you are willing to share with foreign clusters. |
| advertisement.imageName | string | `"liqo/advertisement-operator"` | advertisement image repository |
| advertisement.pod.annotations | object | `{}` | advertisement pod annotations |
| advertisement.pod.labels | object | `{}` | advertisement pod labels |
| apiServer.address | string | `""` | The address that must be used to contact your API server, it needs to be reachable from the clusters that you will peer with (defaults to your master IP) |
| apiServer.port | string | `"6443"` | The port that must be used to contact your API server |
| auth.config.allowEmptyToken | bool | `false` | Set to true to disable the authentication of discovered clusters. NB: use it only for testing installations |
| auth.imageName | string | `"liqo/auth-service"` | auth image repository |
| auth.ingress.annotations | object | `{}` | Auth ingress annotations |
| auth.ingress.class | string | `""` | Set your ingress class |
| auth.ingress.enable | bool | `false` | Whether to enable the creation of the Ingress resource |
| auth.ingress.host | string | `""` | Set the hostname for your ingress |
| auth.initContainer.imageName | string | `"nginx:1.19"` | auth init container image repository |
| auth.pod.annotations | object | `{}` | auth pod annotations |
| auth.pod.labels | object | `{}` | auth pod labels |
| auth.portOverride | string | `""` | Overrides the port were your service is available, you should configure it if behind a NAT or using an Ingress with a port different from 443. |
| auth.service.annotations | object | `{}` | auth service annotations |
| auth.service.type | string | `"NodePort"` | The type of service used to expose the Authentication Service If you are exposing this service with an Ingress consider to change it to ClusterIP, otherwise if you plan to use liqo over the Internet consider to change this field to "LoadBalancer". See https://doc.liqo.io/user/scenarios/ for more details. |
| auth.tls | bool | `true` | Enable TLS for the Authentication Service Pod (using a self-signed certificate). If you are exposing this service with an Ingress consider to disable it or add the appropriate annotations to the Ingress resource. |
| crdReplicator.imageName | string | `"liqo/crd-replicator"` | crdReplicator image repository |
| crdReplicator.pod.annotations | object | `{}` | crdReplicator pod annotations |
| crdReplicator.pod.labels | object | `{}` | crdReplicator pod labels |
| discovery.config.autojoin | bool | `true` | Automatically join discovered cluster exposing the Authentication Service with a valid certificate |
| discovery.config.autojoinUntrusted | bool | `true` | Automatically join discovered cluster exposing the Authentication Service with a self-signed certificate |
| discovery.config.clusterName | string | `""` | Set a mnemonic name for your cluster |
| discovery.config.enableAdvertisement | bool | `true` | Enable the mDNS advertisement on LANs, set to false to not be discoverable from other clusters in the same LAN |
| discovery.config.enableDiscovery | bool | `true` | Enable the mDNS discovery on LANs, set to false to not look for other clusters available in the same LAN |
| discovery.config.ttl | int | `90` | Time-to-live before an automatically discovered clusters is deleted from the list of available ones if no longer announced (in seconds) |
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
| networkManager.config.GKEProvider | bool | `false` | Set this field to true if you are deploying liqo in GKE cluster |
| networkManager.config.podCIDR | string | `""` | The subnet used by the cluster for the pods, in CIDR notation. At the moment the internal IPAM used by liqo only supports podCIDRs with netmask /16 (255.255.0.0). |
| networkManager.config.reservedSubnets | list | `[]` | Usually the IPs used for the pods in k8s clusters belong to private subnets. In order to prevent IP conflicting between locally used private subnets in your infrastructure and private subnets belonging to remote clusters you need tell liqo the subnets used in your cluster. E.g if your cluster nodes belong to the 192.168.2.0/24 subnet then you should add that subnet to the reservedSubnets. PodCIDR and serviceCIDR used in the local cluster are automatically added to the reserved list. |
| networkManager.config.serviceCIDR | string | `""` | The subnet used by the cluster for the services, in CIDR notation |
| networkManager.imageName | string | `"liqo/liqonet"` | networkManager image repository |
| networkManager.pod.annotations | object | `{}` | networkManager pod annotations |
| networkManager.pod.labels | object | `{}` | networkManager pod labels |
| peeringRequest.imageName | string | `"liqo/peering-request-operator"` | peeringRequest image repository |
| peeringRequest.pod.annotations | object | `{}` | peering request pod annotations |
| peeringRequest.pod.labels | object | `{}` | peering request pod labels |
| pullPolicy | string | `"IfNotPresent"` | The pullPolicy for liqo pods |
| route.imageName | string | `"liqo/liqonet"` | route image repository |
| route.pod.annotations | object | `{}` | route pod annotations |
| route.pod.labels | object | `{}` | route pod labels |
| tag | string | `""` | Images' tag to select a development version of liqo instead of a release |
| virtualKubelet.imageName | string | `"liqo/virtual-kubelet"` | virtual kubelet image repository |
| virtualKubelet.initContainer.imageName | string | `"liqo/init-vkubelet"` | virtual kubelet init container image repository |
| webhook.imageName | string | `"liqo/liqo-webhook"` | webhook image repository |
| webhook.initContainer.imageName | string | `"liqo/webhook-configuration"` | webhook init container image repository |
| webhook.mutatingWebhookConfiguration.annotations | object | `{}` | mutatingWebhookConfiguration annotations |
| webhook.mutatingWebhookConfiguration.namespaceSelector | object | `{"liqo.io/enabled":"true"}` | The label that needs to be applied to a namespace to make it eligible for pod offloading in a remote cluster |
| webhook.pod.annotations | object | `{}` | webhook pod annotations |
| webhook.pod.labels | object | `{}` | webhook pod labels |
| webhook.service.annotations | object | `{}` | webhook service annotations |