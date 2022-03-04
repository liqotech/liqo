---
title: Installation
weight: 2
---

{{% install-url-query-parser %}}

### Pre-install

Before installing Liqo, you should:

* Provision the clusters you would like to use with Liqo.
* Have a look to the [Connectivity requirements](/installation/connect-requirements) section, which help understand how to set some configuration options in Liqo in order to overcome possible network limitations, and in general to allow arbitrary clusters to peer to each other.

#### Connectivity requirements

The [Connectivity requirements](/installation/connect-requirements) section presents the requirement of Liqo in terms of network connectivity. In particular:

* How to configure some Liqo services (i.e., `liqo-auth`, `liqo-gateway`) to be reachable from outside the cluster.
* How to configure some Liqo with the URL of additional services (i.e., API server) that need to be reachable from outside the cluster.
* How to determine IP addresses and TCP/UDP ports used by the above services, which can be used to configure a firewall that protects the access to the cluster.
* How to configure a Liqo cluster under NAT, and how to configure a NAT in order to support Liqo.

#### liqoctl

Liqoctl is the swiss-knife CLI tool to install and manage Liqo clusters.
We strongly recommend installing Liqo using liqoctl because it automatically handles the required customizations for each supported providers (e.g., AWS, EKS, etc.).

Under the hood, liqoctl uses [Helm 3](https://helm.sh/) to configure and install the Liqo chart available on the official repository.
If you prefer to customize the installation configuration, you can use liqoctl as a provider-specific values file generator and then install Liqo with Helm as usual.

To install liqoctl, first, you have to set the architecture and OS of your host:

```bash
OS=linux # possible values: linux,windows,darwin
ARCH=amd64 # possible values: amd64,arm64
```

Then, you can install the latest version of liqoctl:

```bash
curl --fail -LSO "https://get.liqo.io/liqoctl-${OS}-${ARCH}" && \
chmod +x "liqoctl-${OS}-${ARCH}" && \
sudo mv "liqoctl-${OS}-${ARCH}" /usr/local/bin/liqoctl
```

Alternatively, you can directly download liqoctl from the [Liqo releases](https://github.com/liqotech/liqo/releases/) page on GitHub.
For more information and options about liqoctl, you can check out the [advanced installation section](/installation/install-advanced).

##### Command Completion (Optional)

{{< tabs groupId="shell" >}}
{{% tab name="Bash" %}}
To load completions in the current session, execute once:
```bash
source <(liqoctl completion bash)
```

To load completions for each session, execute once the following command:

```bash
source <(liqoctl completion bash) >> ~/.bashrc
```
{{% /tab %}}

{{% tab name="ZSH" %}}

If ZSH completion is not already enabled, you have first to execute the following once:
```zsh
echo "autoload -U compinit; compinit" >> ~/.zshrc
```

To load completions for each session, execute once:
```zsh
liqoctl completion zsh > "${fpath[1]}/_liqoctl"
source ~/.zshrc
```

{{% /tab %}}

{{% tab name="Fish" %}}
To load completions for the current session, execute once:
```bash
liqoctl completion fish | source
```

To load completions for each session, execute once:
```bash
liqoctl completion fish > ~/.config/fish/completions/liqoctl.fish
```
{{% /tab %}}

{{% tab name="Powershell" %}}
To load completions for the current session, execute once:
```bash
liqoctl completion powershell | Out-String | Invoke-Expression
```

To load completions for each session, execute once:
```bash
liqoctl completion powershell > liqoctl.ps1
```

and source this file from your PowerShell profile.
{{% /tab %}}


{{< /tabs >}}

#### Pre-Requirements

{{% notice note %}}
Liqo only supports Kubernetes >= 1.19.0.
{{% /notice %}}

According to your cluster provider, you may have to perform simple steps before triggering the installation process:

{{< tabs groupId="provider" >}}
{{% tab name="Kubernetes IN Docker (KIND)" %}}

##### Configuration

You only have to export the KUBECONFIG environment variable.
Otherwise, liqoctl will use the kubeconfig in kubectl default path (i.e. `${HOME}/.kube/config`)

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

If you are installing Liqo on a cluster with Calico, you MUST read the [dedicated configuration page](/installation/calico-configuration) to avoid unwanted misconfigurations.

##### Configuration

You only have to export the KUBECONFIG environment variable.
Otherwise, liqoctl will use the kubeconfig in kubectl default path (i.e. `${HOME}/.kube/config`)

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

To install Liqo on EKS, you should log in using the AWS CLI (if you already did that, you can skip this step).
This is widely documented on the [official CLI documentation](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-quickstart.html).

In a nutshell, after having installed the CLI, you have to set up your identity:
```bash
aws configure
```

Before continuing, you should first export few variables about your cluster:

```bash
export EKS_CLUSTER_NAME=liqo-cluster # the name of the target cluster
export EKS_CLUSTER_REGION=my-cluster # the AWS region where the cluster is deployed
```

Second, you should export the cluster's KUBECONFIG if you have not already. You may use the following CLI command:

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

First, you should have the AZ CLI installed and your AKS cluster deployed. If you haven't, you can follow the [official guide](https://docs.microsoft.com/en-us/cli/azure/install-azure-cli).

Second, you should log in:
```bash
az login
```

First, let's start exporting required variables:

```bash
export AKS_RESOURCE_GROUP=myResourceGroup # the resourceGroup where the cluster is created
export AKS_RESOURCE_NAME=myCluster # the name of AKS cluster resource on Azure
export AKS_SUBSCRIPTION_ID=subscriptionId # the subscription id associated to the AKS cluster's resource group
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
export GKE_SERVICE_ACCOUNT_ID=liqoctl-sa #the name of the service account used to interact by liqoctl with GCP
export GKE_PROJECT_ID=XYZ # the id of the GCP project where your cluster was created
export GKE_SERVICE_ACCOUNT_PATH=~/.liqo/gcp_service_account # the path where the google service account is stored
export GKE_CLUSTER_ZONE=europe-west-1b # the GCP zone where your GKE cluster is executed
export GKE_CLUSTER_ID=liqo-cluster # the name of the GKE resource on GCP
```

Second, you should create a GCP Service account.
This will provide you an identity used by Liqoctl to query all the information needed to properly configure Liqo on your cluster.

The ServiceAccount can be created using:
```bash
gcloud iam service-accounts create ${GKE_SERVICE_ACCOUNT_ID} \
    --description="DESCRIPTION" \
    --display-name="DISPLAY_NAME" \
    --project="${GKE_PROJECT_ID}"
```

Third, you should provide the ServiceAccount just created with the rights to inspect the cluster and virtual networks parameters:

```bash
gcloud projects add-iam-policy-binding ${GKE_PROJECT_ID} \
    --member="serviceAccount:${GKE_SERVICE_ACCOUNT_ID}@${GKE_PROJECT_ID}.iam.gserviceaccount.com" \
    --role="roles/container.clusterViewer"
gcloud projects add-iam-policy-binding ${GKE_PROJECT_ID} \
    --member="serviceAccount:${GKE_SERVICE_ACCOUNT_ID}@${GKE_PROJECT_ID}.iam.gserviceaccount.com" \
    --role="roles/compute.networkViewer"
```

Fourth, you should create and download valid service accounts keys, as presented [by the official documentation](https://cloud.google.com/iam/docs/creating-managing-service-account-keys#creating_service_account_keys).

The keys will be used by liqoctl to authenticate to GCP as the service account we just created.

```bash
gcloud iam service-accounts keys create ${GKE_SERVICE_ACCOUNT_PATH} \
    --iam-account=${GKE_SERVICE_ACCOUNT_ID}@${GKE_PROJECT_ID}.iam.gserviceaccount.com
```

Now, you can obtain the cluster kubeconfig with the following command:
```bash
gcloud container clusters get-credentials ${GKE_CLUSTER_ID} --zone ${GKE_CLUSTER_ZONE} --project ${GKE_PROJECT_ID}
```
The kubeconfig will be added to the current selected file (KUBECONFIG environment variable or the default path `~/.kube/config`) or created otherwise.

You are ready to start the installation.
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
Otherwise, liqoctl will use the kubeconfig in kubectl default path (i.e. `${HOME}/.kube/config`)

```bash
export KUBECONFIG=/your/kubeconfig/path
```
{{% /tab %}}
{{% tab name="OpenShift Container Platform (OCP)" %}}

Liqo was tested running on OpenShift Container Platform (OCP) 4.8.

##### Configuration

You only have to export the KUBECONFIG environment variable.
Otherwise, liqoctl will use the kubeconfig in kubectl default path (i.e. `${HOME}/.kube/config`)

```bash
export KUBECONFIG=/your/kubeconfig/path
```

{{% /tab %}}
{{< /tabs >}}

### Quick Installation

Now, you can perform the proper Liqo installation on your cluster.
{{< tabs groupId="provider" >}}
{{% tab name="Kubernetes IN Docker (KIND)" %}}
```bash
liqoctl install kind
```
{{% /tab %}}

{{% tab name="K8s (Kubeadm)" %}}
```bash
liqoctl install kubeadm
```
{{% /tab %}}
{{% tab name="EKS" %}}

```bash
liqoctl install eks --region=${EKS_CLUSTER_REGION} --eks-cluster-name=${EKS_CLUSTER_NAME}
```

{{% /tab %}}
{{% tab name="AKS" %}}
```bash
liqoctl install aks --resource-group-name "${AKS_RESOURCE_GROUP}" \
         --resource-name "${AKS_RESOURCE_NAME}" \
         --subscription-id "${AKS_SUBSCRIPTION_ID}"
```
{{% /tab %}}
{{% tab name="GKE" %}}
```bash

liqoctl install gke --project-id ${GKE_PROJECT_ID} \
    --cluster-id ${GKE_CLUSTER_ID} \
    --zone ${GKE_CLUSTER_ZONE} \
    --credentials-path ${GKE_SERVICE_ACCOUNT_PATH}
```
{{% /tab %}}
{{% tab name="K3s" %}}
```bash
liqoctl install k3s
```
{{% /tab %}}
{{% tab name="OpenShift Container Platform (OCP)" %}}
```bash
liqoctl install openshift
```
{{% /tab %}}
{{< /tabs >}}

#### Next Steps

After you have successfully installed Liqo, you may:

* [Configure](/configuration): configure the Liqo security, the automatic discovery of new clusters and other system parameters.
* [Use](/usage) Liqo: orchestrate your applications across multiple clusters.
