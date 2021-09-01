---
title: Installation 
weight: 2
---

### Pre-Installation

As presented in the ["Getting Started" section](/gettingstarted/), Liqo can be used with different topologies and scenarios. 
Clusters can be used for incoming or outgoing peering and construct different topologies (e.g. virtual cluster, cloud bursting, etc).

Before installing Liqo, you should:
* Provision the clusters you would like to use with Liqo.
* Have a look to the [pre-install section](./pre-install), that presents some common patterns used to expose and interconnect clusters when using Liqo.

#### liqoctl 

Liqoctl is the swiss-knife CLI tool to install and manager Liqo. 
It is strongly recommended installing Liqo using Liqoctl since it provides customizations for supported providers.

Under the hood, liqoctl uses [Helm 3](https://helm.sh/) to configure and install the Liqo chart available on the official repository. 
If you prefer to customize the installation configuration, you can use liqoctl as a provider-specific values file generator and then install Liqo with Helm as usual.

To install liqoctl, first, you have to set the architecture and OS of your host:

```bash
OS=linux # possible values: linux,windows,darwin
ARCH=amd64 # possible values: amd64,arm64 
```

Then, you should execute the following commands the latest version of liqoctl:
```
LATEST_RELEASE=$(curl -L -s -H 'Accept: application/json' https://github.com/liqotech/liqo/releases/latest)
LATEST_VERSION=$(echo $LATEST_RELEASE | sed -e 's/.*"tag_name":"\([^"]*\)".*/\1/')
curl --fail -LSO "https://github.com/liqotech/liqo/releases/download/${LATEST_VERSION}/liqoctl-${OS}-${ARCH}" && chmod +x liqoctl-${OS}-${ARCH} && sudo mv liqoctl-${OS}-${ARCH} /usr/local/bin/liqoctl
```

Alternatively, you can directly download liqoctl from the [Liqo releases](https://github.com/liqotech/liqo/releases/) page on GitHub.

#### Pre-Requirements

{{% notice note %}}
Liqo only supports Kubernetes >= 1.19.0.
{{% /notice %}}

According to your cluster provider, you may have to perform simple steps before triggering the installation process:

{{< tabs >}}
{{% tab name="Kubernetes IN Docker (KIND)" %}}

##### Configuration

You only have to export the KUBECONFIG environment variable.
Otherwise, liqoctl will use the kubeconfig in kubectl default path (i.e. `${HOME}/.kube/config` )

```bash
kind get kubeconfig --name ${CLUSTER_NAME} > kind_kubeconfig
export KUBECONFIG=kind_kubeconfig
```

{{% /tab %}}

{{% tab name="K8s (Kubeadm)" %}}

##### Supported CNIs

Liqo supports Kubernetes clusters using the following CNIs:

* [Weave](https://github.com/weaveworks/weave)
* [Flannel](https://github.com/coreos/flannel)
* [Canal](https://docs.projectcalico.org/getting-started/kubernetes/flannel/flannel)
* [Calico](https://www.projectcalico.org/)

If you are installing Liqo on a cluster with Calico, you MUST read the [dedicated configuration page](./advanced#calico) to avoid unwanted misconfigurations.

##### Configuration

You only have to export the KUBECONFIG environment variable. 
Otherwise, liqoctl will use the kubeconfig in kubectl default path (i.e. `${HOME}/.kube/config` )

```bash
export KUBECONFIG=/your/kubeconfig/path
```

{{% /tab %}}

{{% tab name="EKS" %}}

##### Supported CNIs

Liqo supports EKS clusters using the default CNI:
* [AWS EKS - amazon-vpc-cni-k8s](https://github.com/aws/amazon-vpc-cni-k8s)

##### Configuration

Liqo leverages AWS credentials to authenticate peered clusters.
Specifically, in addition to the read-only permissions used to configure the cluster installation (i.e., retrieve the appropriate parameters), Liqo uses AWS users to map peering access to EKS clusters.

To install Liqo on EKS, you should log in using the AWS cli (if you already did that, you can skip this step).
This is widely documented on the [official CLI documentation](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-quickstart.html).

In a nutshell, after having installed the CLI, you have to set up your identity:
```bash
aws configure
```

Second, you should export the cluster's kubeconfig if you have not already. You may use the following CLI command:

```bash
aws eks --region ${EKS_CLUSTER_REGION} update-kubeconfig --name ${EKS_CLUSTER_NAME}
```
{{% /tab %}}

{{% tab name="AKS" %}}

##### Supported CNIs

Liqo supports AKS clusters using the following CNIs:
* [Azure AKS - Kubenet](https://docs.microsoft.com/en-us/azure/aks/configure-kubenet)
* [Azure AKS - Azure CNI](https://docs.microsoft.com/en-us/azure/aks/configure-azure-cni)

##### Configuration

First, you should have the AZ cli installed and your AKS cluster deployed. If you haven't, you can follow the [official guide](https://docs.microsoft.com/en-us/cli/azure/install-azure-cli).

Second, you should log-in:
```bash
az login
```

First, let's start exporting required variables:

```bash
export AZURE_RESOURCE_GROUP=myResourceGroup # the resourceGroup where the cluster is created
export AKS_RESOURCE_NAME=myCluster # the name of AKS cluster resource on Azure
export AZURE_SUBSCRIPTION_ID=subscriptionId # the subscription id associated to the AKS cluster's resource group 
```

{{% notice note %}}
You also need read-only permissions on AKS cluster and on the Virtual Networks, if your cluster has an Azure CNI.
{{% /notice %}}

{{% /tab %}}
{{% tab name="GKE" %}}

##### Supported CNIs 

Liqo supports GKE clusters using the default CNI:
* [Google GKE - VPC-Native](https://cloud.google.com/kubernetes-engine/docs/how-to/alias-ips)

{{% notice note %}}
Liqo does not support GKE Autopilot Clusters
{{% /notice %}}

##### Configuration

To install Liqo on GKE, you should at first create a service account for liqoctl, granting the read rights for the GKE clusters (you may reduce the scope to a specific cluster if you prefer).

First, let's start exporting required variables:
```bash
export SERVICE_ACCOUNT_ID=liqoctl-sa #the name of the service account used to interact by liqoctl with GCP
export PROJECT_ID=XYZ # the id of the GCP project where your cluster was created
export SERVICE_ACCOUNT_PATH=~/.liqo/gcp_service_account # the path where the google service account is stored
export GKE_CLUSTER_ZONE=europe-west-1b # the GCP zone where your GKE cluster is executed
```

Second, you should create a GCP Service account. 
This will provide you an identity used by Liqoctl to query all the information needed to properly configure Liqo on your cluster.

The ServiceAccount can be created using:
```bash
gcloud iam service-accounts create ${SERVICE_ACCOUNT_ID} \
    --description="DESCRIPTION" \
    --display-name="DISPLAY_NAME" \
    --project="${PROJECT_ID}"
```

Third, you should provide the ServiceAccount just created with the rights to inspect the cluster and virtual networks parameters:

```bash
gcloud projects add-iam-policy-binding ${PROJECT_ID} \
    --member="serviceAccount:${SERVICE_ACCOUNT_ID}@${PROJECT_ID}.iam.gserviceaccount.com" \
    --role="roles/container.clusterViewer"
gcloud projects add-iam-policy-binding ${PROJECT_ID} \
    --member="serviceAccount:${SERVICE_ACCOUNT_ID}@${PROJECT_ID}.iam.gserviceaccount.com" \
    --role="roles/compute.networkViewer"
```

Fourth, you should create and download valid service accounts keys, as presented [by the official documentation](https://cloud.google.com/iam/docs/creating-managing-service-account-keys#creating_service_account_keys).

The keys will be used by liqoctl to authenticate to GCP as the service account we just created.

```bash
gcloud iam service-accounts keys create ${SERVICE_ACCOUNT_PATH} \
    --iam-account=${SERVICE_ACCOUNT_ID}@${PROJECT_ID}.iam.gserviceaccount.com
```

```bash
export KUBECONFIG=/your/kubeconfig/path
```

{{% /tab %}}
{{% tab name="K3s" %}}

##### Supported CNIs

Liqo supports K3s clusters using the following CNIs:
* [Weave](https://github.com/weaveworks/weave)
* [Flannel](https://github.com/coreos/flannel)
* [Canal](https://docs.projectcalico.org/getting-started/kubernetes/flannel/flannel)
* [Calico](https://www.projectcalico.org/)

##### Configuration

You only have to export the KUBECONFIG environment variable.
Otherwise, liqoctl will use the kubeconfig in kubectl default path (i.e. `${HOME}/.kube/config` )

```bash
export KUBECONFIG=/your/kubeconfig/path
```
{{% /tab %}}
{{< /tabs >}}

### Quick Installation

Now, you can perform the proper Liqo installation on your cluster.
{{< tabs >}}
{{% tab name="Kubernetes IN Docker (KIND)" %}}
```bash
liqoctl install --provider kind
```
{{% /tab %}}

{{% tab name="K8s (Kubeadm)" %}}
```bash
liqoctl install --provider kubeadm
```
{{% /tab %}}
{{% tab name="EKS" %}}

```bash
liqoctl install --provider eks --eks.region=${EKS_CLUSTER_REGION} --eks.cluster-name=${EKS_CLUSTER_NAME} 
```
{{% /tab %}}
{{% tab name="AKS" %}}
```bash
liqoctl install --provider aks --aks.resource-group-name ${AZURE_RESOURCE_GROUP} \ 
         --aks.resource-name ${AZURE_RESOURCE_NAME} \
         --aks.subscription-id ${AZURE_SUBSCRIPTION_ID}"
```
{{% /tab %}}
{{% tab name="GKE" %}}
```bash

liqoctl install --provider gke --gke.project-id=${GKE_PROJECT_ID} \
    --gke.cluster-id=${GKE_CLUSTER_ID} \
    --gke.zone=${GKE_CLUSTER_ZONE} \ 
    --gke.credentials-path=${SERVICE_ACCOUNT_PATH}
```
{{% /tab %}}
{{% tab name="K3s" %}}
```bash
liqoctl install --provider k3s
```
{{% /tab %}}
{{< /tabs >}}

#### Next Steps

After you have successfully installed Liqo, you may:

* [Configure](/configuration): configure the Liqo security, the automatic discovery of new clusters and other system parameters.
* [Use](/usage) Liqo: orchestrate your applications across multiple clusters.
