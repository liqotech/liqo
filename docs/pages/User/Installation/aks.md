---
title: AKS
weight: 1
---

This guide will show you how to install Liqo on your AKS cluster

### Requirements

* an [Azure account](https://azure.microsoft.com/auth/signin/?loginProvider=Microsoft&redirectUri=%2Fen-us%2F)

### Steps

* [Create an AKS Cluster](#create-an-aks-cluster)
* [Enable required features](#enable-required-features)
* [Deploy Liqo](#deploy-liqo)
* [Check that Liqo is Running](#check-that-liqo-is-running)

## Create an AKS Cluster

### Access Azure Portal

Access in the Azure [Portal](https://portal.azure.com/) to the [Kubernetes Service](https://portal.azure.com/#blade/HubsExtension/BrowseResource/resourceType/Microsoft.ContainerService%2FmanagedClusters).

![](/images/install/aks/01.png)

### Create a new Cluster

Click on `Add` > `Add Kuberntes cluster` to create a new cluster. A new panel will appear.

Select the desired `Subscription` and `Resource Group`, choose a name, a region and an availability zone to assign to
your cluster.

__NOTE__: Liqo only supports a `Kubernetes version` >= 1.19.0

Liqo does not require any other configurations to the cluster. You can click on the `Review + create` button.

![](/images/install/aks/02.png)

When the validation is passed, click on the `Create` button.

![](/images/install/aks/03.png)

Azure will take some minutes to deploy your cluster.

## Enable required features

### Enable HTTP Application Routing

When your cluster has been completely deployed, you have to enable an ingress controller to make the Liqo Auth Service
accessible from the external world.

Azure has a built-in plugin that enable this feature, although it is not recommended for use in production, called
[HTTP application routing](https://docs.microsoft.com/en-US/azure/aks/http-application-routing).

To enable it on your cluster dashboard go in `Settings` > `Networking` and make sure that the _Enable HTTP application
routing_ checkbox is enabled and click on the `Save` button.

![](/images/install/aks/04.png)

Azure will take some minutes to deploy required components and enable the required services.

When the operation is completed you will see a new [DNS zone](https://portal.azure.com/#blade/HubsExtension/BrowseResource/resourceType/Microsoft.Network%2FdnsZones)
in the Azure Portal.

![](/images/install/aks/05.png)

Congratulations! Your AKS cluster is now ready to run Liqo!

## Deploy Liqo

### One Line installer

#### Export Variables

The Liqo one line installer needs some environment variables to know where the Liqo components are accessible from the
external world and how the local Kubernetes installation is configured.

In particular, we have to export the following environment variables:

| Variable               | Default | Description                                 |
| ---------------------- | ------- | ------------------------------------------- |
| `POD_CIDR`             |         | the cluster Pod CIDR                        |
| `SERVICE_CIDR`         |         | the cluster Service CIDR                    |
| `LIQO_INGRESS_CLASS`   |         | the [ingress class](https://kubernetes.io/docs/concepts/services-networking/ingress/#ingress-class) to be used by the Auth Service Ingress |
| `LIQO_APISERVER_ADDR`  |         | the hostname where to access the API server |
| `LIQO_APISERVER_PORT`  | `6443`  | the port where to access the API server     |
| `LIQO_AUTHSERVER_ADDR` |         | the hostname where to access the Auth Service, the one exposed with the ingress, if it is not set the service will be exposed with a [NodePort Service](https://kubernetes.io/docs/concepts/services-networking/service/#nodeport) instead of an [Ingress](https://kubernetes.io/docs/concepts/services-networking/ingress/) |
| `LIQO_AUTHSERVER_PORT` | `443`   | the port where to access the Auth Service   |

#### How can I know those variable values in AKS?

Some variables are the same for each AKS cluster.

| Variable               | Value                          | Notes                                  |
| ---------------------- | ------------------------------ | -------------------------------------- |
| `POD_CIDR`             | 10.244.0.0/16                  |                                        |
| `SERVICE_CIDR`         | 10.0.0.0/16                    |                                        |
| `LIQO_INGRESS_CLASS`   | addon-http-application-routing | If you enabled the built-in AKS plugin |
| `LIQO_APISERVER_PORT`  | 443                            |                                        |
| `LIQO_AUTHSERVER_PORT` | 443                            |                                        |

The other two values can be found in the Azure Portal.

The `LIQO_APISERVER_ADDR` con be found in our cluster overview as __API server address__

![](/images/install/aks/06.png)

The `LIQO_AUTHSERVER_ADDR` is where the Liqo Auth Service will be reachable, so we have to export some hostname that we
can manage. If you are using the AKS HTTP Application Routing a DNS zone should be available, so you can create a subdomain
on it.

![](/images/install/aks/07.png)

In this case a viable hostname would be `auth.f83c28d9ce1449b2bb45.westeurope.aksapp.io`

#### Install Liqo

After this configuration step we can [run the Liqo one line installer](/user/gettingstarted/install/#default-install)
as usual.

If our cluster is using a private address space, when Liqo is deployed, we need a last configuration step required. We have to change the `liqo-gateway-endpoint` service type from
`NodePort` to `LoadBalancer` to make it reachable.

```bash
kubectl patch service -n liqo liqo-gateway-endpoint \
    --patch '{"spec":{"type":"LoadBalancer"}}'
```

#### Example

Export variables

```bash
export POD_CIDR=10.244.0.0/16
export SERVICE_CIDR=10.0.0.0/16
export LIQO_INGRESS_CLASS=addon-http-application-routing
export LIQO_APISERVER_PORT=443
export LIQO_APISERVER_ADDR=liqo-dns-6f1bc41d.hcp.westeurope.azmk8s.io
export LIQO_AUTHSERVER_ADDR=auth.f83c28d9ce1449b2bb45.westeurope.aksapp.io
```

Install Liqo

```bash
curl -sL https://get.liqo.io | bash
```

Make the gateway reachable

```bash
kubectl patch service -n liqo liqo-gateway-endpoint \
    --patch '{"spec":{"type":"LoadBalancer"}}'
```

## Check that Liqo is Running

Wait that all Liqo pods are up and running

```bash
kubectl get pods -n liqo
```

![](/images/install/aks/08.png)

### Access the cluster configurations

You can get the cluster configurations from the Auth Service endpoint to check that this service has been correctly deployed

```bash
curl --insecure https://auth.f83c28d9ce1449b2bb45.westeurope.aksapp.io/ids
```

```json
{"clusterId":"4e33e2dc-b2e3-4052-9a74-d18f6d8cdab2","clusterName":"LiqoCluster5852","guestNamespace":"liqo"}
```

Congratulations! Liqo is now up and running on your AKS cluster, you can now peer with other Liqo instances!

### Establish a Peering

The Auth Service URL is the only required value to make this cluster peerable from the external world.

You can add a `ForeignCluster` resource in any other cluster where Liqo is installed to be able to join your cluster.

An example of this resource can be:

```yaml
apiVersion: discovery.liqo.io/v1alpha1
kind: ForeignCluster
metadata:
  name: my-aks-cluster
spec:
  authUrl: "https://auth.f83c28d9ce1449b2bb45.westeurope.aksapp.io"
```

When the CR will be created the Liqo control plane will contact the URL shown in the step before with the curl command to
retrieve all the required cluster information.
