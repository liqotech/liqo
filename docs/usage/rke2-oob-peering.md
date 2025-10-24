# RKE2 Out-of-Band Peering

This section describes how to establish peering between RKE2 clusters in restricted networks where `liqoctl peer` cannot be used (e.g., different networks, security policies, GitOps workflows).

## Overview

Out-of-band peering manually creates `ForeignCluster` resources on each cluster, enabling peering without direct cluster-to-cluster communication during setup.
This approach is essential when:

- Clusters cannot directly communicate
- Using declarative GitOps workflows
- Different organizations manage each cluster

## Prerequisites

- Two RKE2 clusters with Liqo installed
- Secure method to exchange configuration (e.g., shared storage, secure transfer)

```{admonition} Note
For standard peering where both clusters are accessible, use `liqoctl peer` instead. See [peer two clusters](/usage/peer).
```

## Install Liqo on both clusters

First, install Liqo with explicit cluster IDs:

```bash
# Consumer cluster
liqoctl install rke2 --cluster-id consumer-rke2

# Provider cluster
liqoctl install rke2 --cluster-id provider-rke2
```

## Create ForeignCluster resources

### On the consumer

Create a `ForeignCluster` representing the provider:

```yaml
apiVersion: core.liqo.io/v1beta1
kind: ForeignCluster
metadata:
  name: provider-rke2
  labels:
    liqo.io/remote-cluster-id: provider-rke2
spec:
  clusterID: provider-rke2
  modules:
    networking: {enabled: true}
    authentication: {enabled: true}
    offloading: {enabled: true}
```

```bash
kubectl apply -f consumer-foreigncluster.yaml
```

### On the provider

Create a `ForeignCluster` representing the consumer:

```yaml
apiVersion: core.liqo.io/v1beta1
kind: ForeignCluster
metadata:
  name: consumer-rke2
  labels:
    liqo.io/remote-cluster-id: consumer-rke2
spec:
  clusterID: consumer-rke2
  modules:
    networking: {enabled: true}
    authentication: {enabled: true}
    offloading: {enabled: true}
```

```bash
kubectl apply -f provider-foreigncluster.yaml
```

```{admonition} Important
The `liqo.io/remote-cluster-id` label improves lookup performance (O(1) vs O(n)).
```

## Exchange credentials

The peering modules require manual credential exchange. Refer to the individual module documentation:

- [Networking](/advanced/peering/inter-cluster-network) - Gateway configuration
- [Authentication](/advanced/peering/inter-cluster-authentication) - Identity exchange
- [Offloading](/advanced/peering/offloading-in-depth) - ResourceSlice creation

## Verify peering

Check the `ForeignCluster` status:

```bash
# Consumer
kubectl get foreigncluster -n liqo provider-rke2 -o yaml
kubectl get nodes -l liqo.io/type=virtual-node

# Provider
kubectl get foreigncluster -n liqo consumer-rke2 -o yaml
kubectl get resourceslice -n liqo
```

## GitOps integration

Store `ForeignCluster` manifests in Git and apply via your GitOps operator:

```
gitops-repo/
├── clusters/
│   ├── consumer-rke2/
│   │   └── foreignclusters/
│   │       └── provider-rke2.yaml
│   └── provider-rke2/
│       └── foreignclusters/
│           └── consumer-rke2.yaml
```

## Troubleshooting

**ForeignCluster not found:**

```bash
# Verify name matches cluster ID
kubectl get foreigncluster -A

# Add label if missing
kubectl label foreigncluster <name> liqo.io/remote-cluster-id=<cluster-id> -n liqo
```

**Networking issues:**

```bash
kubectl get gatewayserver,gatewayclient -n liqo
kubectl logs -n liqo -l app.kubernetes.io/component=liqo-fabric
```

**Authentication failures:**

```bash
kubectl get identity -n liqo
kubectl logs -n liqo -l app.kubernetes.io/name=controller-manager
```
