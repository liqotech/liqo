---
title: Scheduling
weight: 3
---

## Customizing the Kubernetes scheduling logic

The default Kubernetes scheduler is not Liqo-aware, hence we may need to use some tricks to force the scheduler to start a pod on a given cluster.

### Default scheduling behavior
By default, the Kubernetes scheduler selects the node with the highest free resources.
Given that the virtual node summarizes all the resources shared by a given foreign cluster (no matter how many remote physical nodes are involved), is very likely that the above node will be perceived as *fatter* than any physical node available locally. Hence, very likely, new pods will be scheduled on that node.

However, in general, you cannot know which node (either local, or in the foreign cluster) will be selected: it simply depends on the amount of available resources.

To schedule a pod on a given cluster, you have to follow one of the options below.

### Scheduling a pod in a remote cluster using the 'liqo.io/enabled' label

First, you need to configure a Kubernetes namespace that spans also across foreign clusters, which can be achieved by setting the `liqo.io/enabled=true label`, as follows (which refers to namespace `test-liqo`):

```
# Create a new namespace named 'test-liqo'
kubectl create ns test-liqo
# Associate the 'liqo.io/enabled' label to the above namespace
kubectl label ns test-liqo liqo.io/enabled=true
```

Second, you need to start a pod whose specification includes the `nodeSelector` tag set to `virtual-node`, as follows:

```
apiVersion: v1
kind: Pod
metadata:
  name: nginx
  labels:
    app: test-liqo
spec:
  containers:
  - name: nginx
    image: nginxdemos/hello
    imagePullPolicy: IfNotPresent
    ports:
      - containerPort: 80
        name: web
  nodeSelector:
    type: virtual-node
```

<!-- TODO  It looks there's a limitation here. If I'm connected to *two* foreign cluster, how can I specify exactly which *one* I have to use? -->

<!-- TODO  How can I start two services that talk to each other, one in my cluster, the second in the foreign cluster? -->


### Scheduling a pod in a remote cluster using the 'taint' mechanism

<!-- TODO Sorry, please tell something more, I cannot understand this text -->

* add a toleration for taints:
```
    taints:
    - effect: NoExecute
      key: virtual-node.liqo.io/not-allowed
      value: "true"
```



