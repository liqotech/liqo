## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| apiServer.address | string | `""` | The address that must be used to contact your API server, it needs to be reachable from the clusters that you will peer with (defaults to your master IP). |
| apiServer.ca | string | `""` | The CA certificate used to issue x509 user certificates for the API server (base64). Leave it empty to use the default CA. |
| apiServer.trustedCA | bool | `false` | Indicates that the API Server is exposing a certificate issued by a trusted Certification Authority. |
| authentication.awsConfig.accessKeyId | string | `""` | AccessKeyID for the Liqo user. |
| authentication.awsConfig.clusterName | string | `""` | Name of the EKS cluster. |
| authentication.awsConfig.region | string | `""` | AWS region where the clsuter is runnnig. |
| authentication.awsConfig.secretAccessKey | string | `""` | SecretAccessKey for the Liqo user. |
| authentication.awsConfig.useExistingSecret | bool | `false` | Use an existing secret to configure the AWS credentials. |
| authentication.enabled | bool | `true` | Enable/Disable the authentication module. |
| common.affinity | object | `{}` | Affinity for all liqo pods, excluding virtual kubelet. |
| common.extraArgs | list | `[]` | Extra arguments for all liqo pods, excluding virtual kubelet. |
| common.nodeSelector | object | `{}` | NodeSelector for all liqo pods, excluding virtual kubelet. |
| common.tolerations | list | `[]` | Tolerations for all liqo pods, excluding virtual kubelet. |
| controllerManager.config.enableNodeFailureController | bool | `false` | Ensure offloaded pods running on a failed node are evicted and rescheduled on a healthy node, preventing them to remain in a terminating state indefinitely. This feature can be useful in case of remote node failure to guarantee better service continuity and to have the expected pods workload on the remote cluster. However, enabling this feature could produce zombies in the worker node, in case the node returns Ready again without a restart. |
| controllerManager.config.enableResourceEnforcement | bool | `false` | It enforces offerer-side that offloaded pods do not exceed offered resources (based on container limits). This feature is suggested to be enabled when consumer-side enforcement is not sufficient. It has the same tradeoffs of resource quotas (i.e, it requires all offloaded pods to have resource limits set). |
| controllerManager.image.name | string | `"ghcr.io/liqotech/liqo-controller-manager"` | Image repository for the controller-manager pod. |
| controllerManager.image.version | string | `""` | Custom version for the controller-manager image. If not specified, the global tag is used. |
| controllerManager.metrics.service | object | `{"annotations":{},"labels":{}}` | Service used to expose metrics. |
| controllerManager.metrics.service.annotations | object | `{}` | Annotations for the metrics service. |
| controllerManager.metrics.service.labels | object | `{}` | Labels for the metrics service. |
| controllerManager.metrics.serviceMonitor.enabled | bool | `false` | Enable/Disable a Prometheus servicemonitor. Turn on this flag when the Prometheus Operator runs in your cluster |
| controllerManager.metrics.serviceMonitor.interval | string | `""` | Customize service monitor requests interval. If empty, Prometheus uses the global scrape interval (https://github.com/prometheus-operator/prometheus-operator/blob/main/Documentation/api.md#endpoint). |
| controllerManager.metrics.serviceMonitor.labels | object | `{}` | Labels for the gateway servicemonitor. |
| controllerManager.metrics.serviceMonitor.scrapeTimeout | string | `""` | Customize service monitor scrape timeout. If empty, Prometheus uses the global scrape timeout (https://github.com/prometheus-operator/prometheus-operator/blob/main/Documentation/api.md#endpoint). |
| controllerManager.pod.annotations | object | `{}` | Annotations for the controller-manager pod. |
| controllerManager.pod.extraArgs | list | `[]` | Extra arguments for the controller-manager pod. |
| controllerManager.pod.labels | object | `{}` | Labels for the controller-manager pod. |
| controllerManager.pod.priorityClassName | string | `""` | PriorityClassName (https://kubernetes.io/docs/concepts/scheduling-eviction/pod-priority-preemption/#pod-priority) for the controller-manager pod. |
| controllerManager.pod.resources | object | `{"limits":{},"requests":{}}` | Resource requests and limits (https://kubernetes.io/docs/user-guide/compute-resources/) for the controller-manager pod. |
| controllerManager.replicas | int | `1` | The number of controller-manager instances to run, which can be increased for active/passive high availability. |
| crdReplicator.image.name | string | `"ghcr.io/liqotech/crd-replicator"` | Image repository for the crdReplicator pod. |
| crdReplicator.image.version | string | `""` | Custom version for the crdReplicator image. If not specified, the global tag is used. |
| crdReplicator.metrics.podMonitor.enabled | bool | `false` | Enable/Disable the creation of a Prometheus podmonitor. Turn on this flag when the Prometheus Operator runs in your cluster |
| crdReplicator.metrics.podMonitor.interval | string | `""` | Setup pod monitor requests interval. If empty, Prometheus uses the global scrape interval (https://github.com/prometheus-operator/prometheus-operator/blob/main/Documentation/api.md#endpoint). |
| crdReplicator.metrics.podMonitor.labels | object | `{}` | Labels for the crdReplicator podmonitor. |
| crdReplicator.metrics.podMonitor.scrapeTimeout | string | `""` | Setup pod monitor scrape timeout. If empty, Prometheus uses the global scrape timeout (https://github.com/prometheus-operator/prometheus-operator/blob/main/Documentation/api.md#endpoint). |
| crdReplicator.pod.annotations | object | `{}` | Annotations for the crdReplicator pod. |
| crdReplicator.pod.extraArgs | list | `[]` | Extra arguments for the crdReplicator pod. |
| crdReplicator.pod.labels | object | `{}` | Labels for the crdReplicator pod. |
| crdReplicator.pod.priorityClassName | string | `""` | PriorityClassName (https://kubernetes.io/docs/concepts/scheduling-eviction/pod-priority-preemption/#pod-priority) for the crdReplicator pod. |
| crdReplicator.pod.resources | object | `{"limits":{},"requests":{}}` | Resource requests and limits (https://kubernetes.io/docs/user-guide/compute-resources/) for the crdReplicator pod. |
| discovery.config.autojoin | bool | `true` | Automatically join discovered clusters. |
| discovery.config.clusterIDOverride | string | `""` | Specify an unique ID (must be a valid uuidv4) for your cluster, instead of letting helm generate it automatically at install time. You can generate it using the command: `uuidgen` This field is needed when using tools such as ArgoCD, since the helm lookup function is not supported and a new value would be generated at each deployment. |
| discovery.config.clusterLabels | object | `{}` | A set of labels that characterizes the local cluster when exposed remotely as a virtual node. It is suggested to specify the distinguishing characteristics that may be used to decide whether to offload pods on this cluster. |
| discovery.config.clusterName | string | `""` | Set a mnemonic name for your cluster. |
| discovery.config.enableAdvertisement | bool | `false` | Enable the mDNS advertisement on LANs, set to false to not be discoverable from other clusters in the same LAN. When this flag is 'false', the cluster can still receive the advertising from other (local) clusters, and automatically peer with them.  |
| discovery.config.enableDiscovery | bool | `false` | Enable the mDNS discovery on LANs, set to false to not look for other clusters available in the same LAN. Usually this feature should be active when you have multiple (tiny) clusters on the same LAN (e.g., multiple K3s running on individual devices); if your clusters operate on the big Internet, this feature is not needed and it can be turned off. |
| discovery.config.incomingPeeringEnabled | bool | `true` | Allow (by default) the remote clusters to establish a peering with our cluster. |
| discovery.config.ttl | int | `90` | Time-to-live before an automatically discovered clusters is deleted from the list of available ones if no longer announced (in seconds). |
| discovery.image.name | string | `"ghcr.io/liqotech/discovery"` | Image repository for the discovery pod. |
| discovery.image.version | string | `""` | Custom version for the discovery image. If not specified, the global tag is used. |
| discovery.pod.annotations | object | `{}` | Annotation for the discovery pod. |
| discovery.pod.extraArgs | list | `[]` | Extra arguments for the discovery pod. |
| discovery.pod.labels | object | `{}` | Labels for the discovery pod. |
| discovery.pod.priorityClassName | string | `""` | PriorityClassName (https://kubernetes.io/docs/concepts/scheduling-eviction/pod-priority-preemption/#pod-priority) for the discovery pod. |
| discovery.pod.resources | object | `{"limits":{},"requests":{}}` | Resource requests and limits (https://kubernetes.io/docs/user-guide/compute-resources/) for the discovery pod. |
| fabric.config.fullMasquerade | bool | `false` | Enabe/Disable the full masquerade mode for the fabric pod. It means that all traffic will be masquerade using the first external cidr IP, instead of using the pod IP. Full masquerade is useful when the cluster nodeports uses a PodCIDR IP to masqerade the incoming traffic. IMPORTANT: Please consider that enabling this feature will masquerade the source IP of traffic towards a remote cluster,  making impossible for a pod that receives the traffic to know the original source IP.  |
| fabric.config.gatewayMasqueradeBypass | bool | `false` | Enable/Disable the masquerade bypass for the gateway pods. It means that the packets from gateway pods will not be masqueraded from the host where the pod is scheduled. This is useful in scenarios where CNIs masquerade the traffic from pod to nodes. For example this is required when using the Azure CNI or Kindnet. |
| fabric.image.name | string | `"ghcr.io/liqotech/fabric"` | Image repository for the fabric pod. |
| fabric.image.version | string | `""` | Custom version for the fabric image. If not specified, the global tag is used. |
| fabric.pod.annotations | object | `{}` | Annotations for the fabric pod. |
| fabric.pod.extraArgs | list | `[]` | Extra arguments for the fabric pod. |
| fabric.pod.labels | object | `{}` | Labels for the fabric pod. |
| fabric.pod.priorityClassName | string | `""` | PriorityClassName (https://kubernetes.io/docs/concepts/scheduling-eviction/pod-priority-preemption/#pod-priority) for the fabric pod. |
| fabric.pod.resources | object | `{"limits":{},"requests":{}}` | Resource requests and limits (https://kubernetes.io/docs/user-guide/compute-resources/) for the fabric pod. |
| fabric.tolerations | list | `[]` | Extra tolerations for the fabric daemonset. |
| fullnameOverride | string | `""` | Override the standard full name used by Helm and associated to Kubernetes/Liqo resources. |
| ipam.additionalPools | list | `[]` | Set of additional network pools to perform the automatic address mapping in Liqo. Network pools are used to map a cluster network into another one in order to prevent conflicts. Default set of network pools is: [10.0.0.0/8, 192.168.0.0/16, 172.16.0.0/12] |
| ipam.external.enabled | bool | `false` | Use an external IPAM to allocate the IP addresses for the pods. Enabling it will disable the internal IPAM. |
| ipam.external.url | string | `""` | The URL of the external IPAM. |
| ipam.externalCIDR | string | `"10.70.0.0/16"` | The subnet used for the external CIDR. |
| ipam.internal.enabled | bool | `true` | Use the default Liqo IPAM. |
| ipam.internal.image.name | string | `"ghcr.io/liqotech/ipam"` | Image repository for the IPAM pod. |
| ipam.internal.image.version | string | `""` | Custom version for the IPAM image. If not specified, the global tag is used. |
| ipam.internal.pod.annotations | object | `{}` | Annotations for the IPAM pod. |
| ipam.internal.pod.extraArgs | list | `[]` | Extra arguments for the IPAM pod. |
| ipam.internal.pod.labels | object | `{}` | Labels for the IPAM pod. |
| ipam.internal.pod.priorityClassName | string | `""` | PriorityClassName (https://kubernetes.io/docs/concepts/scheduling-eviction/pod-priority-preemption/#pod-priority) for the IPAM pod. |
| ipam.internal.pod.resources | object | `{"limits":{},"requests":{}}` | Resource requests and limits (https://kubernetes.io/docs/user-guide/compute-resources/) for the IPAM pod. |
| ipam.internal.replicas | int | `1` | The number of IPAM instances to run, which can be increased for active/passive high availability. |
| ipam.internalCIDR | string | `"10.80.0.0/16"` | The subnet used for the internal CIDR. These IPs are assigned to the Liqo internal-network interfaces. |
| ipam.podCIDR | string | `""` | The subnet used by the pods in your cluster, in CIDR notation (e.g., 10.0.0.0/16). |
| ipam.reservedSubnets | list | `[]` | List of IP subnets that do not have to be used by Liqo. Liqo can perform automatic IP address remapping when a remote cluster is peering with you, e.g., in case IP address spaces (e.g., PodCIDR) overlaps. In order to prevent IP conflicting between locally used private subnets in your infrastructure and private subnets belonging to remote clusters you need tell liqo the subnets used in your cluster. E.g if your cluster nodes belong to the 192.168.2.0/24 subnet, then you should add that subnet to the reservedSubnets. PodCIDR and serviceCIDR used in the local cluster are automatically added to the reserved list. |
| ipam.serviceCIDR | string | `""` | The subnet used by the services in you cluster, in CIDR notation (e.g., 172.16.0.0/16). |
| metricAgent.config.timeout | object | `{"read":"30s","write":"30s"}` | Set the timeout for the metrics server. |
| metricAgent.enable | bool | `true` | Enable/Disable the virtual kubelet metric agent. This component aggregates all the kubelet-related metrics (e.g., CPU, RAM, etc) collected on the nodes that are used by a remote cluster peered with you, then exporting  the resulting values as a property of the virtual kubelet running on the remote cluster. |
| metricAgent.image.name | string | `"ghcr.io/liqotech/metric-agent"` | Image repository for the metricAgent pod. |
| metricAgent.image.version | string | `""` | Custom version for the metricAgent image. If not specified, the global tag is used. |
| metricAgent.initContainer.image.name | string | `"ghcr.io/liqotech/cert-creator"` | Image repository for the init container of the metricAgent pod. |
| metricAgent.initContainer.image.version | string | `""` | Custom version for the init container image of the metricAgent pod. If not specified, the global tag is used. |
| metricAgent.pod.annotations | object | `{}` | Annotations for the metricAgent pod. |
| metricAgent.pod.extraArgs | list | `[]` | Extra arguments for the metricAgent pod. |
| metricAgent.pod.labels | object | `{}` | Labels for the metricAgent pod. |
| metricAgent.pod.priorityClassName | string | `""` | PriorityClassName (https://kubernetes.io/docs/concepts/scheduling-eviction/pod-priority-preemption/#pod-priority) for the metricAgent pod. |
| metricAgent.pod.resources | object | `{"limits":{},"requests":{}}` | Resource requests and limits (https://kubernetes.io/docs/user-guide/compute-resources/) for the metricAgent pod. |
| nameOverride | string | `""` | Override the standard name used by Helm and associated to Kubernetes/Liqo resources. |
| networking.clientResources | list | `[{"apiVersion":"networking.liqo.io/v1alpha1","resource":"wggatewayclients"}]` | Set the list of resources that implement the GatewayClient |
| networking.enabled | bool | `true` | Use the default Liqo networking module. |
| networking.gatewayTemplates | object | `{"container":{"gateway":{"image":{"name":"ghcr.io/liqotech/gateway","version":""}},"geneve":{"image":{"name":"ghcr.io/liqotech/gateway/geneve","version":""}},"wireguard":{"image":{"name":"ghcr.io/liqotech/gateway/wireguard","version":""}}},"ping":{"interval":"2s","lossThreshold":5,"updateStatusInterval":"10s"},"replicas":1,"server":{"service":{"allocateLoadBalancerNodePorts":"","annotations":{"service.beta.kubernetes.io/aws-load-balancer-type":"nlb"}}},"wireguard":{"implementation":"kernel"}}` | Set the options for the default gateway (server/client) templates. The default templates use a WireGuard implementation to connect the gateway of the clusters. These options are used to configure only the default templates and should not be considered if a custom template is used. |
| networking.gatewayTemplates.container.gateway.image.name | string | `"ghcr.io/liqotech/gateway"` | Image repository for the gateway container. |
| networking.gatewayTemplates.container.gateway.image.version | string | `""` | Custom version for the gateway image. If not specified, the global tag is used. |
| networking.gatewayTemplates.container.geneve.image.name | string | `"ghcr.io/liqotech/gateway/geneve"` | Image repository for the geneve container. |
| networking.gatewayTemplates.container.geneve.image.version | string | `""` | Custom version for the geneve image. If not specified, the global tag is used. |
| networking.gatewayTemplates.container.wireguard.image.name | string | `"ghcr.io/liqotech/gateway/wireguard"` | Image repository for the wireguard container. |
| networking.gatewayTemplates.container.wireguard.image.version | string | `""` | Custom version for the wireguard image. If not specified, the global tag is used. |
| networking.gatewayTemplates.ping | object | `{"interval":"2s","lossThreshold":5,"updateStatusInterval":"10s"}` | Set the options to configure the gateway ping used to check connection |
| networking.gatewayTemplates.ping.interval | string | `"2s"` | Set the interval between two consecutive pings |
| networking.gatewayTemplates.ping.lossThreshold | int | `5` | Set the number of consecutive pings that must fail to consider the connection as lost |
| networking.gatewayTemplates.ping.updateStatusInterval | string | `"10s"` | Set the interval at which the connection resource status is updated |
| networking.gatewayTemplates.replicas | int | `1` | Set the number of replicas for the gateway deployments |
| networking.gatewayTemplates.server | object | `{"service":{"allocateLoadBalancerNodePorts":"","annotations":{"service.beta.kubernetes.io/aws-load-balancer-type":"nlb"}}}` | Set the options to configure the gateway server |
| networking.gatewayTemplates.server.service | object | `{"allocateLoadBalancerNodePorts":"","annotations":{"service.beta.kubernetes.io/aws-load-balancer-type":"nlb"}}` | Set the options to configure the server service |
| networking.gatewayTemplates.server.service.allocateLoadBalancerNodePorts | string | `""` | Set to "false" if you expose the gateway service as LoadBalancer and you do not want to create also a NodePort associated to it (Note: this setting is useful only on cloud providers that support this feature). |
| networking.gatewayTemplates.server.service.annotations | object | `{"service.beta.kubernetes.io/aws-load-balancer-type":"nlb"}` | Annotations for the server service. |
| networking.gatewayTemplates.wireguard.implementation | string | `"kernel"` | Set the implementation used for the WireGuard connection. Possible values are "kernel" and "userspace". |
| networking.iptables | object | `{"mode":"nf_tables"}` | Iptables configuration tuning. |
| networking.iptables.mode | string | `"nf_tables"` | Select the iptables mode to use. Possible values are "legacy" and "nf_tables". |
| networking.mtu | int | `1340` | Set the MTU for the interfaces managed by liqo: vxlan, tunnel and veth interfaces. The value is used by the gateway and route operators. The default value is configured to ensure correct behavior regardless of the combination of the underlying environments (e.g., cloud providers). This guarantees improved compatibility at the cost of possible limited performance drops. |
| networking.reflectIPs | bool | `true` | Reflect pod IPs and EnpointSlices to the remote clusters. |
| networking.securityMode | string | `"FullPodToPod"` | Select the mode to enforce security on connectivity among clusters. Possible values are "FullPodToPod" and "IntraClusterTrafficSegregation"  |
| networking.serverResources | list | `[{"apiVersion":"networking.liqo.io/v1alpha1","resource":"wggatewayservers"}]` | Set the list of resources that implement the GatewayServer |
| offloading.defaultNodeResources.cpu | string | `"4"` | The amount of CPU to reserve for a virtual node targeting this cluster. |
| offloading.defaultNodeResources.ephemeralStorage | string | `"20Gi"` | The amount of ephemeral storage to reserve for a virtual node targeting this cluster. |
| offloading.defaultNodeResources.memory | string | `"8Gi"` | The amount of memory to reserve for a virtual node targeting this cluster. |
| offloading.defaultNodeResources.pods | string | `"110"` | The amount of pods that can be scheduled on a virtual node targeting this cluster. |
| offloading.enabled | bool | `true` | Enable/Disable the offloading module |
| offloading.reflection.configmap.type | string | `"DenyList"` | The type of reflection used for the configmaps reflector. Ammitted values: "DenyList", "AllowList". |
| offloading.reflection.configmap.workers | int | `3` | The number of workers used for the configmaps reflector. Set 0 to disable the reflection of configmaps. |
| offloading.reflection.endpointslice.workers | int | `10` | The number of workers used for the endpointslices reflector. Set 0 to disable the reflection of endpointslices. |
| offloading.reflection.event.type | string | `"DenyList"` | The type of reflection used for the events reflector. Ammitted values: "DenyList", "AllowList". |
| offloading.reflection.event.workers | int | `3` | The number of workers used for the events reflector. Set 0 to disable the reflection of events. |
| offloading.reflection.ingress.ingressClasses | list | `[]` | List of ingress classes that will be shown to remote clusters. If empty, ingress class will be reflected as-is. Example: ingressClasses: - name: nginx   default: true - name: traefik |
| offloading.reflection.ingress.type | string | `"DenyList"` | The type of reflection used for the ingresses reflector. Ammitted values: "DenyList", "AllowList". |
| offloading.reflection.ingress.workers | int | `3` | The number of workers used for the ingresses reflector. Set 0 to disable the reflection of ingresses. |
| offloading.reflection.persistentvolumeclaim.workers | int | `3` | The number of workers used for the persistentvolumeclaims reflector. Set 0 to disable the reflection of persistentvolumeclaims. |
| offloading.reflection.pod.workers | int | `10` | The number of workers used for the pods reflector. Set 0 to disable the reflection of pods. |
| offloading.reflection.secret.type | string | `"DenyList"` | The type of reflection used for the secrets reflector. Ammitted values: "DenyList", "AllowList". |
| offloading.reflection.secret.workers | int | `3` | The number of workers used for the secrets reflector. Set 0 to disable the reflection of secrets. |
| offloading.reflection.service.loadBalancerClasses | list | `[]` | List of load balancer classes that will be shown to remote clusters. If empty, load balancer classes will be reflected as-is. Example: loadBalancerClasses: - name: public   default: true - name: internal |
| offloading.reflection.service.type | string | `"DenyList"` | The type of reflection used for the services reflector. Ammitted values: "DenyList", "AllowList". |
| offloading.reflection.service.workers | int | `3` | The number of workers used for the services reflector. Set 0 to disable the reflection of services. |
| offloading.reflection.serviceaccount.workers | int | `3` | The number of workers used for the serviceaccounts reflector. Set 0 to disable the reflection of serviceaccounts. |
| offloading.reflection.skip.annotations | list | `["cloud.google.com/neg","cloud.google.com/neg-status","kubernetes.digitalocean.com/load-balancer-id","ingress.kubernetes.io/backends","ingress.kubernetes.io/forwarding-rule","ingress.kubernetes.io/target-proxy","ingress.kubernetes.io/url-map","metallb.universe.tf/address-pool","metallb.universe.tf/ip-allocated-from-pool","metallb.universe.tf/loadBalancerIPs","loadbalancer.openstack.org/load-balancer-id"]` | List of annotations that must not be reflected on remote clusters. |
| offloading.reflection.skip.labels | list | `[]` | List of labels that must not be reflected on remote clusters. |
| offloading.runtimeClass.annotations | object | `{}` | Annotations for the runtime class. |
| offloading.runtimeClass.enable | bool | `false` |  |
| offloading.runtimeClass.handler | string | `"liqo"` | Handler for the runtime class. |
| offloading.runtimeClass.labels | object | `{}` | Labels for the runtime class. |
| offloading.runtimeClass.name | string | `"liqo"` | Name of the runtime class to use for offloading. |
| offloading.runtimeClass.nodeSelector | object | `{"enable":true,"labels":{"liqo.io/type":"virtual-node"}}` | Node selector for the runtime class. |
| offloading.runtimeClass.nodeSelector.labels | object | `{"liqo.io/type":"virtual-node"}` | Labels for the node selector. |
| offloading.runtimeClass.tolerations | object | `{"enable":true,"tolerations":[{"effect":"NoExecute","key":"virtual-node.liqo.io/not-allowed","operator":"Exists"}]}` | Tolerations for the runtime class. |
| offloading.runtimeClass.tolerations.tolerations | list | `[{"effect":"NoExecute","key":"virtual-node.liqo.io/not-allowed","operator":"Exists"}]` | Tolerations for the tolerations. |
| openshiftConfig.enable | bool | `false` | Enable/Disable the OpenShift support, enabling Openshift-specific resources, and setting the pod security contexts in a way that is compatible with Openshift. |
| openshiftConfig.virtualKubeletSCCs | list | `["anyuid"]` | Security context configurations granted to the virtual kubelet in the local cluster. The configuration of one or more SCCs for the virtual kubelet is not strictly required, and privileges can be reduced in production environments. Still, the default configuration (i.e., anyuid) is suggested to prevent problems (i.e., the virtual kubelet fails to add the appropriate labels) when attempting to offload pods not managed by higher-level abstractions (e.g., Deployments), and not associated with a properly privileged service account. Indeed, "anyuid" is the SCC automatically associated with pods created by cluster administrators. Any pod granted a more privileged SCC and not linked to an adequately privileged service account will fail to be offloaded. |
| peering.networking | object | `{"gateway":{"mtu":1340,"server":{"service":{"port":51820,"type":"LoadBalancer"}}}}` | Set the default configuration for the networking resources created during the peering process |
| peering.networking.gateway | object | `{"mtu":1340,"server":{"service":{"port":51820,"type":"LoadBalancer"}}}` | Set the options for gateways |
| peering.networking.gateway.mtu | int | `1340` | Set the MTU |
| peering.networking.gateway.server | object | `{"service":{"port":51820,"type":"LoadBalancer"}}` | Set the options to configure the gateway server |
| peering.networking.gateway.server.service | object | `{"port":51820,"type":"LoadBalancer"}` | Set the options to configure the server service |
| peering.networking.gateway.server.service.port | int | `51820` | Set the port of the service |
| peering.networking.gateway.server.service.type | string | `"LoadBalancer"` | Set the type of the service |
| proxy.config.listeningPort | int | `8118` | Port used by the proxy pod. |
| proxy.image.name | string | `"ghcr.io/liqotech/proxy"` | Image repository for the proxy pod. |
| proxy.image.version | string | `""` | Custom version for the proxy image. If not specified, the global tag is used. |
| proxy.pod.annotations | object | `{}` | Annotations for the proxy pod. |
| proxy.pod.extraArgs | list | `[]` | Extra arguments for the proxy pod. |
| proxy.pod.labels | object | `{}` | Labels for the proxy pod. |
| proxy.pod.priorityClassName | string | `""` | PriorityClassName (https://kubernetes.io/docs/concepts/scheduling-eviction/pod-priority-preemption/#pod-priority) for the proxy pod. |
| proxy.pod.resources | object | `{"limits":{},"requests":{}}` | Resource requests and limits (https://kubernetes.io/docs/user-guide/compute-resources/) for the proxy pod. |
| proxy.service.annotations | object | `{}` |  |
| proxy.service.type | string | `"ClusterIP"` |  |
| pullPolicy | string | `"IfNotPresent"` | The pullPolicy for liqo pods. |
| storage.enable | bool | `true` | Enable/Disable the liqo virtual storage class on the local cluster. You will be able to offload your persistent volumes, while other clusters will be able to schedule their persistent workloads on the current cluster. |
| storage.realStorageClassName | string | `""` | Name of the real storage class to use in the local cluster. |
| storage.storageNamespace | string | `"liqo-storage"` | Namespace where liqo will deploy specific PVCs. Internal parameter, do not change. |
| storage.virtualStorageClassName | string | `"liqo"` | Name to assign to the liqo virtual storage class. |
| tag | string | `""` | Images' tag to select a development version of liqo instead of a release |
| telemetry.config.schedule | string | `""` | Set the schedule of the telemetry collector CronJob. Consider setting this value on ArgoCD deployments to avoid randomization. |
| telemetry.enable | bool | `true` | Enable/Disable the telemetry collector. |
| telemetry.image.name | string | `"ghcr.io/liqotech/telemetry"` | Image repository for the telemetry pod. |
| telemetry.image.version | string | `""` | Custom version for the telemetry image. If not specified, the global tag is used. |
| telemetry.pod.annotations | object | `{}` | Annotations for the telemetry pod. |
| telemetry.pod.extraArgs | list | `[]` | Extra arguments for the telemetry pod. |
| telemetry.pod.labels | object | `{}` | Labels for the telemetry pod. |
| telemetry.pod.resources | object | `{"limits":{},"requests":{}}` | Resource requests and limits (https://kubernetes.io/docs/user-guide/compute-resources/) for the telemetry pod. |
| uninstaller.image.name | string | `"ghcr.io/liqotech/uninstaller"` | Image repository for the uninstaller pod. |
| uninstaller.image.version | string | `""` | Custom version for the uninstaller image. If not specified, the global tag is used. |
| uninstaller.pod.annotations | object | `{}` | Annotations for the uninstaller pod. |
| uninstaller.pod.extraArgs | list | `[]` | Extra arguments for the uninstaller pod. |
| uninstaller.pod.labels | object | `{}` | Labels for the uninstaller pod. |
| uninstaller.pod.resources | object | `{"limits":{},"requests":{}}` | Resource requests and limits (https://kubernetes.io/docs/user-guide/compute-resources/) for the uninstaller pod. |
| virtualKubelet.extra.annotations | object | `{}` | Annotations for the virtual kubelet pod. |
| virtualKubelet.extra.args | list | `[]` | Extra arguments virtual kubelet pod. |
| virtualKubelet.extra.labels | object | `{}` | Labels for the virtual kubelet pod. |
| virtualKubelet.extra.resources | object | `{"limits":{},"requests":{}}` | Resource requests and limits (https://kubernetes.io/docs/user-guide/compute-resources/) for the virtual kubelet pod. |
| virtualKubelet.image.name | string | `"ghcr.io/liqotech/virtual-kubelet"` | Image repository for the virtual kubelet pod. |
| virtualKubelet.image.version | string | `""` | Custom version for the virtual kubelet image. If not specified, the global tag is used. |
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