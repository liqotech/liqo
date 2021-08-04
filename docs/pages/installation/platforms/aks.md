---
title: AKS
weight: 2
---

#### About AKS

Azure Kubernetes Service (AKS) is a managed Kubernetes service available on the Microsoft Azure public cloud.

#### Scenarios

This guide will show you how to install Liqo on your AKS cluster. AKS clusters have by default an Internet-exposed API Server and can easily expose LoadBalancer services using public IPs. As discussed in [Scenarios](/installation/pre-install) section, those latter are the requirements to have a "public-exposed" cluster, which can be accessed by other Liqo instances.

Liqo may be installed on newly created clusters or existing ones.

### Installation

### Access Azure Portal

Access in the Azure [Portal](https://portal.azure.com/) to the [Kubernetes Service](https://portal.azure.com/#blade/HubsExtension/BrowseResource/resourceType/Microsoft.ContainerService%2FmanagedClusters).

![](/images/install/aks/01.png)

### Create a new Cluster

Click on `Add` > `Add Kubernetes cluster` to create a new cluster. A new panel will appear.

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

Azure has a built-in plugin that enables this feature, although it is not recommended for use in production, called
[HTTP application routing](https://docs.microsoft.com/en-US/azure/aks/http-application-routing).

To enable it on your cluster dashboard, you should go in `Settings` > `Networking` and make sure that the _Enable HTTP application
routing_ checkbox is enabled. Finally, you can persist the configuration by clicking on the `Save` button.

![](/images/install/aks/04.png)

Azure will take some minutes to deploy the required components and enable the required services.

When the operation is completed you will see a new [DNS zone](https://portal.azure.com/#blade/HubsExtension/BrowseResource/resourceType/Microsoft.Network%2FdnsZones)
in the Azure Portal.

![](/images/install/aks/05.png)

Congratulations! Your AKS cluster is now ready to run Liqo!


### Installation with Helm

#### Set-up values

In order to install Liqo, we need to configure some values of the Helm chart related to the accessibility of the cluster
and its internal configuration.

In particular, we have to set the following values:

| Variable               | Default | Description                                 |
| ---------------------- | ------- | ------------------------------------------- |
| `networkManager.config.podCIDR`             |    10.244.0.0/16     | The cluster Pod CIDR                        |
| `networkManager.config.serviceCIDR`         |    10.0.0.0/16     | The cluster Service CIDR                    |
| `auth.ingress.class`   |   addon-http-application-routing      | The [ingress class](https://kubernetes.io/docs/concepts/services-networking/ingress/#ingress-class) to be used by the Auth Service Ingress |
| `apiServer.address`  |         | The address where to access the API server |
| `apiServer.port`  | 443 | the port where to access the API server     |
| `auth.ingress.host` |         | The hostname where to access the Auth Service, the one exposed with the ingress, if it is not set the service will be exposed with a [NodePort Service](https://kubernetes.io/docs/concepts/services-networking/service/#nodeport) instead of an [Ingress](https://kubernetes.io/docs/concepts/services-networking/ingress/) |
| `auth.ingress.port` | 443  | The port where to access the Auth Service   |

>__NOTE__: if at cluster creation time you changed the default values, make sure to set the right ones.

The `apiServer.address` con be found in our cluster overview as __API server address__

![](/images/install/aks/06.png)

The `auth.ingress.host` is where the Liqo Auth Service will be reachable, so we have to export some hostname that we
can manage. If you are using the AKS HTTP Application Routing, a DNS zone should be available so that you can create a subdomain
on it.

![](/images/install/aks/07.png)

In the above screenshot, a viable hostname for the ingress would be `auth.f83c28d9ce1449b2bb45.westeurope.aksapp.io`.

#### Deploy

You can install Liqo using helm 3.

Firstly, you should add the official Liqo repository to your Helm Configuration:

```bash
helm repo add liqo https://helm.liqo.io/
```

If you are installing Liqo for the first time, you can download the default values.yaml file from the chart.

```bash
helm show values liqo/liqo > ./values.yaml
```

After modifying the `values.yaml` file with the desired values, as described in [the previous section](#setup-values), you can perform the Liqo installation by typing:

```bash
helm install liqo liqo/liqo -f ./values.yaml -n liqo --create-namespace
```

#### Expose the Auth Service with a LoadBalancer Service

To make the Auth Service reachable without the needing of an Ingress and a Domain Name, you can change the `auth-service`
Service type from `NodePort` to `LoadBalancer` by setting the value `.auth.service.type` to `LoadBalancer`.

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
