Let's analyze and debug the complete path of an ICMP packet sent from a pod in the consumer cluster to a pod in the provider cluster.

For this example, we will use two Kubernetes-in-Docker (KinD) clusters with only one worker each and Calico as the CNI.

Here are the details of the two clusters:

- **Cluster Rome (Local):**
  - **Pod:** `nginx-local` (`10.200.1.2`)
  - **Gateway:** `gw-local` (`10.200.1.3`)
  - **Node:** `rome-worker` (`172.18.0.2`)
- **Cluster Milan (Remote):**
  - **Pod:** `nginx-remote` (`10.200.1.4`)
  - **Gateway:** `gw-remote` (`10.200.1.5`)
  - **Node:** `milan-worker` (`172.18.0.3`)

Since both clusters use `10.200.0.0/16` as the pod CIDR, Liqo remaps the pod CIDR of the remote cluster to `10.201.0.0/16` on the local cluster.

This means that, from the perspective of the local cluster, the remote pod `nginx-remote` will appear to have the IP address `10.201.1.4`.

This examples follows the path of an ICMP Echo Request packet sent from `nginx-local` to `nginx-remote` using the command: `ping 10.201.1.4`.

### Local Node

The ICMP packet exits the pod's default interface (e.g. `eth0`) and immediately enters the worker node's root network namespace via the other end of the veth pair (e.g., `caliaaaaa`). The node must now decide where to send this packet.

- **Policy Routing:** A custom policy routing rule matches the destination IP `10.201.0.0/16`. The matching rule is as follows: `10.201.0.0/16 via 10.71.0.3 dev liqo.00000`
- **Interface:** This route sends the packet to a special interface named `liqo.00000`, which is a **GENEVE tunnel**.
- **Encapsulation:** This interface encapsulates the _entire L2 Ethernet frame_ (not just the IP packet) inside a new UDP packet.
  - **Inner Packet (Original):** `10.200.1.2` > `10.201.1.4`
  - **Outer Packet (New):**
    - **Source IP:** `172.18.0.2` (the `rome-worker` node)
    - **Destination IP:** `10.200.1.3` (the `gw-local` pod)
    - **Destination Port:** `6091` (the GENEVE port)

This new, larger UDP packet is then sent via the standard CNI network to the `gw-local` pod.

In this case, with just one node, the packet is sent directly to the `gw-local` pod, however in a multi-node cluster, the CNI may route it to another node first.

TIP: to inspect the policy routing, use the command `ip rule show all` to find the correct table and `ip route show table <table_id>`.

### Local Gateway

The encapsulated packet arrives at the gateway's default interface (e.g. `eth0`).

- **Decapsulation:** The gateway's networking stack recognizes the `6091` destination port and directs the packet to its own GENEVE interface (e.g., `liqo.11111`), which is paired with the worker's interface. The gateway strips the outer UDP/IP headers, extracting the original inner packet:
  > `SRC: 10.200.1.2` > `DST: 10.201.1.4`
- **DNAT (Destination NAT):** This packet's destination (`10.201.1.4`) is meaningless to the remote cluster. The gateway must translate it back to the pod's _real_ IP. This is done using **`nftables`**. A rule in the `remap-podcidr` table performs a Destination NAT:
  > `ip daddr 10.201.0.0/16 ... dnat prefix to 10.200.0.0/16`
- **Packet Transformation:**
  - **Before DNAT:** `10.200.1.2` > `10.201.1.4`
  - **After DNAT:** `10.200.1.2` > `10.200.1.4`
- **Routing to WireGuard:** Now that the packet has its _real_ destination, it's routed to the inter-cluster tunnel. The route is: `10.200.0.0/16 via 169.254.18.1 dev liqo-tunnel`

TIP: to know which is the other end of a GENEVE interface, use the command `ip -d link`, both ends must have the same ID.

### WireGuard Tunnel

The packet is sent to the `liqo-tunnel` interface, which is (by default) a **WireGuard** interface.

- **Encapsulation:** WireGuard encrypts the IP packet and wraps it in a new UDP packet for transport.
  - **Inner Packet (NATTed):** `10.200.1.2` > `10.200.1.4`
  - **Outer Packet:**
    - **Source IP:** `10.200.1.3` (the `gw-local` pod)
    - **Destination IP:** `172.18.0.3` (the `milan-worker` node, where `gw-remote` runs)
- **Node-Level SNAT:** This encrypted packet exits the `gw-local` pod and goes back to the `rome-worker` node. The node performs _another_ SNAT (Source NAT) to make the packet routable on the external network.
  - **Before SNAT:** `10.200.1.3:36252` > `172.18.0.3:31864`
  - **After SNAT:** `172.18.0.2:17806` > `172.18.0.3:31864`

This final, encrypted, and twice-NATted packet now travels "across the internet" (or in this case, the Docker network) from `rome-worker` to `milan-worker`.

### Remote Node

The encrypted packet arrives at the `milan-worker`'s default interface (e.g. `eth0`).

- **Node-Level DNAT:** Since the packet is addressed to the node itself, the node's kube-proxy intercepts it and performs a Destination NAT to send it to the `gw-remote` pod.
  - **Before DNAT:** `172.18.0.2:17806` > `172.18.0.3:31864`
  - **After DNAT:** `172.18.0.3:44207` > `10.200.1.5:51840`
- **Routing:** The packet is sent to the pod according to the CNI routing rules. In our example the rule is: `10.200.1.5 dev caliccccc scope link`

### Remote Gateway

The encrypted packet arrives at the `gw-remote`'s default interface (e.g. `eth0`).

- **Decapsulation (WireGuard):** The gateway listens on port `51840` .
  > **Decrypted Packet:** `SRC: 10.200.1.2` > `DST: 10.200.1.4`
- **SNAT (Source NAT):** This packet is almost ready, but the _source_ IP (`10.200.1.2`) is unknown to the `nginx-remote` pod and is overlapping with its own Pod CIDR. It must be natted to the _remapped_ IP that Cluster Milan expects. The gateway performs a Source NAT.
  - **Before SNAT:** `10.200.1.2` > `10.200.1.4`
  - **After SNAT:** `10.201.1.2` > `10.200.1.4`
- **Routing to GENEVE:** The packet is now routed to the destination pod via another GENEVE tunnel that connects the remote gateway to its worker. In our example the rule is: `10.200.1.4 via 10.71.0.2 dev liqo.22222`
- **Encapsulation (GENEVE):** The packet is encapsulated one last time for delivery to the worker node.
  - **Inner Packet:** `10.201.1.2` > `10.200.1.4`
  - **Outer Packet:** `10.200.1.5` > `172.18.0.3` (Port `6091`)

### Final Delivery

1.  The GENEVE packet exits the `gw-remote` pod and enters the `milan-worker` node, which in our case is also the node hosting the target pod.
2.  The node's GENEVE interface (`liqo.33333`) decapsulates the packet, retrieving:
    > `SRC: 10.201.1.2` > `DST: 10.200.1.4`
3.  The node's routing table (created by the CNI plugin) directs this packet to the final pod via the appropriate end of the `veth` pair.
4.  The packet travels across the `veth` and arrives at the default interface of the `nginx-remote` pod.

The `nginx-remote` pod successfully receives the ICMP Echo Request from the (remapped) source `10.201.1.2` and sends an Echo Reply, which follows the entire path in reverse.
