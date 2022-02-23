---
title: Teardown the Playground
weight: 9
---

## Uninstall steps

This procedure uninstalls Liqo from your cluster (also Liqo CRDs are automatically purged):

```bash
export KUBECONFIG=$KUBECONFIG_1
helm uninstall liqo -n liqo
kubectl get namespace | grep "liqo" | cut -d " " -f1 | xargs -IC kubectl delete namespace C
kubectl get crds | grep -e "liqo" | cut -d " " -f1 | xargs -IC kubectl delete crd C
```

Repeat this procedure for the other two clusters.

## Destroy clusters

To teardown the kind clusters:

```bash
kind delete cluster --name cluster1
kind delete cluster --name cluster2
kind delete cluster --name cluster3
```
