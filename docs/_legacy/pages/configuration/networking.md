# Networking

### MTU Configuration

The maximum transmission unit (MTU) is a measurement representing the largest data packet that can be transmitted through a network.
The correct MTU setting is vital for a given network.
In general a larger MTU brings greater efficiency allowing to reach the maximum performance of the network.
Beware that higher MTU could cause fragmentation or dropped packets on the path between two devices.
Special cases are the tunnel devices interconnecting different networks and need to be carefully configured in order for the packets flowing between the tunnel devices to not be dropped or fragmented.

#### Liqo's Network Interfaces

Liqo's network operators handle different network interfaces.
On each node of a given cluster a VxLAN network interface named `liqo.vxlan` is configured bringing up an overlay network between all the nodes. 
In the node where the active gateway is running there will be a [custom network namespace](../../../concepts/networking/components/gateway/#tunnel-operator) named `liqo-netns`.
In the custom namespace a tunnel device named `liqo.wg`, used to interconnect the cluster to remote ones, is created.
In the host network namespace a veth device named `liqo.host` is created, and the other end named `liqo.gateway` is created in the custom network namespace.

#### Liqo and MTU size

The following table lists common MTU sizes for Liqo environments.
{{% notice note %}}
Beware that the MTU is a global property of the network path between endpoints, you should set the MTU to the minimum MTU of any path that the packets may take. 
For example if you are interconnecting two clusters one running in AKS and the other one in GCE the MTU should be set to 1340 for both clusters.
{{% /notice %}}

|Network MTU	| Liqo Wireguard MTU(IPV4)  |   	
|---	        |---                        |
|   1500        |   1440                    |
|   9000	    |   8940                    |
|   1500 (AKS)  |   1340                    |
|   1460 (GCE)  |   1400                    |
|   1500 (EKS)  |   1440                    |

VxLAN uses a 50-byte header and WireGuard uses a [60-byte header](https://lists.zx2c4.com/pipermail/wireguard/2017-December/002201.html).
In the AKS environment the network interfaces will have an MTU of 1500 but the underlying network has an [MTU of 1400](https://docs.microsoft.com/en-us/azure/virtual-network/virtual-network-tcpip-performance-tuning#azure-and-vm-mtu).
Since WireGuard sets the `Don't Fragment(DF)` bit on its packets, the MTU for the Liqo network interfaces has to be set to 1340.

#### Configure MTU

Configuring the MTU is as simple as specifying the mtu value in [liqoctl](../../../installation/#quick-installation) install command by using the appropriate flag:

```
liqoctl install ${YOUR_PROVIDER} --cluster-name ${YOUR_CLUSTER_NAME} --mtu 1400
```
{{% notice note %}}
The `liqoctl install` command is idempotent and can be executed multiple times to enforce the desired configuration.
{{% /notice %}}

If you are installing Liqo using the provided helm chart than the MTU size can be configured by setting the `networking.mtu` variable in the [values.yaml file](../../../installation/chart_values/#values).



