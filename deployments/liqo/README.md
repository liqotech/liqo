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
| auth.config.allowEmptyToken | bool | `true` |  |
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
| auth.tls | bool | `true` |  |
| crdReplicator.config.resourcesToReplicate[0].group | string | `"net.liqo.io"` |  |
| crdReplicator.config.resourcesToReplicate[0].resource | string | `"networkconfigs"` |  |
| crdReplicator.config.resourcesToReplicate[0].version | string | `"v1alpha1"` |  |
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
| gateway.service.type | string | `"NodePort"` |  |
| nameOverride | string | `""` | liqo name override |
| networkManager.config.GKEProvider | bool | `false` | set this field to true if you are deploying liqo in GKE cluster |
| networkManager.config.podCIDR | string | `""` | the subnet used by the cluster for the pods, in CIDR notation |
| networkManager.config.reservedSubnets | list | `[]` | this field is used by the IPAM embedded in the liqo-networkManager.Subnets listed in this field are excluded from the list of possible subnets used for natting POD CIDR. Add here the subnets already used in your environment as a list in CIDR notation (e.g. [10.1.0.0/16, 10.200.1.0/24]). |
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