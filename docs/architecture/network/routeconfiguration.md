# Route Configuration

The **Route Configuration** is a CRD (Custom Resource Definition) that defines a set of **policy routing** rules for routing traffic within the Liqo network.

**Route configurations** are managed by a dedicated controller running inside gateways and fabric pods. This controller reconciles the **route configurations** and applies the rules.

## Before Peering

### \<local-cluster-id\>-\<node-name\>-extcidr (Gateway)

Contains all the routes that match traffic targeting the local external CIDR.

```yaml
apiVersion: networking.liqo.io/v1beta1
kind: RouteConfiguration
metadata:
  labels:
    liqo.io/managed: "true"
    networking.liqo.io/route-category: gateway
    networking.liqo.io/route-subcategory: fabric-node
    networking.liqo.io/route-unique: cheina-cluster1-worker
  name: cheina-cluster1-worker-extcidr
  namespace: liqo
spec:
  table:
    name: cheina-cluster1-worker-extcidr
    rules:
      - iif: liqo-tunnel
        routes:
          - dev: liqo.cjntnn4bdj
            dst: 10.111.105.133/32
            gw: 10.80.0.3
          - dev: liqo.cjntnn4bdj
            dst: 10.111.0.1/32
            gw: 10.80.0.3
```

### \<local-cluster-id\>-\<node-name\>-service-nodeport-routing (Gateway)

These rules use marks with policy routing to route the returning traffic towards the correct node. Refer to the **service-nodeport-routing** firewall configuration for more details.

```yaml
apiVersion: networking.liqo.io/v1beta1
kind: RouteConfiguration
metadata:
  labels:
    liqo.io/managed: "true"
    networking.liqo.io/route-category: gateway
    networking.liqo.io/route-subcategory: fabric
  name: cheina-cluster1-control-plane-service-nodeport-routing
  namespace: liqo
spec:
  table:
    name: cheina-cluster1-control-plane-service-nodeport-routing
    rules:
      - dst: 10.70.0.0/32
        fwmark: 2
        routes:
          - dev: liqo.jdr5xndgmb
            dst: 10.70.0.0/32
            gw: 10.80.0.2
        targetRef:
          kind: InternalNode
          name: cheina-cluster1-control-plane
```

### \<local-cluster-id\>-\<node-name\>-gw-node (Gateway)

Contains the rule that allows traffic from the gateway to nodes using Geneve tunnels. Note that Liqo uses the internal CIDR to assign an IP to every Geneve interface. If you need to debug the traffic between Geneve interfaces, you can ping each interface.

Also contains a route for each pod in the cluster. These routes allow traffic coming from other clusters to be forwarded to the correct node. This is necessary because Kubernetes does not provide a standard way to determine the pod CIDR range used for each node.

```yaml
apiVersion: networking.liqo.io/v1beta1
kind: RouteConfiguration
metadata:
  labels:
    liqo.io/managed: "true"
    networking.liqo.io/route-category: gateway
    networking.liqo.io/route-subcategory: fabric
  name: cheina-cluster1-control-plane-gw-node
  namespace: liqo
spec:
  table:
    name: cheina-cluster1-control-plane
    rules:
      - dst: 10.80.0.2/32
        routes:
          - dev: liqo.jdr5xndgmb
            dst: 10.80.0.2/32
            scope: link
      - iif: liqo-tunnel
        routes:
          - dst: 10.112.0.229/32
            gw: 10.80.0.2
            targetRef:
              kind: Pod
              name: coredns-9ff4c5cf6-xbx5w
              namespace: kube-system
              uid: 3cb83b91-98b5-412c-b5a2-f1ebe28497df
```

### \<local-cluster-id\>-gw-ext (Gateway)

This route configuration contains all the routes that forward traffic from a gateway to another. It includes rules for remote pod CIDRs and external CIDRs.

Note that the routes with a destination of 10.70.0.0/16 are related to the external CIDR. It may seem strange since it is not using a remapped CIDR, but this is because the DNAT rules (which translate from remapped CIDR to original CIDR) act in **prerouting**.

```yaml
apiVersion: networking.liqo.io/v1beta1
kind: RouteConfiguration
metadata:
  labels:
    liqo.io/managed: "true"
    networking.liqo.io/route-category: gateway
    networking.liqo.io/route-unique: cheina-cluster2
  name: cheina-cluster2-gw-ext
  namespace: liqo-tenant-cheina-cluster2
spec:
  table:
    name: cheina-cluster2
    rules:
      - dst: 10.122.0.0/16
        iif: liqo.fgckffk4dv
        routes:
          - dst: 10.122.0.0/16
            gw: 169.254.18.1
      - dst: 10.70.0.0/16
        iif: liqo.fgckffk4dv
        routes:
          - dst: 10.70.0.0/16
            gw: 169.254.18.1
      - dst: 10.122.0.0/16
        iif: liqo.jdr5xndgmb
        routes:
          - dst: 10.122.0.0/16
            gw: 169.254.18.1
      - dst: 10.70.0.0/16
        iif: liqo.jdr5xndgmb
        routes:
          - dst: 10.70.0.0/16
            gw: 169.254.18.1
      - dst: 10.122.0.0/16
        iif: liqo.cjntnn4bdj
        routes:
          - dst: 10.122.0.0/16
            gw: 169.254.18.1
      - dst: 10.70.0.0/16
        iif: liqo.cjntnn4bdj
        routes:
          - dst: 10.70.0.0/16
            gw: 169.254.18.1
```

### \<local-cluster-id\>-node-gw (Node)

This route configuration contains the routes to reach the "other" side of the Geneve tunnel. It also includes all the routes that point to the remote cluster's pod CIDR and external CIDR.

```yaml
apiVersion: networking.liqo.io/v1beta1
kind: RouteConfiguration
metadata:
  labels:
    liqo.io/managed: "true"
    networking.liqo.io/route-category: fabric
  name: cheina-cluster2-node-gw
  namespace: liqo-tenant-cheina-cluster2
spec:
  table:
    name: cheina-cluster2-node-gw
    rules:
      - dst: 10.80.0.4/32
        routes:
          - dev: liqo.7hr82v9br5
            dst: 10.80.0.4/32
            scope: link
      - dst: 10.68.0.0/16
        routes:
          - dst: 10.68.0.0/16
            gw: 10.80.0.4
      - dst: 10.71.0.0/18
        routes:
          - dst: 10.71.0.0/18
            gw: 10.80.0.4
```
