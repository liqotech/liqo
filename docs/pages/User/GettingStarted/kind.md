---
title: Provision the Liqo Playground
weight: 1
---

Before testing Liqo, we should create a pair of clusters to use it.

### Requirements

The Liqo playground set-up has the following requirements:

* You should have installed [Docker](https://docker.io) on your machine. The following guide is also compatible with Mac Os X and Windows (with Docker Desktop).

* [Kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/), the official Kubernetes client, is not mandatory for this step, but is strongly recommended to explore the playground and for next steps.

* `curl` can be easily installed on linux and Mac Os X. For example on Ubuntu, curl can be simply installed by typing:
`sudo apt update && sudo apt install -y curl`

## Deploy the clusters

Let's open a terminal and launch the clusters scripts:

```bash
source <(curl -L https://get.liqo.io/clusters.sh)
```

The script will download and execute the [Kind](https://kind.sigs.k8s.io) tool to create a pair of clusters. Those clusters are composed of two nodes each (one for the control plane and one as a simple worker).

### Explore the playground (Optional)

You can inspect the deployed clusters by typing:

```
kind get clusters
```

You should see a couple of entries:

```
cluster1
cluster2
```

This means that 2 kind clusters are deployed and running on your host.

Then, you can simply inspect the status of the clusters. To do so, you can export the `KUBECONFIG` variable to specify the identity file for kubectl and then contact the cluster.

By default, the kubeconfigs of the two clusters are saved in the current directory (`./liqo_kubeconf_1`, `./liqo_kubeconf_2`) and both are already exported as environment variables (`KUBECONFIG_1`,`KUBECONFIG_2`).

For example, on the first cluster, you can get the available pods by merely typing:

```bash
export KUBECONFIG=$KUBECONFIG_1
kubectl get pods -A
```

Similarly, on the second cluster, you can observe the pods in execution:

```bash
export KUBECONFIG=$KUBECONFIG_2
kubectl get pods -A
```

```
netlab@cloud-docker:~$ kubectl get po -A
NAMESPACE            NAME                                             READY   STATUS    RESTARTS   AGE
kube-system          coredns-f9fd979d6-6mmhl                          1/1     Running   0          57m
kube-system          coredns-f9fd979d6-szfwc                          1/1     Running   0          57m
kube-system          etcd-cluster1-control-plane                      1/1     Running   0          57m
kube-system          kindnet-8tg8s                                    1/1     Running   0          57m
kube-system          kindnet-whcfm                                    1/1     Running   0          57m
kube-system          kube-apiserver-cluster1-control-plane            1/1     Running   0          57m
kube-system          kube-controller-manager-cluster1-control-plane   1/1     Running   0          57m
kube-system          kube-proxy-88m2g                                 1/1     Running   0          57m
kube-system          kube-proxy-zctxs                                 1/1     Running   0          57m
kube-system          kube-scheduler-cluster1-control-plane            1/1     Running   0          57m
local-path-storage   local-path-provisioner-78776bfc44-rk58g          1/1     Running   0          57m
```

If the above commands return each output similar to this, your clusters are up and ready.

Now, you can move forward to the [next step](../install): the installation of Liqo.
