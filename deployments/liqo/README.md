## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| apiServer.address | string | `""` | The address that must be used to contact your API server, it needs to be reachable from the clusters that you will peer with (defaults to your master IP) |
| apiServer.trustedCA | bool | `false` | Indicates that the API Server is exposing a certificate issued by a trusted Certification Authority |
| auth.config.addressOverride | string | `""` | Override the default address where your service is available, you should configure it if behind a reverse proxy or NAT. |
| auth.config.enableAuthentication | bool | `true` | Set to false to disable the authentication of discovered clusters. NB: use it only for testing installations |
| auth.config.portOverride | string | `""` | Overrides the port where your service is available, you should configure it if behind a reverse proxy or NAT or using an Ingress with a port different from 443. |
| auth.imageName | string | `"ghcr.io/liqotech/auth-service"` | auth image repository |
| auth.ingress.annotations | object | `{}` | Auth ingress annotations |
| auth.ingress.class | string | `""` | Set your ingress class |
| auth.ingress.enable | bool | `false` | Whether to enable the creation of the Ingress resource |
| auth.ingress.host | string | `""` | Set the hostname for your ingress |
| auth.initContainer.imageName | string | `"ghcr.io/liqotech/cert-creator"` | auth init container image repository |
| auth.pod.annotations | object | `{}` | auth pod annotations |
| auth.pod.extraArgs | list | `[]` | auth pod extra arguments |
| auth.pod.labels | object | `{}` | auth pod labels |
| auth.pod.resources | object | `{"limits":{},"requests":{}}` | auth pod containers' resource requests and limits (https://kubernetes.io/docs/user-guide/compute-resources/) |
| auth.service.annotations | object | `{}` | auth service annotations |
| auth.service.type | string | `"LoadBalancer"` | The type of service used to expose the Authentication Service. If you are exposing this service with an Ingress, you can change it to ClusterIP; if your cluster does not support LoadBalancer services, consider to switch it to NodePort. See https://doc.liqo.io/installation/ for more details. |
| auth.tls | bool | `true` | Enable TLS for the Authentication Service Pod (using a self-signed certificate). If you are exposing this service with an Ingress consider to disable it or add the appropriate annotations to the Ingress resource. |
| awsConfig.accessKeyId | string | `""` | accessKeyID for the Liqo user |
| awsConfig.clusterName | string | `""` | name of the EKS cluster |
| awsConfig.region | string | `""` | AWS region where the clsuter is runnnig |
| awsConfig.secretAccessKey | string | `""` | secretAccessKey for the Liqo user |
| controllerManager.config.enableResourceEnforcement | bool | `false` | It enforces offerer-side that offloaded pods do not exceed offered resources (based on container limits). This feature is suggested to be enabled when consumer-side enforcement is not sufficient. It has the same tradeoffs of resource quotas (i.e, it requires all offloaded pods to have resource limits set). |
| controllerManager.config.offerUpdateThresholdPercentage | string | `""` | the threshold (in percentage) of resources quantity variation which triggers a ResourceOffer update. |
| controllerManager.config.resourcePluginAddress | string | `""` | The address of an external resource plugin service (see https://github.com/liqotech/liqo-resource-plugins for additional information), overriding the default resource computation logic based on the percentage of available resources. Leave it empty to use the standard local resource monitor. |
| controllerManager.config.resourceSharingPercentage | int | `30` | It defines the percentage of available cluster resources that you are willing to share with foreign clusters. |
| controllerManager.imageName | string | `"ghcr.io/liqotech/liqo-controller-manager"` | controller-manager image repository |
| controllerManager.pod.annotations | object | `{}` | controller-manager pod annotations |
| controllerManager.pod.extraArgs | list | `[]` | controller-manager pod extra arguments |
| controllerManager.pod.labels | object | `{}` | controller-manager pod labels |
| controllerManager.pod.resources | object | `{"limits":{},"requests":{}}` | controller-manager pod containers' resource requests and limits (https://kubernetes.io/docs/user-guide/compute-resources/) |
| controllerManager.replicas | int | `1` | The number of controller-manager instances to run, which can be increased for active/passive high availability. |
| crdReplicator.imageName | string | `"ghcr.io/liqotech/crd-replicator"` | crdReplicator image repository |
| crdReplicator.pod.annotations | object | `{}` | crdReplicator pod annotations |
| crdReplicator.pod.extraArgs | list | `[]` | crdReplicator pod extra arguments |
| crdReplicator.pod.labels | object | `{}` | crdReplicator pod labels |
| crdReplicator.pod.resources | object | `{"limits":{},"requests":{}}` | crdReplicator pod containers' resource requests and limits (https://kubernetes.io/docs/user-guide/compute-resources/) |
| discovery.config.autojoin | bool | `true` | Automatically join discovered clusters |
| discovery.config.clusterIDOverride | string | `""` | Specify an unique ID (must be a valid uuidv4) for your cluster, instead of letting helm generate it automatically at install time. You can generate it using the command: `uuidgen` Setting this field is necessary when using tools such as ArgoCD, since the helm lookup function is not supported and a new value would be generated at each deployment. |
| discovery.config.clusterLabels | object | `{}` | A set of labels which characterizes the local cluster when exposed remotely as a virtual node. It is suggested to specify the distinguishing characteristics that may be used to decide whether to offload pods on this cluster. |
| discovery.config.clusterName | string | `""` | Set a mnemonic name for your cluster |
| discovery.config.enableAdvertisement | bool | `false` | Enable the mDNS advertisement on LANs, set to false to not be discoverable from other clusters in the same LAN |
| discovery.config.enableDiscovery | bool | `false` | Enable the mDNS discovery on LANs, set to false to not look for other clusters available in the same LAN |
| discovery.config.incomingPeeringEnabled | bool | `true` | Allow (by default) the remote clusters to establish a peering with our cluster |
| discovery.config.ttl | int | `90` | Time-to-live before an automatically discovered clusters is deleted from the list of available ones if no longer announced (in seconds) |
| discovery.imageName | string | `"ghcr.io/liqotech/discovery"` | discovery image repository |
| discovery.pod.annotations | object | `{}` | discovery pod annotations |
| discovery.pod.extraArgs | list | `[]` | discovery pod extra arguments |
| discovery.pod.labels | object | `{}` | discovery pod labels |
| discovery.pod.resources | object | `{"limits":{},"requests":{}}` | discovery pod containers' resource requests and limits (https://kubernetes.io/docs/user-guide/compute-resources/) |
| fullnameOverride | string | `""` | full liqo name override |
| gateway.config.addressOverride | string | `""` | Override the default address where your service is available, you should configure it if behind a reverse proxy or NAT. |
| gateway.config.listeningPort | int | `5871` | port used by the vpn tunnel. |
| gateway.config.portOverride | string | `""` | Overrides the port where your service is available, you should configure it if behind a reverse proxy or NAT and is different from the listening port. |
| gateway.imageName | string | `"ghcr.io/liqotech/liqonet"` | gateway image repository |
| gateway.metrics.enabled | bool | `false` | expose metrics about network traffic towards cluster peers. |
| gateway.metrics.port | int | `5872` | port used to expose metrics. |
| gateway.metrics.serviceMonitor.enabled | bool | `false` | create a prometheus servicemonitor. |
| gateway.metrics.serviceMonitor.interval | string | `""` | setup service monitor requests interval. If empty, Prometheus uses the global scrape interval. ref: https://github.com/prometheus-operator/prometheus-operator/blob/main/Documentation/api.md#endpoint |
| gateway.metrics.serviceMonitor.scrapeTimeout | string | `""` | setup service monitor scrape timeout. If empty, Prometheus uses the global scrape timeout. ref: https://github.com/prometheus-operator/prometheus-operator/blob/main/Documentation/api.md#endpoint |
| gateway.pod.annotations | object | `{}` | gateway pod annotations |
| gateway.pod.extraArgs | list | `[]` | gateway pod extra arguments |
| gateway.pod.labels | object | `{}` | gateway pod labels |
| gateway.pod.resources | object | `{"limits":{},"requests":{}}` | gateway pod containers' resource requests and limits (https://kubernetes.io/docs/user-guide/compute-resources/) |
| gateway.replicas | int | `1` | The number of gateway instances to run. The gateway component supports active/passive high availability. Make sure that there are enough nodes to accommodate the replicas, because being the instances in host network no more than one replica can be scheduled on a given node. |
| gateway.service.annotations | object | `{}` |  |
| gateway.service.type | string | `"LoadBalancer"` | If you plan to use liqo over the Internet, consider to change this field to "LoadBalancer". Instead, if your nodes are directly reachable from the cluster you are peering to, you may change it to "NodePort". |
| metricAgent.enable | bool | `true` | Enable the metric agent |
| metricAgent.imageName | string | `"ghcr.io/liqotech/metric-agent"` | metricAgent image repository |
| metricAgent.initContainer.imageName | string | `"ghcr.io/liqotech/cert-creator"` | auth init container image repository |
| metricAgent.pod.annotations | object | `{}` | metricAgent pod annotations |
| metricAgent.pod.extraArgs | list | `[]` | metricAgent pod extra arguments |
| metricAgent.pod.labels | object | `{}` | metricAgent pod labels |
| metricAgent.pod.resources | object | `{"limits":{},"requests":{}}` | metricAgent pod containers' resource requests and limits (https://kubernetes.io/docs/user-guide/compute-resources/) |
| nameOverride | string | `""` | liqo name override |
| networkConfig.mtu | int | `1340` | set the mtu for the interfaces managed by liqo: vxlan, tunnel and veth interfaces The value is used by the gateway and route operators. The default value is configured to ensure correct functioning regardless of the combination of the underlying environments (e.g., cloud providers). This guarantees improved compatibility at the cost of possible limited performance drops. |
| networkManager.config.additionalPools | list | `[]` | Set of additional network pools. Network pools are used to map a cluster network into another one in order to prevent conflicts. Default set of network pools is: [10.0.0.0/8, 192.168.0.0/16, 172.16.0.0/12] |
| networkManager.config.podCIDR | string | `""` | The subnet used by the cluster for the pods, in CIDR notation |
| networkManager.config.reservedSubnets | list | `[]` | Usually the IPs used for the pods in k8s clusters belong to private subnets. In order to prevent IP conflicting between locally used private subnets in your infrastructure and private subnets belonging to remote clusters you need tell liqo the subnets used in your cluster. E.g if your cluster nodes belong to the 192.168.2.0/24 subnet then you should add that subnet to the reservedSubnets. PodCIDR and serviceCIDR used in the local cluster are automatically added to the reserved list. |
| networkManager.config.serviceCIDR | string | `""` | The subnet used by the cluster for the services, in CIDR notation |
| networkManager.imageName | string | `"ghcr.io/liqotech/liqonet"` | networkManager image repository |
| networkManager.pod.annotations | object | `{}` | networkManager pod annotations |
| networkManager.pod.extraArgs | list | `[]` | networkManager pod extra arguments |
| networkManager.pod.labels | object | `{}` | networkManager pod labels |
| networkManager.pod.resources | object | `{"limits":{},"requests":{}}` | networkManager pod containers' resource requests and limits (https://kubernetes.io/docs/user-guide/compute-resources/) |
| openshiftConfig.enable | bool | `false` | enable the OpenShift support |
| openshiftConfig.virtualKubeletSCCs | list | `["anyuid"]` | the security context configurations granted to the virtual kubelet in the local cluster. The configuration of one or more SCCs for the virtual kubelet is not strictly required, and privileges can be reduced in production environments. Still, the default configuration (i.e., anyuid) is suggested to prevent problems (i.e., the virtual kubelet fails to add the appropriate labels) when attempting to offload pods not managed by higher-level abstractions (e.g., Deployments), and not associated with a properly privileged service account. Indeed, "anyuid" is the SCC automatically associated with pods created by cluster administrators. Any pod granted a more privileged SCC and not linked to an adequately privileged service account will fail to be offloaded. |
| proxy.config.listeningPort | int | `8118` | port used by envoy proxy |
| proxy.imageName | string | `"envoyproxy/envoy:v1.21.0"` | proxy image repository |
| proxy.pod.annotations | object | `{}` | proxy pod annotations |
| proxy.pod.extraArgs | list | `[]` | proxy pod extra arguments |
| proxy.pod.labels | object | `{}` | proxy pod labels |
| proxy.pod.resources | object | `{"limits":{},"requests":{}}` | proxy pod containers' resource requests and limits (https://kubernetes.io/docs/user-guide/compute-resources/) |
| proxy.service.annotations | object | `{}` |  |
| proxy.service.type | string | `"ClusterIP"` |  |
| pullPolicy | string | `"IfNotPresent"` | The pullPolicy for liqo pods |
| route.imageName | string | `"ghcr.io/liqotech/liqonet"` | route image repository |
| route.pod.annotations | object | `{}` | route pod annotations |
| route.pod.extraArgs | list | `[]` | route pod extra arguments |
| route.pod.labels | object | `{}` | route pod labels |
| route.pod.resources | object | `{"limits":{},"requests":{}}` | route pod containers' resource requests and limits (https://kubernetes.io/docs/user-guide/compute-resources/) |
| storage.enable | bool | `true` | enable the liqo virtual storage class on the local cluster. You will be able to offload your persistent volumes and other clusters will be able to schedule their persistent workloads on the current cluster. |
| storage.realStorageClassName | string | `""` | name of the real storage class to use in the local cluster |
| storage.storageNamespace | string | `"liqo-storage"` | namespace where liqo will deploy specific PVCs |
| storage.virtualStorageClassName | string | `"liqo"` | name to assign to the liqo virtual storage class |
| tag | string | `""` | Images' tag to select a development version of liqo instead of a release |
| telemetry.config.schedule | string | `""` | Set the schedule of the telemetry collector CronJob |
| telemetry.enable | bool | `true` | Enable the telemetry collector |
| telemetry.imageName | string | `"ghcr.io/liqotech/telemetry"` | telemetry image repository |
| telemetry.pod.annotations | object | `{}` | telemetry pod annotations |
| telemetry.pod.extraArgs | list | `[]` | telemetry pod extra arguments |
| telemetry.pod.labels | object | `{}` | telemetry pod labels |
| telemetry.pod.resources | object | `{"limits":{},"requests":{}}` | telemetry pod containers' resource requests and limits (https://kubernetes.io/docs/user-guide/compute-resources/) |
| uninstaller.imageName | string | `"ghcr.io/liqotech/uninstaller"` | uninstaller image repository |
| uninstaller.pod.annotations | object | `{}` | uninstaller pod annotations |
| uninstaller.pod.extraArgs | list | `[]` | uninstaller pod extra arguments |
| uninstaller.pod.labels | object | `{}` | uninstaller pod labels |
| uninstaller.pod.resources | object | `{"limits":{},"requests":{}}` | uninstaller pod containers' resource requests and limits (https://kubernetes.io/docs/user-guide/compute-resources/) |
| virtualKubelet.extra.annotations | object | `{}` | virtual kubelet pod extra annotations |
| virtualKubelet.extra.args | list | `[]` | virtual kubelet pod extra arguments |
| virtualKubelet.extra.labels | object | `{}` | virtual kubelet pod extra labels |
| virtualKubelet.extra.resources | object | `{"limits":{},"requests":{}}` | virtual kubelet pod containers' resource requests and limits (https://kubernetes.io/docs/user-guide/compute-resources/) |
| virtualKubelet.imageName | string | `"ghcr.io/liqotech/virtual-kubelet"` | virtual kubelet image repository |
| virtualKubelet.virtualNode.extra.annotations | object | `{}` | virtual node extra annotations |
| virtualKubelet.virtualNode.extra.labels | object | `{}` | virtual node extra labels |
| webhook.failurePolicy | string | `"Fail"` | the webhook failure policy, among Ignore and Fail |
| webhook.patch.image | string | `"k8s.gcr.io/ingress-nginx/kube-webhook-certgen:v1.1.1"` | the image used for the patch jobs to manage certificates |
| webhook.port | int | `9443` | the port the webhook server binds to |