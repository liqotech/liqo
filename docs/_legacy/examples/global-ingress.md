# Global Ingress

In this tutorial, you will learn how to leverage Liqo and [K8GB](https://www.k8gb.io/) to deploy and expose a multi-cluster application through a *global ingress*.
More in detail, this enables improved load balancing and distribution of the external traffic towards the application replicated across multiple clusters.

The figure below outlines the high-level scenario, with a client consuming an application from either cluster 1 (e.g., located in EU) or cluster 2 (e.g., located in the US), based on the endpoint returned by the DNS server.

![Global Ingress Overview](/_static/images/examples/global-ingress/global-ingress-overview.drawio.svg)

## Provision the playground

First, check that you are compliant with the [requirements](/examples/requirements.md).
Additionally, this example requires [k3d](https://k3d.io/v5.4.1/#installation) to be installed in your system.
Specifically, this tool is leveraged instead of KinD to match the [K8GB Sample Demo](https://www.k8gb.io/docs/local.html#sample-demo).

To provision the playground, clone the [Liqo repository](https://github.com/liqotech/liqo) and run the setup script:

{{ env.config.html_context.generate_clone_example('global-ingress') }}

The setup script creates three k3s clusters and deploys the appropriate infrastructural application on top of them, as detailed in the following:

* **edgedns**: this cluster will be used to deploy the DNS service.
  In a production environment, this should be an external DNS service (e.g. AWS Route53).
  It includes the Bind Server (manifests in `manifests/edge` folder).
* **gslb-eu** and **gslb-us**: these clusters will be used to deploy the application.
  They include:
  * [ExternalDNS](https://github.com/kubernetes-sigs/external-dns): it is responsible for configuring the DNS entries.
  * [Ingress Nginx](https://kubernetes.github.io/ingress-nginx/): it is responsible for handling the local ingress traffic.
  * [K8GB](https://www.k8gb.io/): it configures the multi-cluster ingress.
  * [Liqo](/index.md): it enables the application to spread across multiple clusters, and takes care of reflecting the required resources.

Export the kubeconfigs environment variables to use them in the rest of the tutorial:

```bash
export KUBECONFIG_DNS=$(k3d kubeconfig write edgedns)
export KUBECONFIG=$(k3d kubeconfig write gslb-eu)
export KUBECONFIG_US=$(k3d kubeconfig write gslb-us)
```

```{admonition} Note
We suggest exporting the kubeconfig of the *gslb-eu* as default (i.e., `KUBECONFIG`), since it will be the entry point of the virtual cluster and you will mainly interact with it.
```

## Peer the clusters

Once Liqo is installed in your clusters, you can establish new *peerings*.

Specifically, to implement the desired scenario, you should enable a peering from the *gslb-eu* cluster to the *gslb-us* cluster.
This will allow Liqo to [offload workloads and reflect services](/features/offloading.md) from the first cluster to the second cluster.

To proceed, first we need to peer the *gslb-eu* cluster with *gslb-us*:

```bash
liqoctl peer --remote-kubeconfig "$KUBECONFIG_US" --gw-server-service-type NodePort
```

```{admonition} Note
As no LoadBalancer is configured in the example environments, we need to expose the Liqo gateway with a `NodePort` service.
```

When the above command returns successfully, you can check the peering status by running:

```bash
kubectl get foreignclusters
```

The output should look like the following, indicating that a peering is currently active towards the *gslb-us* cluster:

```text
NAME      ROLE       AGE
gslb-us   Provider   32s
```

Additionally, you should see a new virtual node (`liqo-gslb-us`) in the *gslb-eu* cluster, and representing the whole *gslb-us* cluster.
Every pod scheduled onto this node will be automatically offloaded to the remote cluster by Liqo.

```bash
kubectl get node --selector=liqo.io/type=virtual-node
```

The output should be similar to:

```text
NAME           STATUS   ROLES   AGE   VERSION
liqo-gslb-us   Ready    agent   14s   v1.30.2+k3s2
```

## Deploy an application

Now that the Liqo peering is established, and the virtual node is ready, it is possible to proceed deploying the [*podinfo*](https://github.com/stefanprodan/podinfo) demo application.
This application serves a web-page showing different information, including the name of the pod; hence, easily identifying which replica is generating the HTTP response.

First, create a hosting namespace in the *gslb-eu* cluster, and offload it to the remote cluster through Liqo.

```bash
kubectl create namespace podinfo
liqoctl offload namespace podinfo --namespace-mapping-strategy EnforceSameName
```

At this point, it is possible to deploy the *podinfo* helm chart in the `podinfo` namespace:

```bash
helm install podinfo --namespace podinfo \
    podinfo/podinfo -f manifests/values/podinfo.yaml
```

This chart creates a *Deployment* with a *custom affinity* to ensure that the two frontend replicas are scheduled on different nodes and clusters:

```yaml
affinity:
  nodeAffinity:
    requiredDuringSchedulingIgnoredDuringExecution:
      nodeSelectorTerms:
      - matchExpressions:
        - key: node-role.kubernetes.io/control-plane
          operator: DoesNotExist
  podAntiAffinity:
    requiredDuringSchedulingIgnoredDuringExecution:
    - labelSelector:
        matchExpressions:
        - key: app.kubernetes.io/name
          operator: In
          values:
          - podinfo
      topologyKey: "kubernetes.io/hostname"
```

Additionally, it creates an *Ingress* resource configured with the `k8gb.io/strategy: roundRobin` annotation.
This annotation will [instruct the K8GB Global Ingress Controller](https://www.k8gb.io/docs/ingress_annotations.html) to distribute the traffic across the different clusters.

## Check application spreading

Let's now check that Liqo replicated the ingress resource in both clusters and that each *Nginx Ingress Controller* was able to assign them the correct IPs (different for each cluster).

```{admonition} Note
You can see the output for the second cluster appending the `--kubeconfig $KUBECONFIG_US` flag to each command.
```

```bash
kubectl get ingress -n podinfo
```

The output in the *gslb-eu* cluster should be similar to:

```text
NAME      CLASS   HOSTS                    ADDRESS                 PORTS   AGE
podinfo   nginx   liqo.cloud.example.com   172.19.0.3,172.19.0.4   80      6m9s
```

```bash
kubectl get ingress -n podinfo --kubeconfig $KUBECONFIG_US
```

While the output in the *gslb-us* cluster should be similar to:

```text
NAME      CLASS   HOSTS                    ADDRESS                 PORTS   AGE
podinfo   nginx   liqo.cloud.example.com   172.19.0.5,172.19.0.6   80      6m16s
```

With reference to the output above, the `liqo.cloud.example.com` hostname is served in the demo environment on:

* `172.19.0.3`, `172.19.0.4`: addresses exposed by cluster *gslb-eu*
* `172.19.0.5`, `172.19.0.6`: addresses exposed by cluster *gslb-us*

Each local *K8GB* installation creates a *Gslb* resource with the Ingress information and the given strategy (*RoundRobin* in this case), and *ExternalDNS* populates the DNS records accordingly.

On the *gslb-eu* cluster, the command:

```bash
kubectl get gslbs.k8gb.absa.oss -n podinfo podinfo -o yaml
```

should return an output along the lines of:

```yaml
apiVersion: k8gb.absa.oss/v1beta1
kind: Gslb
metadata:
  annotations:
    k8gb.io/strategy: roundRobin
  name: podinfo
  namespace: podinfo
spec:
  ingress:
    ingressClassName: nginx
    rules:
    - host: liqo.cloud.example.com
      http:
        paths:
        - backend:
            service:
              name: podinfo
              port:
                number: 9898
          path: /
          pathType: ImplementationSpecific
  strategy:
    dnsTtlSeconds: 30
    splitBrainThresholdSeconds: 300
    type: roundRobin
status:
  geoTag: eu
  healthyRecords:
    liqo.cloud.example.com:
    - 172.19.0.3
    - 172.19.0.4
    - 172.19.0.5
    - 172.19.0.6
  serviceHealth:
    liqo.cloud.example.com: Healthy
```

Similarly, when issuing the command from the *gslb-us* cluster:

```bash
kubectl get gslbs.k8gb.absa.oss -n podinfo podinfo -o yaml --kubeconfig $KUBECONFIG_US
```

```yaml
apiVersion: k8gb.absa.oss/v1beta1
kind: Gslb
metadata:
  annotations:
    k8gb.io/strategy: roundRobin
  name: podinfo
  namespace: podinfo
spec:
  ingress:
    ingressClassName: nginx
    rules:
    - host: liqo.cloud.example.com
      http:
        paths:
        - backend:
            service:
              name: podinfo
              port:
                number: 9898
          path: /
          pathType: ImplementationSpecific
  strategy:
    dnsTtlSeconds: 30
    splitBrainThresholdSeconds: 300
    type: roundRobin
status:
  geoTag: us
  healthyRecords:
    liqo.cloud.example.com:
    - 172.19.0.5
    - 172.19.0.6
    - 172.19.0.3
    - 172.19.0.4
  serviceHealth:
    liqo.cloud.example.com: Healthy
```

In both clusters, the *Gslb* resources are pretty identical; they only differ for the *geoTag* field.
The resource status also reports:

* the *serviceHealth* status, that should be *Healthy* for both clusters
* the list of IPs exposing the HTTP service: they are the IPs of the nodes of both clusters since the *Nginx Ingress Controller* is deployed in *HostNetwork DaemonSet* mode.

## Check service reachability

Since *podinfo* is an HTTP service, you can contact it using the *curl* command.
Use the `-v` option to understand which of the nodes is being targeted.

You need to use the DNS server in order to resolve the hostname to the IP address of the service.
To this end, create a pod in one of the clusters (it does not matter which one) overriding its DNS configuration.

```bash
HOSTNAME="liqo.cloud.example.com"
K8GB_COREDNS_IP=$(kubectl get svc k8gb-coredns -n k8gb -o custom-columns='IP:spec.clusterIP' --no-headers)

kubectl run -it --rm curl --restart=Never --image=curlimages/curl:7.82.0 --command \
    --overrides "{\"spec\":{\"dnsConfig\":{\"nameservers\":[\"${K8GB_COREDNS_IP}\"]},\"dnsPolicy\":\"None\"}}" \
    -- curl $HOSTNAME -v
```

```{admonition} Note
Launching this pod several times, you will see different IPs and different frontend pods answering in a round-robin fashion (as set in the *Gslb* policy).
```

```text
*   Trying 172.19.0.3:80...
* Connected to liqo.cloud.example.com (172.19.0.3) port 80 (#0)
...
{
  "hostname": "podinfo-67f46d9b5f-xrbmg",
  "version": "6.1.4",
  "revision": "",
...
```

```text
*   Trying 172.19.0.6:80...
* Connected to liqo.cloud.example.com (172.19.0.6) port 80 (#0)
...
{
  "hostname": "podinfo-67f46d9b5f-xrbmg",
  "version": "6.1.4",
  "revision": "",
...
```

```text
*   Trying 172.19.0.3:80...
* Connected to liqo.cloud.example.com (172.19.0.3) port 80 (#0)
...
{
  "hostname": "podinfo-67f46d9b5f-cmnp5",
  "version": "6.1.4",
  "revision": "",
...
```

This brief tutorial showed how you could leverage Liqo and *K8GB* to deploy and expose a multi-cluster application.
In addition to the *RoundRobin* policy, which provides load distribution among clusters, *K8GB* allows favoring closer endpoints (through the *GeoIP* strategy), or adopt a *Failover* policy.
Additional details are provided in its [official documentation](https://www.k8gb.io/docs/strategy.html).

## Tear down the playground

Our example is finished; now we can remove all the created resources and tear down the playground.

### Unoffload namespaces

Before starting the uninstallation process, make sure that all namespaces are unoffloaded:

```bash
liqoctl unoffload namespace podinfo
```

Every pod that was offloaded to a remote cluster is going to be rescheduled onto the local cluster.

### Revoke peerings

Similarly, make sure that all the peerings are revoked:

```bash
liqoctl unpeer out-of-band gslb-us
```

At the end of the process, the virtual node is removed from the local cluster.

### Uninstall Liqo

Now you can remove Liqo from your clusters with *liqoctl*:

```bash
liqoctl uninstall
liqoctl uninstall --kubeconfig="$KUBECONFIG_US"
```

```{admonition} Purge
By default the Liqo CRDs will remain in the cluster, but they can be removed with the `--purge` flag:

```bash
liqoctl uninstall --purge
liqoctl uninstall --kubeconfig="$KUBECONFIG_US" --purge
```

### Destroy clusters

To teardown the k3d clusters, you can issue:

```bash
k3d cluster delete gslb-eu gslb-us edgedns
```
