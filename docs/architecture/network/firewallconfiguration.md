# Firewall Configuration

The **firewall configuration** is a CRD (Custom Resource Definition) that defines a set of **nftables** rules.

**Firewall configurations** are managed by a dedicated controller running inside gateways and fabric pods. This controller reconcile the **firewall configurations** and applies the rules.

## Before Peering

### \<node-name\>-gw-masquerade-bypass (Node)

Some CNIs masquerade traffic from a pod to a node (not running the pod) using the node's internal IP. For example, if a pod has IP 10.0.0.8 and is scheduled on a node with internal IP 192.168.0.5, pinging another node will result in packets with 192.168.0.5 as the source IP.

This behavior has been observed in the following scenarios:

- Azure CNI
- Calico (when pod masquerade is enabled, e.g., Crownlabs)
- KindNet

This can be problematic because Geneve does not support NAT (or IP changes) between the two hosts. When a Geneve tunnel needs to be established (such as between gateway pods and nodes), both hosts must be able to "ping" each other using the IP assigned to their network interface.
In the case described above, the source address of the packets from the Gateway pod directed to one of the node via the Geneve tunnel is masquerated with the IP address of where the Gateway pod is running.

This firewall configuration solves the issue using the same approach as the **\<tenant\>-masquerade-bypass** configuration. The double SNAT trick is also used here to prevent masquerading. That's because a SNAT rule that enforces a specific IP address nullify any subsequent SNAT rules targetting that address. The same applies to DNAT rules.
For instance , if a packet uses 10.0.0.1 as the source IP, a SNAT that enforces 10.0.0.1 will nullify any subsequent SNAT rules targetting 10.0.0.1.
Which means that if a SNAT is applied on traffic coming from the IP address of the gateway and enforces its same IP address, prevents the CNI from source natting packets from the gateway with the IP address of the node.
Whenever a gateway pod is scheduled on a node, a rule is added only on that specific node (whose name is reported in the label `networking.liqo.io/firewall-unique`). It matches only traffic with the gateway IP as the source and the Geneve port as the destination.

```yaml
apiVersion: networking.liqo.io/v1beta1
kind: FirewallConfiguration
metadata:
  labels:
    liqo.io/managed: "true"
    networking.liqo.io/firewall-category: fabric
    networking.liqo.io/firewall-subcategory: single-node
    networking.liqo.io/firewall-unique: cheina-cluster1-worker2
    networking.liqo.io/gateway-masquerade-bypass: "true"
  name: cheina-cluster1-worker2-gw-masquerade-bypass
  namespace: liqo
spec:
  table:
    chains:
      - hook: postrouting
        name: pre-postrouting
        policy: accept
        priority: 99
        rules:
          natRules:
            - match:
                - ip:
                    position: src
                    value: 10.127.65.33
                  op: eq
                  port:
                    position: dst
                    value: "6091"
                  proto:
                    value: udp
              name: gw-cheina-cluster2-66bc45dd78-75d9j
              natType: snat
              targetRef:
                kind: Pod
                name: gw-cheina-cluster2-66bc45dd78-75d9j
                namespace: liqo-tenant-cheina-cluster2
              to: 10.127.65.33
        type: nat
    family: IPV4
    name: cheina-cluster1-worker2-gw-masquerade-bypass
```

### service-nodeport-routing

This rule contains the ctmark rule to create the contract with the mark corresponding to the node and to add the mark to the packet metadata once the return traffic traverse the gateway.

```yaml
chains:
  - hook: forward
    name: mark-to-conntrack
    policy: accept
    priority: 0
    rules:
      filterRules:
        - action: ctmark
          counter: false
          match:
            - ip:
                position: src
                value: 10.70.0.0
              op: eq
            - dev:
                position: in
                value: liqo.ljklgrhxmg
              op: eq
          name: k3d-rome-agent-0
          value: "1"
        - action: ctmark
          counter: false
          match:
            - ip:
                position: src
                value: 10.70.0.0
              op: eq
            - dev:
                position: in
                value: liqo.68qnvfq22j
              op: eq
          name: k3d-rome-agent-1
          value: "2"
        - action: ctmark
          counter: false
          match:
            - ip:
                position: src
                value: 10.70.0.0
              op: eq
            - dev:
                position: in
                value: liqo.skqvtmsksl
              op: eq
          name: k3d-rome-server-0
          value: "3"
    type: filter
  - hook: prerouting
    name: conntrack-mark-to-meta-mark
    policy: accept
    priority: 0
    rules:
      filterRules:
        - action: metamarkfromctmark
          counter: false
          match:
            - ip:
                position: dst
                value: 10.70.0.0
              op: eq
            - dev:
                position: in
                value: liqo-tunnel
              op: eq
          name: conntrack-mark-to-meta-mark
    type: filter
```

## After Peering

### \<tenant-name\>-masquerade-bypass (Node)

This firewall configuration contains several rules with different purposes.

Sometimes CNIs masquerade traffic which is not part of the pod CIDR. This masquerade can cause issues when the traffic needs to be routed back to the originating cluster. To prevent this, a SNAT rule is applied to packets that are originated from the pod CIDR and destined to the remote pod CIDR.

```yaml
- match:
    - ip:
        position: dst
        value: 10.71.0.0/18
      op: eq
    - ip:
        position: src
        value: 10.127.64.0/18
      op: eq
  name: podcidr-cheina-cluster2
  natType: snat
  to: 10.127.64.0/18
```

