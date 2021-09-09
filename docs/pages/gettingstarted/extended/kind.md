---
title: Set up the Playground
weight: 1
---

### Deploy clusters

After having installed the [requirements](/gettingstarted#system-requirements), you can launch the clusters script:

```bash
// script
```

The script downloads and executes the [Kind](https://kind.sigs.k8s.io) tool to create single-node clusters. 

### Explore the Playground (Optional)

You can inspect the deployed clusters by typing:

```bash
kind get clusters
```

You should see three entries:

```bash
cluster1
cluster2
cluster3
```

This means that three kind clusters are deployed and running on your host.

You can inspect the clusters' status. 
To do so, you can export the **KUBECONFIG** variable to specify the identity file for *kubectl* and then contact the clusters.
By default, the kubeconfigs of the clusters are saved in the current directory (“*./liqo_kubeconf_1* ”, “*./liqo_kubeconf_2* ”, “*./liqo_kubeconf_3* ”). 
You can export them as environment variables:

```bash
CURRENT_DIRECTORY=$(pwd)
KUBECONFIG_1=${CURRENT_DIRECTORY}/liqo_kubeconf_1
KUBECONFIG_2=${CURRENT_DIRECTORY}/liqo_kubeconf_2
KUBECONFIG_3=${CURRENT_DIRECTORY}/liqo_kubeconf_3
```

Now you can get the available pods on the first cluster:

```bash
export KUBECONFIG=$KUBECONFIG_1
kubectl get pods -A
```

If each cluster provides an output similar to this, it means that the three kind clusters are correctly running on your host.

```bash
NAMESPACE            NAME                                             READY   STATUS    
kube-system          coredns-f9fd979d6-c95ss                          1/1     Running            
kube-system          coredns-f9fd979d6-dggxr                          1/1     Running             
kube-system          etcd-cluster1-control-plane                      1/1     Running             
kube-system          kindnet-c5654                                    1/1     Running             
kube-system          kube-apiserver-cluster1-control-plane            1/1     Running             
kube-system          kube-controller-manager-cluster1-control-plane   1/1     Running             
kube-system          kube-proxy-qqtvc                                 1/1     Running             
kube-system          kube-scheduler-cluster1-control-plane            1/1     Running             
local-path-storage   local-path-provisioner-78776bfc44-scbxl          1/1     Running             
```

You can move forward to the next step: [the Liqo installation](../install).
