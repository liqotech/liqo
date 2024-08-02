# External IP remapping

You can use liqo to map external IPs and make them reachable from a peered cluster. You can configure the external IP remapping using the **IP** CRD.

```{warning}
This feature is available only if [network module](/advanced/manual-peering.md) is enabled.
```

Check the figure below to understand how the external IP remapping works.
We are going to make the **external host** reachable from **cluster 1**.

```{figure} /_static/images/advanced/ipremap/ipremap.drawio.svg
---
align: center
---
Remap External IPs
```

## Forge an IP CRD

Export the kubeconfig file of **cluster 2**:

```bash
export KUBECONFIG=./cluster2-kubeconfig
```

First of all, you need to create a file called **ip.yaml**.

```yaml
apiVersion: ipam.liqo.io/v1alpha1
kind: IP
metadata:
  name: external-ip-remap
spec:
  ip: <EXTERNAL_IP>
```

Replace `<EXTERNAL_IP>` with the **external host** you want to map.

Now, apply the **IP** CRD:

```bash
kubectl apply -f ip.yaml
```

Check the status of the **IP** CRD:

```bash
kubectl get ip external-ip-remap -o yaml
```

The status should be similar to the following:

```yaml
apiVersion: ipam.liqo.io/v1alpha1
kind: IP
...
status:
  ipMappings:
    cluster1: <REMAPPED_IP>

```

The **status** field shows how the **external host** has been remapped.
We are going to use the **remapped IP** on **cluster 1** to reach the **external host**.

## Connect to the *external host*

First of all, export the kubeconfig file of **cluster 1**:

```bash
export KUBECONFIG=./cluster1-kubeconfig
```

Get the **configuration** CRD for **cluster 2**:

```bash
kubectl get configuration -n liqo -o yaml cluster2
```

The output should be similar to the following:

```yaml
apiVersion: networking.liqo.io/v1beta1
kind: Configuration
metadata:
  labels:
    configuration.liqo.io/configured: "true"
    liqo.io/remote-cluster-id: cluster2
  name: cluster2
  namespace: liqo-tenant-cluster2
spec:
...
status:
  remote:
    cidr:
      external: <REMAPPED_EXT_CIDR>
      pod: <REMAPPED_POD_CIDR>
```

Lets focus on the `REMAPPED_EXT_CIDR` value. Keep the *prefix* of that CIDR and replace it inside the `REMAPPED_IP` found in the **IP** CRD status (check the previous section).

For example, if the `REMAPPED_EXT_CIDR` is *10.81.0.0/16* and the `REMAPPED_IP` is *10.70.0.1* the final IP will be *10.81.0.1*.

Now, you can use the **forged IP** to reach the **external host** from **cluster 1**.
