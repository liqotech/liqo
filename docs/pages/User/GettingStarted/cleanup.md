---
title: Clean-up 
weight: 5
---


## Clean-up

You can remove all the deployed examples with:

```
kubectl delete -f https://raw.githubusercontent.com/LiqoTech/liqo/master/docs/examples/hello-world.yaml -n test-liqo
```

And:

```
kubectl delete -f https://github.com/LiqoTech/microservices-demo/blob/master/release/kubernetes-manifests.yaml -n test-liqo
```
