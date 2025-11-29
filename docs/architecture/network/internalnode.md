# InternalNode

When a pod using the **host network** communicates with a regular pod, the source IP address depends on the location of the target pod. If the target pod is on the same node, one source IP is used; if the target pod is on a different node, a different source IP is used.

> Please note that this behaviour can change depending on the CNI used.

The InternalNode resource stores the information about which IP is used to reach the node itself and which IP is used to reach the other cluster's nodes.

This information is provided using `ip route get <gw-ip>` on each node and getting the src IP from the output. That command allows to understand what network interface will be used to reach the gateway pod.

This information is fundamental to enstablish the geneve tunnels, because each gateway need to know what IP to use to reach a node. Remember that geneve wants that the couple src/dst IP used in one way is the same used in the other way.

In this example, the gateway for the remote cluster **cheina-cluster2** is scheduled on the node **cheina-cluster1-worker4**. The internal IP of the node **cheina-cluster1-worker4** is **10.112.2.188** and liqo is able to understand it running the command `ip route get 10.112.2.170` (where `10.112.2.170` is the IP address of pod of the Gateway) on the node **cheina-cluster1-worker4**.
The same command is executed for each of the other nodes to get the **remote IP** used by the Gateways to create a Geneve tunnel with each of the nodes of the cluster.

Note that the IPs defined in the InternalNode resources are valid for each of the gateways created in the cluster and they are divided into local and remote because each gateway use the local address to create a tunnel between the Gateway pod and the Node where it is running, and the remote address to create a tunnel with all the other nodes.

```
kubectl get internalnode -o wide
NAME                            NODE IP LOCAL   NODE IP REMOTE   AGE
cheina-cluster1-control-plane                   10.112.0.132     9m36s
cheina-cluster1-worker                          10.112.4.211     9m36s
cheina-cluster1-worker2                         10.112.3.19      9m36s
cheina-cluster1-worker3                         10.112.1.2       9m36s
cheina-cluster1-worker4         10.112.2.188                     9m36s
```

```
kubectl get pods -l networking.liqo.io/component=gateway -A -o wide
NAMESPACE                     NAME                                 READY   STATUS    IP             NODE
liqo-tenant-cheina-cluster2   gw-cheina-cluster2-c5bd76b56-7mxd4   3/3     Running   10.112.2.170   cheina-cluster1-worker4
```
