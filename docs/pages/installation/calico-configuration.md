---
title: Calico configuration
weight: 4
---

When installed, Liqo adds several interfaces to a cluster to handle cross-cluster traffic routing.
Those interfaces are intended to not interfere with the CNI normal job.

However, Calico scans existing interfaces on a node to detect network configuration and establish the correct routes.
To make everything work well, Calico should ignore Liqo interfaces. This change is required only if you have Calico configured in BGP mode.
If you use VPC native setup, you are not required to do this modification.

To do so, In Calico v3.17, you should patch the Installation CR by adding in `.spec.calicoNetwork.nodeAddressAutodectection.skipInterface` with `liqo.*`.
An example of the patched resource is available here:

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

```bash
kubectl edit daemonset -n kube-system calico-node
```

And then, you should add/edit the environment variables of the calico-node container.

```yaml
        - name: IP_AUTODETECTION_METHOD
          value: skip-interface=liqo.*
```
