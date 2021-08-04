---
title: K8s with Kubeadm
weight: 4
---

### About Kubeadm

[Kubeadm](https://kubernetes.io/docs/setup/production-environment/tools/kubeadm/create-cluster-kubeadm/) is a tool built by the Kubernetes community to provision Kubernetes clusters. Kubeadm is used as the basis of most Kubernetes deployments and makes it easier to create K8s clusters.

### CNI Compatibility Matrix

Kubeadm does not install any CNI plugin by default, and it must be deployed after the initialization of the cluster.

Liqo supports several CNIs as mentioned in the following table:

| CNI                    | Version                             | Support                                   | Notes                       |
| ---------------------- | ------------------------------      | --------------------------------------    | --------------------------- |
| [Calico](#calico)      | v3.17.2                             |  Yes, with minor configurations           |                             |
| Flannel                | v0.13.0                             |  Yes                                      |                             |
| Cilium                 | v1.9.4                              |  Yes, but only using kube-proxy           |                             |
| Weavenet               | v2.8.1                              |  Yes                                      |                             |
| Canal                  | v3.17.2                             |  Yes                                      |                             |

#### Calico

>__NOTE__: If you are using Calico on your cluster, __YOU MUST READ__ the following, otherwise you may end up breaking your set-up.

When installed, Liqo adds several interfaces to a cluster to handle cross-cluster traffic routing. Those interfaces are intended to not interfere with the CNI normal job.

However, Calico scans existing interfaces on a node to detect network configuration and establish the correct routes. To make everything work well, Calico should ignore Liqo interfaces. This change are required only if you have Calico configured in BGP mode. If you use VPC native setup, you are not required to do this modification.

To do so, In Calico v3.17, you should patch the Installation CR by adding in `.spec.calicoNetwork.nodeAddressAutodectection.skipInterface` with `liqo.*`. An example of the patched resource is available here:

```yaml
apiVersion: operator.tigera.io/v1
kind: Installation
metadata:
  name: default
spec:
  calicoNetwork:
    bgp: Enabled
    hostPorts: Enabled
    ipPools:
    - blockSize: 24
      cidr: 10.244.0.0/16
      encapsulation: None
      natOutgoing: Enabled
      nodeSelector: all()
    multiInterfaceMode: None
    nodeAddressAutodetectionV4:
      skipInterface: liqo.*
  cni:
    ipam:
      type: Calico
    type: Calico
  flexVolumePath: /usr/libexec/kubernetes/kubelet-plugins/volume/exec/
  nodeUpdateStrategy:
    rollingUpdate:
      maxUnavailable: 1
    type: RollingUpdate
  variant: Calico
```

For versions prior 3.17, you should modify the `calico-node` Daemonset. First, you should type:

```
kubectl edit daemonset -n kube-system calico-node
```

And then, you should add/edit the environment variables of the calico-node container.

```
        - name: IP_AUTODETECTION_METHOD
          value: skip-interface=liqo.*
```


### Liqo Installation

#### Pre-requirements

To install Liqo, you have to install the following dependencies:

* [Helm 3](https://helm.sh/docs/intro/install/)
* [Kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/)

### Deploy

You can install Liqo using helm 3.

Firstly, you should add the official Liqo repository to your Helm Configuration:

```bash
helm repo add liqo https://helm.liqo.io/
```

If you are installing Liqo for the first time, you can download the default values.yaml file from the chart.

```bash
helm show values liqo/liqo > ./values.yaml
```

The most important values you can set are the following:

| Variable               | Description                                 |
| ---------------------- | ------------------------------------------- |
| `networkManager.config.podCIDR`        | the cluster Pod CIDR                        |
| `networkManager.config.serviceCIDR`    | the cluster Service CIDR                    |
| `discovery.config.clusterName`         | nickname for your cluster that others will see. |

#### Extract PodCIDR and ServiceCIDR

The podCIDR and the serviceCIDR can be retrieved using kubectl:

```bash

POD_CIDR=$(kubectl get pods --namespace kube-system --selector component=kube-controller-manager --output jsonpath="{.items[*].spec.containers[*].command}" 2>/dev/null | grep -Po --max-count=1 "(?<=--cluster-cidr=)[0-9.\/]+")

SERVICE_CIDR=$(kubectl get pods --namespace kube-system --selector component=kube-controller-manager --output jsonpath="{.items[*].spec.containers[*].command}" 2>/dev/null | grep -Po --max-count=1 "(?<=--service-cluster-ip-range=)[0-9.\/]+")

echo "POD CIDR: $POD_CIDR"

echo "SERVICE CIDR: $SERVICE_CIDR"
```

### Scenario Mapping

The installation process may require different parameters according to the context (i.e., cluster network exposition).

### On-premise Cluster behind NAT

If your cluster is hosted on-premise behind a NAT and you would like to connect your cluster with another via the Internet, you may use the following configuration:

| Variable | Value/Notes |
| -------- | ----------- |
| `auth.service.type` | NodePort |
| `apiserver.ip` |  The IP/Host exposed by the NAT |
| `apiserver.port` | The port exposed by the NAT  |
| `gateway.service.type` | NodePort |

### On-premise Cluster to Cloud

If you have a full-fledged cluster with support service Load Balancers with external IP and/or ingress-controller. You can set the following parameters:

| Variables | Value/Notes |
| -------- | ----------- |
| `auth.service.type`  | ClusterIp |
| `auth.ingress.enable` | true  |
| `authServer.host`     | true  |
| `apiserver.address` |  The IP/host other clusters can use to access the cluster |
| `apiserver.port` |  The IP/host other clusters can use to access the cluster  |
| `gateway.service.type` | LoadBalancer |

### On-Premise to On-Premise

If you want to connect your cluster with another K3s/K8s in the same LAN, you do not need further configuration. You can install Liqo by just specifying the correct values for the three variables mentioned above:

```
helm install liqo liqo/liqo -n liqo --create-namespace  --set clusterName="MyCluster" --set networkManager.config.podCIDR="10.42.0.0/16" --set networkManager.config.serviceCIDR="10.96.0.0/12"
```

__NOTE__: You should check that `podCIDR` and `serviceCIDR` correspond to the one in your cluster.

If the clusters you would like to connect are in the same L2 broadcast domain, the Liqo discovery mechanism based on mDNS will handle the discovery automatically. If you have your clusters in different L3 domains, you have to manually create [a *foreign_cluster* resource](/configuration/discovery#manual-configuration) or rely on [DNS discovery](/configuration/discovery#).
