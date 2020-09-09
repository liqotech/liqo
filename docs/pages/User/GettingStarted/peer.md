---
title: Peer to a foreign cluster
weight: 2
---


After having installed LIQO on your cluster you can rely on multiple methods to peer with other clusters:

1. L2 Discovery: Liqo will discover other cluster in the same LAN
2. DNS Discovery: Liqo will discover available clusters in a specific SearchDomain (e.g.; liqo.io), by scraping the existence of specifc DNS entries.
3. Manual Addition: Liqo administrators may manually add clusters to the list of known foreign clusters.


All those methods are described in depth in a page the [discovery configuration](/user/configure/discovery)
Before peering your home cluster with a foreign cluster, we suggest having a look at the [Liqo peering basics](../../liqo-brief/#peering-basics) to get an overview about the different peering methods provided by Liqo.
All the next steps can be performed via the graphical [Dashboard](../dashboard), using the Liqo Home view. 

## Explore available clusters in your LAN


As mentioned above, Liqo is able (1) to discover available clusters and (2) make your cluster discoverable. 
Via the dashboard, after login, all different ForeignCluster are available in the "Available Peers" box in the home view.
Default policy tries to activate peering with a remote cluster, when it is discovered.
If you are already connected to a cluster in your LAN, you will observe a cluster inside the "Connected Peers" part.

{{%expand  Using Kubectl, you can also obtain the list of available discovered foreign clusters. %}}

You can list the **foreignclusters** resources via:

```
kubectl get foreignclusters
NAME                                   AGE
ff5aa14a-dd6e-4fd2-80fe-eaecb827cf55   101m
```
Peering can be enabled by setting the `Join` property in `ForeignCluster`resource to `true` or via the dashboard.

The check if the peering is in progress with the other cluster, you can verify that the `join` property of the target `ForeignCluster`.
This can be easily done via:

```
kubectl get foreignclusters ff5aa14a-dd6e-4fd2-80fe-eaecb827cf55 --template={{.spec.join}}
true
```
{{% /expand %}}


## Add Manual Peering

If the cluster you want to peer with is not present in your LAN, you can manually add it via the dashboard.

First, you have to collect some information about the remote cluster. In particular:

1. The APIServer: the address of the remote cluster. It should be accessible without NAT from the Home cluster.

First of all, you need to export the right KUBECONFIG variable, to select the remote cluster and then extract the address of
the APIServer:

```
export KUBECONFIG=$REMOTE_KUBECONFIG_PATH
kubectl config view -o jsonpath='{.clusters[].cluster.server}'
```
The last command suppose that you have just one cluster in your kubeconfig.

2. The ClusterID: the UUID which identifies the remote cluster.

Similarly, to extract the cluster-id:
```
export KUBECONFIG=$REMOTE_KUBECONFIG_PATH
kubectl config view -o jsonpath='{.clusters[?(@.name == "$CLUSTER_NAME")].cluster.server}'
kubectl get configmap -n liqo cluster-id -o jsonpath='{.data.cluster-id}'
```

Now, you can go to the home cluster LiqoDashboard click on "+" near Available Peers and then "Add Remote Peer" tab. Here, you
 can "cut and paste" those parameters from the terminal to the Liqo Dashboard running on the remote cluster. In particular, you have to set the other fields:
 
 1. **Name**: Name of the cluster (i.e. can be identical to the clusterID)
 2. **Discovery Type**: Manual
 3. **AllowedUntrustedCA**: True
 4. **Join**: True

## Peering checking

### Presence of the virtual-node

If the peering has been correctly performed, you should see a virtual node in addition to your physical nodes: 

```
kubectl get no

NAME                                      STATUS   ROLES    AGE     VERSION          LABELS
rar-k3s-01                                Ready    master   3h18m   v1.18.6+k3s1     beta.kubernetes.io/arch=amd64,beta.kubernetes.io/instance-type=k3s,beta.kubernetes.io/os=linux,k3s.io/hostname=rar-k3s-01,k3s.io/internal-ip=10.0.2.4,kubernetes.io/arch=amd64,kubernetes.io/hostname=rar-k3s-01,kubernetes.io/os=linux,net.liqo.io/gateway=true,node-role.kubernetes.io/master=true,node.kubernetes.io/instance-type=k3s
liqo-remote-cluster   Ready    agent    3h5m    v1.17.2-vk-N/A   alpha.service-controller.kubernetes.io/exclude-balancer=true,beta.kubernetes.io/os=linux,kubernetes.io/hostname=vk-e582fe9d-03d1-4788-ad85-4d04674a6437,kubernetes.io/os=linux,kubernetes.io/role=agent,**type=virtual-node**
```

## Verify that the resulting infrastructure works correctly

You are now ready to verify that the resulting infrastructure works correctly, which is presented in the [next step](../test).

