## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| apiServer.address | string | `""` | The address that must be used to contact your API server, it needs to be reachable from the clusters that you will peer with (defaults to your master IP) |
| apiServer.trustedCA | bool | `false` | Indicates that the API Server is exposing a certificate issued by a trusted Certification Authority |
| auth.config.enableAuthentication | bool | `true` | Set to false to disable the authentication of discovered clusters. NB: use it only for testing installations |
| auth.imageName | string | `"liqo/auth-service"` | auth image repository |
| auth.ingress.annotations | object | `{}` | Auth ingress annotations |
| auth.ingress.class | string | `""` | Set your ingress class |
| auth.ingress.enable | bool | `false` | Whether to enable the creation of the Ingress resource |
| auth.ingress.host | string | `""` | Set the hostname for your ingress |
| auth.initContainer.imageName | string | `"liqo/cert-creator"` | auth init container image repository |
| auth.pod.annotations | object | `{}` | auth pod annotations |
| auth.pod.extraArgs | list | `[]` | auth pod extra arguments |
| auth.pod.labels | object | `{}` | auth pod labels |
| auth.portOverride | string | `""` | Overrides the port where your service is available, you should configure it if behind a NAT or using an Ingress with a port different from 443. |
| auth.service.annotations | object | `{}` | auth service annotations |
| auth.service.type | string | `"LoadBalancer"` | The type of service used to expose the Authentication Service. If you are exposing this service with an Ingress, you can change it to ClusterIP; if your cluster does not support LoadBalancer services, consider to switch it to NodePort. See https://doc.liqo.io/installation/ for more details. |
| auth.tls | bool | `true` | Enable TLS for the Authentication Service Pod (using a self-signed certificate). If you are exposing this service with an Ingress consider to disable it or add the appropriate annotations to the Ingress resource. |
| awsConfig.accessKeyId | string | `""` | accessKeyID for the Liqo user |
| awsConfig.clusterName | string | `""` | name of the EKS cluster |
| awsConfig.region | string | `""` | AWS region where the clsuter is runnnig |
| awsConfig.secretAccessKey | string | `""` | secretAccessKey for the Liqo user |
| controllerManager.config.resourceSharingPercentage | int | `30` | It defines the percentage of available cluster resources that you are willing to share with foreign clusters. |
| controllerManager.imageName | string | `"liqo/liqo-controller-manager"` | controller-manager image repository |
| controllerManager.pod.annotations | object | `{}` | controller-manager pod annotations |
| controllerManager.pod.extraArgs | list | `[]` | controller-manager pod extra arguments |
| controllerManager.pod.labels | object | `{}` | controller-manager pod labels |
| crdReplicator.imageName | string | `"liqo/crd-replicator"` | crdReplicator image repository |
| crdReplicator.pod.annotations | object | `{}` | crdReplicator pod annotations |
| crdReplicator.pod.extraArgs | list | `[]` | crdReplicator pod extra arguments |
| crdReplicator.pod.labels | object | `{}` | crdReplicator pod labels |
| discovery.config.autojoin | bool | `true` | Automatically join discovered clusters |
| discovery.config.clusterLabels | object | `{}` | A set of labels which characterizes the local cluster when exposed remotely as a virtual node. It is suggested to specify the distinguishing characteristics that may be used to decide whether to offload pods on this cluster. |
| discovery.config.clusterName | string | `""` | Set a mnemonic name for your cluster |
| discovery.config.enableAdvertisement | bool | `false` | Enable the mDNS advertisement on LANs, set to false to not be discoverable from other clusters in the same LAN |
| discovery.config.enableDiscovery | bool | `false` | Enable the mDNS discovery on LANs, set to false to not look for other clusters available in the same LAN |
| discovery.config.incomingPeeringEnabled | bool | `true` | Allow (by default) the remote clusters to establish a peering with our cluster |
| discovery.config.ttl | int | `90` | Time-to-live before an automatically discovered clusters is deleted from the list of available ones if no longer announced (in seconds) |
| discovery.imageName | string | `"liqo/discovery"` | discovery image repository |
| discovery.pod.annotations | object | `{}` | discovery pod annotations |
| discovery.pod.extraArgs | list | `[]` | discovery pod extra arguments |
| discovery.pod.labels | object | `{}` | discovery pod labels |
| fullnameOverride | string | `""` | full liqo name override |
| gateway.config.listeningPort | int | `5871` | port used by the vpn tunnel. |
| gateway.imageName | string | `"liqo/liqonet"` | gateway image repository |
| gateway.pod.annotations | object | `{}` | gateway pod annotations |
| gateway.pod.extraArgs | list | `[]` | gateway pod extra arguments |
| gateway.pod.labels | object | `{}` | gateway pod labels |
| gateway.replicas | int | `1` | The number of gateway instances to run. The gateway component supports active/passive high availability. Make sure that there are enough nodes to accommodate the replicas, because being the instances in host network no more than one replica can be scheduled on a given node. |
| gateway.service.annotations | object | `{}` |  |
| gateway.service.type | string | `"LoadBalancer"` | If you plan to use liqo over the Internet, consider to change this field to "LoadBalancer". Instead, if your nodes are directly reachable from the cluster you are peering to, you may change it to "NodePort". |
| metricAgent.enable | bool | `true` | Enable the metric agent |
| metricAgent.imageName | string | `"liqo/metric-agent"` | metricAgent image repository |
| metricAgent.initContainer.imageName | string | `"liqo/cert-creator"` | auth init container image repository |
| metricAgent.pod.annotations | object | `{}` | metricAgent pod annotations |
| metricAgent.pod.extraArgs | list | `[]` | metricAgent pod extra arguments |
| metricAgent.pod.labels | object | `{}` | metricAgent pod labels |
| nameOverride | string | `""` | liqo name override |
| networkConfig.mtu | int | `1440` | set the mtu for the interfaces managed by liqo: vxlan, tunnel and veth interfaces The value is used by the gateway and route operators. |
| networkManager.config.additionalPools | list | `[]` | Set of additional network pools. Network pools are used to map a cluster network into another one in order to prevent conflicts. Default set of network pools is: [10.0.0.0/8, 192.168.0.0/16, 172.16.0.0/12] |
| networkManager.config.podCIDR | string | `""` | The subnet used by the cluster for the pods, in CIDR notation |
| networkManager.config.reservedSubnets | list | `[]` | Usually the IPs used for the pods in k8s clusters belong to private subnets. In order to prevent IP conflicting between locally used private subnets in your infrastructure and private subnets belonging to remote clusters you need tell liqo the subnets used in your cluster. E.g if your cluster nodes belong to the 192.168.2.0/24 subnet then you should add that subnet to the reservedSubnets. PodCIDR and serviceCIDR used in the local cluster are automatically added to the reserved list. |
| networkManager.config.serviceCIDR | string | `""` | The subnet used by the cluster for the services, in CIDR notation |
| networkManager.imageName | string | `"liqo/liqonet"` | networkManager image repository |
| networkManager.pod.annotations | object | `{}` | networkManager pod annotations |
| networkManager.pod.extraArgs | list | `[]` | networkManager pod extra arguments |
| networkManager.pod.labels | object | `{}` | networkManager pod labels |
| openshiftConfig.enable | bool | `false` | enable the OpenShift support |
| pullPolicy | string | `"IfNotPresent"` | The pullPolicy for liqo pods |
| route.imageName | string | `"liqo/liqonet"` | route image repository |
| route.pod.annotations | object | `{}` | route pod annotations |
| route.pod.extraArgs | list | `[]` | route pod extra arguments |
| route.pod.labels | object | `{}` | route pod labels |
| storage.enable | bool | `true` | enable the liqo virtual storage class on the local cluster. You will be able to offload your persistent volumes and other clusters will be able to schedule their persistent workloads on the current cluster. |
| storage.realStorageClassName | string | `""` | name of the real storage class to use in the local cluster |
| storage.storageNamespace | string | `"liqo-storage"` | namespace where liqo will deploy specific PVCs |
| storage.virtualStorageClassName | string | `"liqo"` | name to assign to the liqo virtual storage class |
| tag | string | `""` | Images' tag to select a development version of liqo instead of a release |
| virtualKubelet.extra.annotations | object | `{}` | virtual kubelet pod extra annotations |
| virtualKubelet.extra.args | list | `[]` | virtual kubelet pod extra arguments |
| virtualKubelet.extra.labels | object | `{}` | virtual kubelet pod extra labels |
| virtualKubelet.imageName | string | `"liqo/virtual-kubelet"` | virtual kubelet image repository |
| virtualKubelet.initContainer.imageName | string | `"liqo/init-virtual-kubelet"` | virtual kubelet init container image repository |
| virtualKubelet.virtualNode.extra.annotations | object | `{}` | virtual node extra annotations |
| virtualKubelet.virtualNode.extra.labels | object | `{}` | virtual node extra labels |
| webhook.imageName | string | `"liqo/liqo-webhook"` | webhook image repository |
| webhook.initContainer.imageName | string | `"liqo/webhook-configuration"` | webhook init container image repository |
| webhook.mutatingWebhookConfiguration.annotations | object | `{}` | mutatingWebhookConfiguration annotations |
| webhook.pod.annotations | object | `{}` | webhook pod annotations |
| webhook.pod.extraArgs | list | `[]` | webhook pod extra arguments |
| webhook.pod.labels | object | `{}` | webhook pod labels |
| webhook.service.annotations | object | `{}` | webhook service annotations |