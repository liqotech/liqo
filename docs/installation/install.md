# Install

The deployment of all the Liqo components is managed through a **Helm chart**.

We strongly recommend **installing Liqo using *liqoctl***, as it automatically handles the required customizations for each supported provider/distribution (e.g., AWS, EKS, GKE, Kubeadm, etc.).
Under the hood, *liqoctl* uses [Helm 3](https://helm.sh/) to configure and install the Liqo Helm chart available on the official repository.

Alternatively, *liqoctl* can also be configured to output a **pre-configured values file**, which can be further customized and used for a manual installation with Helm.

## Install with liqoctl

Below, you can find the basic information to install and configure Liqo, depending on the selected **Kubernetes distribution** and/or **cloud provider**.
By default, *liqoctl install* installs the latest stable version of Liqo, although it can be tuned through the `--version` flag.

The reminder of this page, then, presents **additional customization options** which apply to all setups, as well as advanced options.

```{admonition} Note
*liqoctl* displays a *kubectl* compatible behavior concerning Kubernetes API access, hence supporting the `KUBECONFIG` environment variable, as well as all the standard flags, including `--kubeconfig` and `--context`.
Ensure you selected the correct target cluster before issuing *liqoctl* commands (as you would do with *kubectl*).
```

`````{tab-set}

````{tab-item} Kubeadm

**Supported CNIs**

Liqo supports Kubernetes clusters using the following CNIs: [Flannel](https://github.com/coreos/flannel), [Calico](https://www.projectcalico.org/), [Canal](https://docs.projectcalico.org/getting-started/kubernetes/flannel/flannel), [Weave](https://github.com/weaveworks/weave).
Additionally, partial support is provided for [Cilium](https://cilium.io/), although with the limitations listed below.

```{warning}
If you are installing Liqo on a cluster using the **Calico** CNI, you MUST read the [dedicated configuration section](InstallationCalicoConfiguration) to avoid unwanted misconfigurations.
```

```{admonition} Liqo + Cilium limitations
Currently, Liqo supports the Cilium CNI only when *kube-proxy* is enabled.
Additionally, known limitations concern the impossibility of accessing the backends of *NodePort* and *LoadBalancer* services hosted on remote clusters, from a local cluster using Cilium as CNI.
```

**Installation**

Liqo can be installed on a Kubeadm cluster through:

```bash
liqoctl install kubeadm
```

By default, the cluster is assigned an automatically generated name, then leveraged during the peering and offloading processes.
Alternatively, you can manually specify a desired name with the `--cluster-name` flag.
````

````{tab-item} OpenShift

**Supported versions**

Liqo was tested running on OpenShift Container Platform (OCP) 4.8.

**Installation**

Liqo can be installed on an OpenShift Container Platform (OCP) cluster through:

```bash
liqoctl install openshift
```

By default, the cluster is assigned an automatically generated name, then leveraged during the peering and offloading processes.
Alternatively, you can manually specify a desired name with the `--cluster-name` flag.
````

````{tab-item} AKS

**Supported CNIs**

Liqo supports AKS clusters using the following CNIs: [Azure AKS - Kubenet](https://docs.microsoft.com/en-us/azure/aks/configure-kubenet) and [Azure AKS - Azure CNI](https://docs.microsoft.com/en-us/azure/aks/configure-azure-cni).

**Configuration**

To install Liqo on AKS, you should first log in using the `az` CLI (if not already done):

```bash
az login
```

Before continuing, you should export a few variables about the properties of your cluster:

```bash
# The resource group where the cluster is created
export AKS_RESOURCE_GROUP=resource-group
# The name of AKS cluster resource on Azure
export AKS_RESOURCE_NAME=cluster-name
# The name of the subscription associated with the AKS cluster
export AKS_SUBSCRIPTION_ID=subscription-name
```

```{admonition} Note
During the installation process, you need read-only permissions on the AKS cluster and on the Virtual Networks, if your cluster leverages the Azure CNI.
```

**Installation**

Liqo can be installed on an AKS cluster through:

```bash
liqoctl install aks --resource-group-name "${AKS_RESOURCE_GROUP}" \
        --resource-name "${AKS_RESOURCE_NAME}" \
        --subscription-name "${AKS_SUBSCRIPTION_ID}"
```

By default, the cluster is assigned the same name as that specified through the `--resource-name` parameter.
Alternatively, you can manually specify a different name with the `--cluster-name` *liqoctl* flag.

```{admonition} Note
If you are running an [AKS private cluster](https://docs.microsoft.com/en-us/azure/aks/private-clusters), you may need to set the `--disable-endpoint-check` *liqoctl* flag, since the API Server in your kubeconfig may be different from the one retrieved from the Azure APIs.

Additionally, since your API Server is not accessible from the public Internet, you shall leverage the [in-band peering approach](FeaturesPeeringInBandControlPlane) towards the clusters not attached to the same Azure Virtual Network.
```
````

````{tab-item} EKS

**Supported CNIs**

Liqo supports EKS clusters using the default CNI: [AWS EKS - amazon-vpc-cni-k8s](https://github.com/aws/amazon-vpc-cni-k8s).

**Configuration**

Liqo leverages **AWS credentials to authenticate peered clusters**.
Specifically, in addition to the read-only permissions used to configure the cluster installation (i.e., retrieve the appropriate parameters), Liqo uses AWS users to map peering access to EKS clusters.

To install Liqo on EKS, you should first log in using the `aws` CLI (if not already done).
This is widely documented on the [official CLI documentation](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-quickstart.html).
In a nutshell, after having installed the CLI, you have to set up your identity:

```bash
aws configure
```

Before continuing, you should export a few variables about the properties of your cluster:

```bash
# The name of the target cluster
export EKS_CLUSTER_NAME=cluster-name
# The AWS region where the cluster is deployed
export EKS_CLUSTER_REGION=cluster-region
```

Then, you should retrieve the cluster's kubeconfig, if you have not already.
You may use the following CLI command:

```bash
aws eks --region ${EKS_CLUSTER_REGION} update-kubeconfig --name ${EKS_CLUSTER_NAME}
```

**Installation**

Liqo can be installed on an EKS cluster through:

```bash
liqoctl install eks --eks-cluster-region=${EKS_CLUSTER_REGION} \
        --eks-cluster-name=${EKS_CLUSTER_NAME}
```

By default, the cluster is assigned the same name as that specified through the `--eks-cluster-name` parameter.
Alternatively, you can manually specify a different name with the `--cluster-name` *liqoctl* flag.
````

````{tab-item} GKE

**Supported CNIs**

Liqo supports GKE clusters using the default CNI: [Google GKE - VPC-Native](https://cloud.google.com/kubernetes-engine/docs/how-to/alias-ips).

```{warning}
Liqo does not support GKE Autopilot Clusters.
```

**Configuration**

To install Liqo on GKE, you should create a service account for *liqoctl*, granting the read rights for the GKE clusters (you may reduce the scope to a specific cluster if you prefer).

First, let's start exporting a few variables about the properties of your cluster and the service account to create:
```bash
# The name of the service account used by liqoctl to interact with GCP
export GKE_SERVICE_ACCOUNT_ID=liqoctl
# The path where the GCP service account is stored
export GKE_SERVICE_ACCOUNT_PATH=~/.liqo/gcp_service_account

# The ID of the GCP project where your cluster was created
export GKE_PROJECT_ID=project-id
# The GCP zone where your GKE cluster is executed (if you are using zonal GKE clusters)
export GKE_CLUSTER_ZONE=europe-west1-b
# The GCP region where your GKE cluster is executed (if you are using regional GKE clusters)
export GKE_CLUSTER_REGION=europe-west1
# The name of the GKE resource on GCP
export GKE_CLUSTER_ID=liqo-cluster
```

Second, you should create a GCP service account.
This will represent the identity used by *liqoctl* to query the information required to properly configure Liqo on your cluster.
The service account can be created using:

```bash
gcloud iam service-accounts create ${GKE_SERVICE_ACCOUNT_ID} \
    --project="${GKE_PROJECT_ID}" \
    --description="The identity used by liqoctl during the installation process" \
    --display-name="liqoctl"
```

Third, you should grant the service account the rights to inspect the cluster and the virtual networks parameters:

```bash
gcloud projects add-iam-policy-binding ${GKE_PROJECT_ID} \
    --member="serviceAccount:${GKE_SERVICE_ACCOUNT_ID}@${GKE_PROJECT_ID}.iam.gserviceaccount.com" \
    --role="roles/container.clusterViewer"
gcloud projects add-iam-policy-binding ${GKE_PROJECT_ID} \
    --member="serviceAccount:${GKE_SERVICE_ACCOUNT_ID}@${GKE_PROJECT_ID}.iam.gserviceaccount.com" \
    --role="roles/compute.networkViewer"
```

Fourth, you should create and download a set of valid service accounts keys, as presented by the [official documentation](https://cloud.google.com/iam/docs/creating-managing-service-account-keys#creating_service_account_keys).
The keys will be used by liqoctl to authenticate to GCP:

```bash
gcloud iam service-accounts keys create ${GKE_SERVICE_ACCOUNT_PATH} \
    --iam-account=${GKE_SERVICE_ACCOUNT_ID}@${GKE_PROJECT_ID}.iam.gserviceaccount.com
```

Finally, you should retrieve the cluster’s kubeconfig, if you have not already.
You may use the following CLI command, in case of zonal GKE clusters:

```bash
gcloud container clusters get-credentials ${GKE_CLUSTER_ID} \
        --zone ${GKE_CLUSTER_ZONE} --project ${GKE_PROJECT_ID}
```

or, in case of regional GKE clusters:

```bash
gcloud container clusters get-credentials ${GKE_CLUSTER_ID} \
        --region ${GKE_CLUSTER_REGION} --project ${GKE_PROJECT_ID}
```

The retrieved kubeconfig will be added to the currently selected file (i.e., based on the `KUBECONFIG` environment variable, with fallback to the default path `~/.kube/config`) or created otherwise.

**Installation**

Liqo can be installed on a zonal GKE cluster through:

```bash
liqoctl install gke --project-id ${GKE_PROJECT_ID} \
    --cluster-id ${GKE_CLUSTER_ID} \
    --zone ${GKE_CLUSTER_ZONE} \
    --credentials-path ${GKE_SERVICE_ACCOUNT_PATH}
```

or, in case of regional GKE clusters:

```bash
liqoctl install gke --project-id ${GKE_PROJECT_ID} \
    --cluster-id ${GKE_CLUSTER_ID} \
    --region ${GKE_CLUSTER_REGION} \
    --credentials-path ${GKE_SERVICE_ACCOUNT_PATH}
```

By default, the cluster is assigned the same name as that assigned in GCP.
Alternatively, you can manually specify a different name with the `--cluster-name` *liqoctl* flag.
````

````{tab-item} K3s

```{admonition} Note
By default, the K3s installer stores the kubeconfig to access your cluster in the non-standard path `/etc/rancher/k3s/k3s.yaml`.
Make sure to properly refer to it when using *liqoctl* (e.g., setting the `KUBECONFIG` variable), and that the current user has permissions to read it.
```

**Installation**

Liqo can be installed on a K3s cluster through:

```bash
liqoctl install k3s
```

You may additionally set the `--api-server-url` flag to override the Kubernetes API Server address used by remote clusters to contact the local one.
This operation is necessary in case the default address (`https://<control-plane-node-ip>:6443`) is unsuitable (e.g., the node IP is externally remapped).

By default, the cluster is assigned an automatically generated name, then leveraged during the peering and offloading processes.
Alternatively, you can manually specify a desired name with the `--cluster-name` flag.
````

````{tab-item} KinD

**Installation**

Liqo can be installed on a KinD cluster through:

```bash
liqoctl install kind
```

By default, the cluster is assigned an automatically generated name, then leveraged during the peering and offloading processes.
Alternatively, you can manually specify a desired name with the `--cluster-name` flag.
````

````{tab-item} Other

**Configuration**

To install Liqo on alternative Kubernetes distributions, it is necessary to manually retrieve three main configuration parameters:

* **API Server URL**: the Kubernetes API Server URL (defaults to the one specified in the kubeconfig).
* **Pod CIDR**: the range of IP addresses used by the cluster for the pod network.
* **Service CIDR**: the range of IP addresses used by the cluster for service VIPs.

**Installation**

Once retrieved the above parameters, Liqo can be installed on a generic cluster through:

```bash
liqoctl install --api-server-url=<API-SERVER-URL> \
      --pod-cidr=<POD-CIDR> --service-cidr=<SERVICE-CIDR>
```

By default, the cluster is assigned an automatically generated name, then leveraged during the peering and offloading processes.
Alternatively, you can manually specify a desired name with the `--cluster-name` flag.
````
`````

(InstallCustomization)=

## Customization options

The following lists the main **customization parameters** exposed by the *liqoctl install* commands, along with a brief description.
Additionally, **arbitrary parameters** available in the Helm *values* file (the full list is provided in the dedicated [repository page](https://github.com/liqotech/liqo/tree/master/deployments/liqo)) can be modified through the `--set` flag, which supports the standard Helm syntax.

### Global

The main global flags, besides those concerning the installation of [development versions](InstallationDevelopmentVersions), include:

* `--enable-ha`: whether to enable the support for **high-availability of the Liqo components**, starting two replicas (in an active/standby configuration) of the **gateway** to ensure no cross-cluster connectivity downtime in case one of the replicas is restarted, as well as of the **controller manager**, which embeds the Liqo control plane logic.
* `--enable-metrics`: enable **metrics** exposition through **prometheus**.
* `--timeout`: configures the timeout for the completion of the installation/upgrade process.
  Once expired, the process is aborted and Liqo is rolled back to the previous version.
* `--verbose`: whether to enable verbose logs, providing additional information concerning the installation/upgrade process (i.e., for troubleshooting purposes).

### Control plane

The main control plane flags include:

* `--cluster-name`: configures a **name identifying the cluster** in Liqo.
This name is propagated to remote clusters during the peering process, and used to identify the corresponding virtual nodes and the technical resources leveraged for the negotiation process. Additionally, it is leveraged as part of the suffix to ensure namespace names uniqueness during the offloading process. In case a cluster name is not specified, it is defaulted to that of the cluster in the cloud provider, if any, or it is automatically generated.
* `--cluster-labels`: a set of **labels** (i.e., key/value pairs) **identifying the cluster in Liqo** (e.g., geographical region, Kubernetes distribution, cloud provider, ...) and automatically propagated during the peering process to the corresponding virtual nodes.
These labels can be used later to **restrict workload offloading to a subset of clusters**, as detailed in the [namespace offloading usage section](/usage/namespace-offloading).
* `--sharing-percentage`: the maximum percentage of available **cluster resources** that could be shared with remote clusters. This is the Liqo's default behavior but you can change it by using a custom [resource plugin](https://github.com/liqotech/liqo-resource-plugins).

### Networking

The main networking flags include:

* `--reserved-subnets`: the list of **private CIDRs to be excluded** from the ones used by Liqo to remap remote clusters in case of address conflicts, as already in use (e.g., the subnet of the cluster nodes).
The Pod CIDR and the Service CIDR shall not be manually specified, as automatically included in the reserved list.

(InstallationHelm)=

## Install with Helm

To install Liqo directly with Helm, it is possible to proceed as follows:

1. Add the Liqo Helm repository:

   ```bash
   helm repo add liqo https://helm.liqo.io/
   ```

2. Update the local Helm repository cache:

   ```bash
   helm repo update
   ```

3. Generate a pre-configured values file with *liqoctl*:

   ```bash
   liqoctl install <provider> [flags] --only-output-values
   ```

   The resulting *values* file is saved in the current directory, as `values.yaml`, or in the path specified through the `--dump-values-path` flag.

   ````{admonition} Note
   This step is optional, but it relieves the user from the retrieval of the set of necessary parameters depending on the target provider/distribution.
   Alternatively, the upstream values file can be retrieved through:

   ```bash
   helm show values liqo/liqo > values.yaml
   ```
   ````

4. Appropriately configure the *values* file.
   The full list of options is provided in the dedicated [repository page](https://github.com/liqotech/liqo/tree/master/deployments/liqo).

5. Install Liqo:

   ```bash
   helm install liqo liqo/liqo --namespace liqo \
          --values <path-to-values-file> --create-namespace
   ```

(InstallationDevelopmentVersions)=

## Install development versions

In addition to released versions (including alpha and beta candidates), *liqoctl* provides the possibility to install **development versions** of Liqo.
Development versions include:

* All commits merged into the master branch of Liqo.
* The commits of *pull requests* to the Liqo repository, whose images have been built through the appropriate bot command.

The installation of a development version of Liqo can be triggered specifying a **commit *SHA*** through the `--version` flag.
In this case, *liqoctl* proceeds to **clone the repository** (either from the official repository, or from a fork configured through the `--repo-url` flag) at the given revision, and to leverage the Helm chart therein contained:

```bash
liqoctl install <provider> --version <commit-sha> --repo-url <forked-repo-url>
```

Alternatively, the Helm chart can be retrieved from a **local path**, as configured through the `--local-chart-path` flag:

```bash
liqoctl install <provider> --version <commit-sha> --local-chart-path <path-to-local-chart>
```

(InstallationCalicoConfiguration)=

## Check installation

After the installation, it is possible to check the status of the Liqo components.
In particular, the following command can be used to check the status of the Liqo **pods** and get **local information**:

```bash
liqoctl status
```

## Liqo and Calico

Liqo adds several interfaces to the cluster nodes to handle cross-cluster traffic routing.
Those interfaces are intended to not interfere with the normal CNI job.

However, by default, Calico scans all existing interfaces on a node to detect network configurations and establish the correct routes.
To prevent misconfigurations, Calico shall then be configured to skip Liqo-managed interfaces during this process.
This is required if Calico is configured in *BGP* mode, while not in case the *VPC native setup* is leveraged.

In Calico v3.17 and above, this can be performed by patching the *Installation CR*, adding the following:

```yaml
apiVersion: operator.tigera.io/v1
kind: Installation
metadata:
  name: default
spec:
  calicoNetwork:
    nodeAddressAutodetectionV4:
      skipInterface: liqo.*
    ...
  ...
```

For Calico versions prior to 3.17, instead, you should modify the `calico-node` *DaemonSet*, adding the appropriate environment variable to the `calico-node` container.

```yaml
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: calico-node
  namespace: kube-system
spec:
  template:
    spec:
      containers:
      - name: calico-node
        env:
        - name: IP_AUTODETECTION_METHOD
          value: skip-interface=liqo.*
      ...
```
