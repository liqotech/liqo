## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| apiServer.address | string | `""` | The address that must be used to contact your API server, it needs to be reachable from the clusters that you will peer with (defaults to your master IP). |
| apiServer.trustedCA | bool | `false` | Indicates that the API Server is exposing a certificate issued by a trusted Certification Authority. |
| auth.config.addressOverride | string | `""` | Override the default address where your service is available, you should configure it if behind a reverse proxy or NAT. |
| auth.config.enableAuthentication | bool | `true` | Set to false to disable the authentication of discovered clusters. Note: use it only for testing installations. |
| auth.config.portOverride | string | `""` | Overrides the port where your service is available, you should configure it if behind a reverse proxy or NAT or using an Ingress with a port different from 443. |
| auth.imageName | string | `"ghcr.io/liqotech/auth-service"` | Image repository for the auth pod. |
| auth.ingress.annotations | object | `{}` | Annotations for the Auth ingress. |
| auth.ingress.class | string | `""` | Set your ingress class. |
| auth.ingress.enable | bool | `false` | Enable/disable the creation of the Ingress resource. |
| auth.ingress.host | string | `""` | Set the hostname for your ingress. |
| auth.ingress.port | int | `443` | Set port for your ingress. |
| auth.ingress.tlsSecretName | string | `""` | Override default (ChartName-auth) tls secretName. |
| auth.initContainer.imageName | string | `"ghcr.io/liqotech/cert-creator"` | Image repository for the init container of the auth pod. |
| auth.pod.annotations | object | `{}` | Annotations for the auth pod. |
| auth.pod.extraArgs | list | `[]` | Extra arguments for the auth pod. |
| auth.pod.labels | object | `{}` | Labels for the auth pod. |
| auth.pod.resources | object | `{"limits":{},"requests":{}}` | Resource requests and limits (https://kubernetes.io/docs/user-guide/compute-resources/) for the auth pod. |
| auth.service.annotations | object | `{}` | Annotations for the auth service. |
| auth.service.labels | object | `{}` | Labels for the auth service. |
| auth.service.loadBalancer | object | `{"allocateLoadBalancerNodePorts":"","ip":""}` | Options valid if service type is LoadBalancer. |
| auth.service.loadBalancer.allocateLoadBalancerNodePorts | string | `""` | Set to false if you expose the gateway service as LoadBalancer and you do not want to create also a NodePort associated to it (Note: this setting is useful only on cloud providers that support this feature). |
| auth.service.loadBalancer.ip | string | `""` | Override the IP here if service type is LoadBalancer and you want to use a specific IP address, e.g., because you want a static LB. |
| auth.service.nodePort | object | `{"port":""}` | Options valid if service type is NodePort. |
| auth.service.nodePort.port | string | `""` | Force the port used by the NodePort service. This value must be included between 30000 and 32767. |
| auth.service.port | int | `443` | Port used by the Authentication Service. |
| auth.service.type | string | `"LoadBalancer"` | Kubernetes service used to expose the Authentication Service. If you are exposing this service with an Ingress, you can change it to ClusterIP; if your cluster does not support LoadBalancer services, consider to switch it to NodePort. See https://doc.liqo.io/installation/ for more details. |
| auth.tls | bool | `true` | Enable TLS for the Authentication Service Pod (using a self-signed certificate). If you are exposing this service with an Ingress, consider to disable it or add the appropriate annotations to the Ingress resource. |
| awsConfig.accessKeyId | string | `""` | AccessKeyID for the Liqo user. |
| awsConfig.clusterName | string | `""` | Name of the EKS cluster. |
| awsConfig.region | string | `""` | AWS region where the clsuter is runnnig. |
| awsConfig.secretAccessKey | string | `""` | SecretAccessKey for the Liqo user. |
| common.affinity | object | `{}` | Affinity for all liqo pods, excluding virtual kubelet. |
| common.extraArgs | list | `[]` | Extra arguments for all liqo pods, excluding virtual kubelet. |
| common.nodeSelector | object | `{}` | NodeSelector for all liqo pods, excluding virtual kubelet. |
| common.tolerations | list | `[]` | Tolerations for all liqo pods, excluding virtual kubelet. |
| controllerManager.config.enableNodeFailureController | bool | `false` | Ensure offloaded pods running on a failed node are evicted and rescheduled on a healthy node, preventing them to remain in a terminating state indefinitely. This feature can be useful in case of remote node failure to guarantee better service continuity and to have the expected pods workload on the remote cluster. However, enabling this feature could produce zombies in the worker node, in case the node returns Ready again without a restart. |
| controllerManager.config.enableResourceEnforcement | bool | `false` | It enforces offerer-side that offloaded pods do not exceed offered resources (based on container limits). This feature is suggested to be enabled when consumer-side enforcement is not sufficient. It has the same tradeoffs of resource quotas (i.e, it requires all offloaded pods to have resource limits set). |
| controllerManager.config.offerUpdateThresholdPercentage | string | `""` | Threshold (in percentage) of the variation of resources that triggers a ResourceOffer update. E.g., when the available resources grow/decrease by X, a new ResourceOffer is generated. |
| controllerManager.config.resourcePluginAddress | string | `""` | The address of an external resource plugin service (see https://github.com/liqotech/liqo-resource-plugins for additional information), overriding the default resource computation logic based on the percentage of available resources. Leave it empty to use the standard local resource monitor. |
| controllerManager.config.resourceSharingPercentage | int | `30` | Percentage of available cluster resources that you are willing to share with foreign clusters. |
| controllerManager.imageName | string | `"ghcr.io/liqotech/liqo-controller-manager"` | Image repository for the controller-manager pod. |
| controllerManager.pod.annotations | object | `{}` | Annotations for the controller-manager pod. |
| controllerManager.pod.extraArgs | list | `[]` | Extra arguments for the controller-manager pod. |
| controllerManager.pod.labels | object | `{}` | Labels for the controller-manager pod. |
| controllerManager.pod.resources | object | `{"limits":{},"requests":{}}` | Resource requests and limits (https://kubernetes.io/docs/user-guide/compute-resources/) for the controller-manager pod. |
| controllerManager.replicas | int | `1` | The number of controller-manager instances to run, which can be increased for active/passive high availability. |
| crdReplicator.imageName | string | `"ghcr.io/liqotech/crd-replicator"` | Image repository for the crdReplicator pod. |
| crdReplicator.pod.annotations | object | `{}` | Annotations for the crdReplicator pod. |
| crdReplicator.pod.extraArgs | list | `[]` | Extra arguments for the crdReplicator pod. |
| crdReplicator.pod.labels | object | `{}` | Labels for the crdReplicator pod. |
| crdReplicator.pod.resources | object | `{"limits":{},"requests":{}}` | Resource requests and limits (https://kubernetes.io/docs/user-guide/compute-resources/) for the crdReplicator pod. |
| discovery.config.autojoin | bool | `true` | Automatically join discovered clusters. |
| discovery.config.clusterIDOverride | string | `""` | Specify an unique ID (must be a valid uuidv4) for your cluster, instead of letting helm generate it automatically at install time. You can generate it using the command: `uuidgen` This field is needed when using tools such as ArgoCD, since the helm lookup function is not supported and a new value would be generated at each deployment. |
| discovery.config.clusterLabels | object | `{}` | A set of labels that characterizes the local cluster when exposed remotely as a virtual node. It is suggested to specify the distinguishing characteristics that may be used to decide whether to offload pods on this cluster. |
| discovery.config.clusterName | string | `""` | Set a mnemonic name for your cluster. |
| discovery.config.enableAdvertisement | bool | `false` | Enable the mDNS advertisement on LANs, set to false to not be discoverable from other clusters in the same LAN. When this flag is 'false', the cluster can still receive the advertising from other (local) clusters, and automatically peer with them.  |
| discovery.config.enableDiscovery | bool | `false` | Enable the mDNS discovery on LANs, set to false to not look for other clusters available in the same LAN. Usually this feature should be active when you have multiple (tiny) clusters on the same LAN (e.g., multiple K3s running on individual devices); if your clusters operate on the big Internet, this feature is not needed and it can be turned off. |
| discovery.config.incomingPeeringEnabled | bool | `true` | Allow (by default) the remote clusters to establish a peering with our cluster. |
| discovery.config.ttl | int | `90` | Time-to-live before an automatically discovered clusters is deleted from the list of available ones if no longer announced (in seconds). |
| discovery.imageName | string | `"ghcr.io/liqotech/discovery"` | Image repository for the discovery pod. |
| discovery.pod.annotations | object | `{}` | Annotation for the discovery pod. |
| discovery.pod.extraArgs | list | `[]` | Extra arguments for the discovery pod. |
| discovery.pod.labels | object | `{}` | Labels for the discovery pod. |
| discovery.pod.resources | object | `{"limits":{},"requests":{}}` | Resource requests and limits (https://kubernetes.io/docs/user-guide/compute-resources/) for the discovery pod. |
| fullnameOverride | string | `""` | Override the standard full name used by Helm and associated to Kubernetes/Liqo resources. |
| gateway.config.addressOverride | string | `""` | Override the default address where your network gateway service is available. You should configure it if the network gateway is behind a reverse proxy or NAT. |
| gateway.config.listeningPort | int | `5871` | Port used by the network gateway. |
| gateway.config.portOverride | string | `""` | Overrides the port where your network gateway service is available. You should configure it if the network gateway is behind a reverse proxy or NAT and is different from the listening port. |
| gateway.config.wireguardImplementation | string | `"kernel"` | Implementation used by wireguard to establish the VPN tunnel between two clusters. Possible values are "userspace" and "kernel". Do not use "userspace" unless strictly necessary  (i.e., only if the Linux kernel does not support Wireguard). |
| gateway.imageName | string | `"ghcr.io/liqotech/liqonet"` | Image repository for the network gateway pod. |
| gateway.metrics.enabled | bool | `false` | Expose metrics about network traffic towards cluster peers. |
| gateway.metrics.port | int | `5872` | Port used to expose metrics. |
| gateway.metrics.service | object | `{"annotations":{},"labels":{}}` | Service used to expose metrics. |
| gateway.metrics.service.annotations | object | `{}` | Annotations for the metrics service. |
| gateway.metrics.service.labels | object | `{}` | Labels for the metrics service. |
| gateway.metrics.serviceMonitor.enabled | bool | `false` | Enable/Disable a Prometheus servicemonitor. Turn on this flag when the Prometheus Operator runs in your cluster; otherwise simply export the port above as an external endpoint. |
| gateway.metrics.serviceMonitor.interval | string | `""` | Customize service monitor requests interval. If empty, Prometheus uses the global scrape interval (https://github.com/prometheus-operator/prometheus-operator/blob/main/Documentation/api.md#endpoint). |
| gateway.metrics.serviceMonitor.labels | object | `{}` | Labels for the gateway servicemonitor. |
| gateway.metrics.serviceMonitor.scrapeTimeout | string | `""` | Customize service monitor scrape timeout. If empty, Prometheus uses the global scrape timeout (https://github.com/prometheus-operator/prometheus-operator/blob/main/Documentation/api.md#endpoint). |
| gateway.pod.annotations | object | `{}` | Annotations for the network gateway pod. |
| gateway.pod.extraArgs | list | `[]` | Extra arguments for the network gateway pod. |
| gateway.pod.labels | object | `{}` | Labels for the network gateway pod. |
| gateway.pod.resources | object | `{"limits":{},"requests":{}}` | Resource requests and limits (https://kubernetes.io/docs/user-guide/compute-resources/) for the network gateway pod. |
| gateway.replicas | int | `1` | The number of gateway instances to run. The gateway component supports active/passive high availability. Make sure that there are enough nodes to accommodate the replicas, because such pod has to run in the host network, hence no more than one replica can be scheduled on a given node. |
| gateway.service.annotations | object | `{}` | Annotations for the network gateway service. |
| gateway.service.labels | object | `{}` | Labels for the network gateway service. |
| gateway.service.loadBalancer | object | `{"allocateLoadBalancerNodePorts":"","ip":""}` | Options valid if service type is LoadBalancer. |
| gateway.service.loadBalancer.allocateLoadBalancerNodePorts | string | `""` | Set to false if you expose the gateway service as LoadBalancer and you do not want to create also a NodePort associated to it (Note: this setting is useful only on cloud providers that support this feature). |
| gateway.service.loadBalancer.ip | string | `""` | Override the IP here if service type is LoadBalancer and you want to use a specific IP address, e.g., because you want a static LB. |
| gateway.service.nodePort | object | `{"port":""}` | Options valid if service type is NodePort. |
| gateway.service.nodePort.port | string | `""` | Force the port used by the NodePort service. |
| gateway.service.type | string | `"LoadBalancer"` | Kubernetes service to be used to expose the network gateway pod. If you plan to use liqo over the Internet, consider to change this field to "LoadBalancer". Instead, if your nodes are directly reachable from the cluster you are peering to, you may change it to "NodePort". |
| metricAgent.config.timeout | object | `{"read":"30s","write":"30s"}` | Set the timeout for the metrics server. |
| metricAgent.enable | bool | `true` | Enable/Disable the virtual kubelet metric agent. This component aggregates all the kubelet-related metrics (e.g., CPU, RAM, etc) collected on the nodes that are used by a remote cluster peered with you, then exporting  the resulting values as a property of the virtual kubelet running on the remote cluster. |
| metricAgent.imageName | string | `"ghcr.io/liqotech/metric-agent"` | Image repository for the metricAgent pod. |
| metricAgent.initContainer.imageName | string | `"ghcr.io/liqotech/cert-creator"` | Image repository for the authentication init container for the metricAgent pod. |
| metricAgent.pod.annotations | object | `{}` | Annotations for the metricAgent pod. |
| metricAgent.pod.extraArgs | list | `[]` | Extra arguments for the metricAgent pod. |
| metricAgent.pod.labels | object | `{}` | Labels for the metricAgent pod. |
| metricAgent.pod.resources | object | `{"limits":{},"requests":{}}` | Resource requests and limits (https://kubernetes.io/docs/user-guide/compute-resources/) for the metricAgent pod. |
| nameOverride | string | `""` | Override the standard name used by Helm and associated to Kubernetes/Liqo resources. |
| networkManager.config.additionalPools | list | `[]` | Set of additional network pools to perform the automatic address mapping in Liqo. Network pools are used to map a cluster network into another one in order to prevent conflicts. Default set of network pools is: [10.0.0.0/8, 192.168.0.0/16, 172.16.0.0/12] |
| networkManager.config.podCIDR | string | `""` | The subnet used by the pods in your cluster, in CIDR notation (e.g., 10.0.0.0/16). |
| networkManager.config.reservedSubnets | list | `[]` | List of IP subnets that do not have to be used by Liqo. Liqo can perform automatic IP address remapping when a remote cluster is peering with you, e.g., in case IP address spaces (e.g., PodCIDR) overlaps. In order to prevent IP conflicting between locally used private subnets in your infrastructure and private subnets belonging to remote clusters you need tell liqo the subnets used in your cluster. E.g if your cluster nodes belong to the 192.168.2.0/24 subnet, then you should add that subnet to the reservedSubnets. PodCIDR and serviceCIDR used in the local cluster are automatically added to the reserved list. |
| networkManager.config.serviceCIDR | string | `""` | The subnet used by the services in you cluster, in CIDR notation (e.g., 172.16.0.0/16). |
| networkManager.externalIPAM.enabled | bool | `false` | Use an external IPAM to allocate the IP addresses for the pods. |
| networkManager.externalIPAM.url | string | `""` | The URL of the external IPAM. |
| networkManager.imageName | string | `"ghcr.io/liqotech/liqonet"` | Image repository for the networkManager pod. |
| networkManager.pod.annotations | object | `{}` | Annotations for the networkManager pod. |
| networkManager.pod.extraArgs | list | `[]` | Extra arguments for the networkManager pod. |
| networkManager.pod.labels | object | `{}` | Labels for the networkManager pod. |
| networkManager.pod.resources | object | `{"limits":{},"requests":{}}` | Resource requests and limits (https://kubernetes.io/docs/user-guide/compute-resources/) for the networkManager pod. |
| networking.internal | bool | `true` | Use the default Liqo network manager. |
| networking.iptables | object | `{"mode":"nf_tables"}` | Iptables configuration tuning. |
| networking.iptables.mode | string | `"nf_tables"` | Select the iptables mode to use. Possible values are "legacy" and "nf_tables". |
| networking.mtu | int | `1340` | Set the MTU for the interfaces managed by liqo: vxlan, tunnel and veth interfaces. The value is used by the gateway and route operators. The default value is configured to ensure correct behavior regardless of the combination of the underlying environments (e.g., cloud providers). This guarantees improved compatibility at the cost of possible limited performance drops. |
| networking.reflectIPs | bool | `true` | Reflect pod IPs and EnpointSlices to the remote clusters. |
| networking.securityMode | string | `"FullPodToPod"` | Select the mode to enforce security on connectivity among clusters. Possible values are "FullPodToPod" and "IntraClusterTrafficSegregation"  |
| openshiftConfig.enable | bool | `false` | Enable/Disable the OpenShift support, enabling Openshift-specific resources, and setting the pod security contexts in a way that is compatible with Openshift. |
| openshiftConfig.virtualKubeletSCCs | list | `["anyuid"]` | Security context configurations granted to the virtual kubelet in the local cluster. The configuration of one or more SCCs for the virtual kubelet is not strictly required, and privileges can be reduced in production environments. Still, the default configuration (i.e., anyuid) is suggested to prevent problems (i.e., the virtual kubelet fails to add the appropriate labels) when attempting to offload pods not managed by higher-level abstractions (e.g., Deployments), and not associated with a properly privileged service account. Indeed, "anyuid" is the SCC automatically associated with pods created by cluster administrators. Any pod granted a more privileged SCC and not linked to an adequately privileged service account will fail to be offloaded. |
| proxy.config.listeningPort | int | `8118` | Port used by the proxy pod. |
| proxy.imageName | string | `"ghcr.io/liqotech/proxy"` | Image repository for the proxy pod. |
| proxy.pod.annotations | object | `{}` | Annotations for the proxy pod. |
| proxy.pod.extraArgs | list | `[]` | Extra arguments for the proxy pod. |
| proxy.pod.labels | object | `{}` | Labels for the proxy pod. |
| proxy.pod.resources | object | `{"limits":{},"requests":{}}` | Resource requests and limits (https://kubernetes.io/docs/user-guide/compute-resources/) for the proxy pod. |
| proxy.service.annotations | object | `{}` |  |
| proxy.service.type | string | `"ClusterIP"` |  |
| pullPolicy | string | `"IfNotPresent"` | The pullPolicy for liqo pods. |
| reflection.configmap.type | string | `"DenyList"` | The type of reflection used for the configmaps reflector. Ammitted values: "DenyList", "AllowList". |
| reflection.configmap.workers | int | `3` | The number of workers used for the configmaps reflector. Set 0 to disable the reflection of configmaps. |
| reflection.endpointslice.workers | int | `10` | The number of workers used for the endpointslices reflector. Set 0 to disable the reflection of endpointslices. |
| reflection.event.type | string | `"DenyList"` | The type of reflection used for the events reflector. Ammitted values: "DenyList", "AllowList". |
| reflection.event.workers | int | `3` | The number of workers used for the events reflector. Set 0 to disable the reflection of events. |
| reflection.ingress.type | string | `"DenyList"` | The type of reflection used for the ingresses reflector. Ammitted values: "DenyList", "AllowList". |
| reflection.ingress.workers | int | `3` | The number of workers used for the ingresses reflector. Set 0 to disable the reflection of ingresses. |
| reflection.persistentvolumeclaim.workers | int | `3` | The number of workers used for the persistentvolumeclaims reflector. Set 0 to disable the reflection of persistentvolumeclaims. |
| reflection.pod.workers | int | `10` | The number of workers used for the pods reflector. Set 0 to disable the reflection of pods. |
| reflection.secret.type | string | `"DenyList"` | The type of reflection used for the secrets reflector. Ammitted values: "DenyList", "AllowList". |
| reflection.secret.workers | int | `3` | The number of workers used for the secrets reflector. Set 0 to disable the reflection of secrets. |
| reflection.service.type | string | `"DenyList"` | The type of reflection used for the services reflector. Ammitted values: "DenyList", "AllowList". |
| reflection.service.workers | int | `3` | The number of workers used for the services reflector. Set 0 to disable the reflection of services. |
| reflection.serviceaccount.workers | int | `3` | The number of workers used for the serviceaccounts reflector. Set 0 to disable the reflection of serviceaccounts. |
| reflection.skip.annotations | list | `["cloud.google.com/neg","cloud.google.com/neg-status","kubernetes.digitalocean.com/load-balancer-id","ingress.kubernetes.io/backends","ingress.kubernetes.io/forwarding-rule","ingress.kubernetes.io/target-proxy","ingress.kubernetes.io/url-map","metallb.universe.tf/address-pool","metallb.universe.tf/ip-allocated-from-pool","metallb.universe.tf/loadBalancerIPs"]` | List of annotations that must not be reflected on remote clusters. |
| reflection.skip.labels | list | `[]` | List of labels that must not be reflected on remote clusters. |
| route.imageName | string | `"ghcr.io/liqotech/liqonet"` | Image repository for the route pod. |
| route.pod.annotations | object | `{}` | Annotations for the route pod. |
| route.pod.extraArgs | list | `[]` | Extra arguments for the route pod. |
| route.pod.labels | object | `{}` | Labels for the route pod. |
| route.pod.resources | object | `{"limits":{},"requests":{}}` | Resource requests and limits (https://kubernetes.io/docs/user-guide/compute-resources/) for the route pod. |
| route.tolerations | list | `[]` | Extra tolerations for the route daemonset. |
| storage.enable | bool | `true` | Enable/Disable the liqo virtual storage class on the local cluster. You will be able to offload your persistent volumes, while other clusters will be able to schedule their persistent workloads on the current cluster. |
| storage.realStorageClassName | string | `""` | Name of the real storage class to use in the local cluster. |
| storage.storageNamespace | string | `"liqo-storage"` | Namespace where liqo will deploy specific PVCs. Internal parameter, do not change. |
| storage.virtualStorageClassName | string | `"liqo"` | Name to assign to the liqo virtual storage class. |
| tag | string | `""` | Images' tag to select a development version of liqo instead of a release |
| telemetry.config.schedule | string | `""` | Set the schedule of the telemetry collector CronJob. Consider setting this value on ArgoCD deployments to avoid randomization. |
| telemetry.enable | bool | `true` | Enable/Disable the telemetry collector. |
| telemetry.imageName | string | `"ghcr.io/liqotech/telemetry"` | Image repository for the telemetry pod. |
| telemetry.pod.annotations | object | `{}` | Annotations for the telemetry pod. |
| telemetry.pod.extraArgs | list | `[]` | Extra arguments for the telemetry pod. |
| telemetry.pod.labels | object | `{}` | Labels for the telemetry pod. |
| telemetry.pod.resources | object | `{"limits":{},"requests":{}}` | Resource requests and limits (https://kubernetes.io/docs/user-guide/compute-resources/) for the telemetry pod. |
| uninstaller.imageName | string | `"ghcr.io/liqotech/uninstaller"` | Image repository for the uninstaller pod. |
| uninstaller.pod.annotations | object | `{}` | Annotations for the uninstaller pod. |
| uninstaller.pod.extraArgs | list | `[]` | Extra arguments for the uninstaller pod. |
| uninstaller.pod.labels | object | `{}` | Labels for the uninstaller pod. |
| uninstaller.pod.resources | object | `{"limits":{},"requests":{}}` | Resource requests and limits (https://kubernetes.io/docs/user-guide/compute-resources/) for the uninstaller pod. |
| virtualKubelet.extra.annotations | object | `{}` | Annotations for the virtual kubelet pod. |
| virtualKubelet.extra.args | list | `[]` | Extra arguments virtual kubelet pod. |
| virtualKubelet.extra.labels | object | `{}` | Labels for the virtual kubelet pod. |
| virtualKubelet.extra.resources | object | `{"limits":{},"requests":{}}` | Resource requests and limits (https://kubernetes.io/docs/user-guide/compute-resources/) for the virtual kubelet pod. |
| virtualKubelet.imageName | string | `"ghcr.io/liqotech/virtual-kubelet"` | Image repository for the virtual kubelet. |
| virtualKubelet.metrics.enabled | bool | `false` | Enable/Disable to expose metrics about virtual kubelet resources. |
| virtualKubelet.metrics.podMonitor.enabled | bool | `false` | Enable/Disable the creation of a Prometheus podmonitor. Turn on this flag when the Prometheus Operator runs in your cluster; otherwise simply export the port above as an external endpoint. |
| virtualKubelet.metrics.podMonitor.interval | string | `""` | Setup pod monitor requests interval. If empty, Prometheus uses the global scrape interval (https://github.com/prometheus-operator/prometheus-operator/blob/main/Documentation/api.md#endpoint). |
| virtualKubelet.metrics.podMonitor.labels | object | `{}` | Labels for the virtualkubelet podmonitor. |
| virtualKubelet.metrics.podMonitor.scrapeTimeout | string | `""` | Setup pod monitor scrape timeout. If empty, Prometheus uses the global scrape timeout (https://github.com/prometheus-operator/prometheus-operator/blob/main/Documentation/api.md#endpoint). |
| virtualKubelet.metrics.port | int | `5872` | Port used to expose metrics. |
| virtualKubelet.virtualNode.extra.annotations | object | `{}` | Extra annotations for the virtual node. |
| virtualKubelet.virtualNode.extra.labels | object | `{}` | Extra labels for the virtual node. |
| webhook.failurePolicy | string | `"Fail"` | Webhook failure policy, either Ignore or Fail. |
| webhook.patch.image | string | `"k8s.gcr.io/ingress-nginx/kube-webhook-certgen:v1.1.1"` | Image used for the patch jobs to manage certificates. |
| webhook.port | int | `9443` | TCP port the webhook server binds to. |