The following rules enforce the presence of the first external CIDR IP in packets received by NodePort services. Refer to the **service-nodeport-routing** firewall configuration for more details.

```yaml
- match:
    - ip:
        position: dst
        value: 10.71.0.0/18
      op: eq
    - ip:
        position: src
        value: 10.127.64.0/18
      op: neq
  name: service-nodeport-cheina-cluster2
  natType: snat
  to: 10.70.0.0
```

The other rules apply the same concept but for the external CIDR.

**This rule is the reason why NodePorts do not work with Cilium without kube-proxy. With no-kubeproxy, Cilium uses eBPF to manage firewall rules, so this fwcfg is bypassed. A future solution should consider moving this rule inside the Gateway. Please notice that moving this rule is not enough since some routing rules inside the gateway use the source IP as policy and the SNAT is only available on pre-routing netfilter's hook.**

#### Full-Masquerade

When the flag **networking.fabric.config.fullMasquerade** is **true**, this firewall configuration changes. In particular, the **service-nodeport-\<cheina-cluster2\>** rule becomes the only one still present, and its match rules do not include a check on the source IP of the packets.

```yaml
- match:
    - ip:
        position: dst
        value: 10.71.0.0/18
      op: eq
  name: service-nodeport-cheina-cluster2
  natType: snat
  to: 10.70.0.0
```

This rule directs all traffic destined for the remote cluster to use the **unknown IP** as the source IP. This means that the remote traffic will see all incoming traffic from its peered cluster as originating from the first external CIDR IP.

This is useful when the cluster's CNI masquerade with the node's IP all the traffic directed to an IP which is not part of the pod CIDR, service CIDR or node CIDR. If this masquerade happens, the remote cluster will not be able to route the traffic back to the originating cluster.

### \<tenant-name\>-remap-podcidr (Gateway)

This rule manages the CIDR remapping in cases where two clusters have the same pod CIDR. It contains two rules.

Before continuing, let's recap how this works:

Imagine we have two clusters named Cluster A and Cluster B, both with the same pod CIDR (e.g., 10.1.0.0/16). Each cluster can remap the CIDR of the adjacent one, even if they are the same. Cluster A can autonomously decide on a new CIDR to identify Cluster B's pods. When Cluster A wants to send traffic to Cluster B, it will use the new remapped CIDR. This rule's purpose is to translate the "fake" destination IP back to the real one. Note that this rule ignores traffic coming from eth0 and liqo-tunnel, as traffic from the pods will be received on Geneve interfaces (liqo-xxx).

```yaml
- match:
    - ip:
        position: dst
        value: 10.71.0.0/18
      op: eq
    - dev:
        position: in
        value: eth0
      op: neq
    - dev:
        position: in
        value: liqo-tunnel
      op: neq
  name: 17b97d17-aa77-4494-bf9c-d307600f37af
  natType: dnat
  to: 10.127.64.0/18
```

This rule performs the opposite function for packets coming from the other cluster. It maps the packet's source IP using the remapped CIDR, which is necessary for routing the returning packets, that's why source natting is applied only to `liqo-tunnel`, which is the interface of the wireguard tunnel:

```yaml
- match:
    - dev:
        position: out
        value: eth0
      op: neq
    - ip:
        position: src
        value: 10.127.64.0/18
      op: eq
    - dev:
        position: in
        value: liqo-tunnel
      op: eq
  name: 75b6467c-4ce3-4434-8e18-9b4f568c12c7
  natType: snat
  to: 10.71.0.0/18
```

### \<tenant-name\>-remap-externalcidr (Gateway)

Functions similarly to **\<tenant-name\>-remap-podcidr** but for the **external-cidr**.

### \<name\>-remap-ipmapping-gw

These firewall configurations are created from IP resources (refer to the IP section), containing SNAT and DNAT rules to make the "local IP" reachable through the external CIDR.

In the next example we can see the firewall configuration created from an IP resource which remaps the local IP `10.122.2.114` (Pod IP) to the external CIDR IP `10.70.0.4` (external CIDR is `10.70.0.0/16`).

```yaml
apiVersion: networking.liqo.io/v1beta1
kind: FirewallConfiguration
metadata:
  labels:
    liqo.io/managed: "true"
    networking.liqo.io/firewall-category: gateway
    networking.liqo.io/firewall-subcategory: ip-mapping
  name: nginx-5869d7778c-pvpvw-remap-ipmapping-gw
  namespace: liqo-demo
spec:
  table:
    chains:
      - hook: prerouting
        name: prerouting
        policy: accept
        priority: -100
        rules:
          natRules:
            - match:
                - ip:
                    position: dst
                    value: 10.70.0.4
                  op: eq
              name: nginx-5869d7778c-pvpvw
              natType: dnat
              to: 10.122.2.114
        type: nat
      - hook: postrouting
        name: postrouting
        policy: accept
        priority: 100
        rules:
          natRules:
            - match:
                - ip:
                    position: src
                    value: 10.70.0.4
                  op: eq
              name: nginx-5869d7778c-pvpvw
              natType: snat
              to: 10.122.2.114
        type: nat
    family: IPV4
    name: nginx-5869d7778c-pvpvw-remap-ipmapping-gw-liqo-demo
```
