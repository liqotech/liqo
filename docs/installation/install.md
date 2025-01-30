# Install

Liqo can be easily installed with *liqoctl*, which automatically handles all the customized settings required to set up the software on the multiple providers/distributions supported (e.g., AWS, EKS, GKE, Kubeadm, etc.).
Under the hood, *liqoctl* uses [Helm 3](https://helm.sh/) to configure and install all the Liqo components, using the Helm chart available in the official repository.

Alternatively, you can install Liqo manually with Helm.
However, we suggest using *liqoctl* also in this case.
In fact, *liqoctl* can also generate a local file with **pre-configured values**, which can be further customized and used for your manual installation.

You can refer to the previous section about [downloading and installing *liqoctl*](liqoctl.md).

## Install with liqoctl

Below, you can find the basic information to install and configure Liqo, depending on the selected **Kubernetes distribution** and/or **cloud provider**.

By default, the `liqoctl install` command installs the same Liqo version of the *liqoctl* client, although it can be overridden with the `--version` flag.

The rest of this page presents **additional customization options** that apply to all setups, as well as advanced options that are cloud/distribution-specific.

```{admonition} Note
*liqoctl* implements a *kubectl* compatible behavior with respect to Kubernetes API access, hence supporting the `KUBECONFIG` environment variable, as well as all the standard flags, including `--kubeconfig` and `--context`.
Hence, make sure you selected the correct target cluster before issuing *liqoctl* commands (as you would do with *kubectl*).
```

`````{tab-set}

````{tab-item} Kubeadm

**Supported CNIs**

Liqo supports Kubernetes clusters using the following CNIs: [Cilium](https://cilium.io/), [Flannel](https://github.com/flannel-io/flannel), [Calico](https://www.tigera.io/project-calico/).

```{warning}
If you are installing Liqo on a cluster using the **Calico** or **Cilium** CNI, you MUST read the [dedicated configuration section](InstallationCNIConfiguration) to avoid unwanted misconfigurations.
```

**Installation**

Liqo can be installed on a Kubeadm cluster with the following command:

```bash
liqoctl install kubeadm
```

The id of the cluster is automatically generated, then used during the peering and offloading processes.
Alternatively, you can manually specify a desired id with the `--cluster-id` flag.

````

````{tab-item} AKS

```{warning}
Liqo does NOT support:

* Cross-cluster API server interaction
* NodePort exposition **only on Azure CNI (Legacy)**
* External IP remapping **only on Azure CNI Overlay and Kubenet**
```

**Supported CNIs**

Liqo supports AKS clusters using the following CNIs: [Azure AKS - Kubenet](https://learn.microsoft.com/en-us/azure/aks/configure-kubenet), [Azure AKS - Azure CNI Overlay](https://learn.microsoft.com/en-us/azure/aks/azure-cni-overlay?tabs=kubectl) and [Azure AKS - Azure CNI (Legacy)](https://learn.microsoft.com/en-us/azure/aks/configure-azure-cni).

**Configuration**

To install Liqo on AKS, you should first log in using the `az` CLI (if not already done):

```bash
az login
```

Before continuing, you should export the following variables with some information about your cluster:

```bash
# The resource group where the cluster is created
export AKS_RESOURCE_GROUP=resource-group
# The name of AKS cluster resource on Azure
export AKS_RESOURCE_NAME=cluster-name
# The id of the subscription associated with the AKS cluster
export AKS_SUBSCRIPTION_ID=subscription-id
```

```{admonition} Note
During the installation process, you need read-only permissions on the AKS cluster and on the Virtual Networks, if your cluster leverages the Azure CNI.
```

**Installation**

Liqo can be installed on an AKS cluster with the following command:

```bash
liqoctl install aks --resource-group-name "${AKS_RESOURCE_GROUP}" \
        --resource-name "${AKS_RESOURCE_NAME}" \
        --subscription-id "${AKS_SUBSCRIPTION_ID}"
```

The id of the cluster will be equal to the one specified in the `--resource-name` parameter.
Alternatively, you can manually set a different name with the `--cluster-id` *liqoctl* flag.

```{admonition} Note
If you are running an [AKS private cluster](https://learn.microsoft.com/en-us/azure/aks/private-clusters), you may need to set the `--disable-api-server-sanity-check` *liqoctl* flag, since the API Server in your kubeconfig may be different from the one retrieved from the Azure APIs.

If the private cluster uses private link, you can set the `--private-link` *liqoctl* flag to use the private FQDN for the API server.
```

```{admonition} Virtual Network Resource Group
By default, it is assumed the Virtual Network Resource for the AKS Subnet is located in the same Resource Group
as the AKS Resource. If that is not the case, you will need to use the `--vnet-resource-group-name` flag to provide the
correct Resource Group name where the Virtual Network Resource is located.
```
````

````{tab-item} EKS

```{warning}
Liqo does NOT support:

* Cross-cluster API server interaction
* External IP remapping
```

```{admonition} Note
If you are planning to use an EKS cluster as [network server](/advanced/peering/inter-cluster-network), you need to install the [AWS Load Balancer V2 Controller](https://kubernetes-sigs.github.io/aws-load-balancer-controller/v2.8/) on the EKS cluster.
```

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

You can install Liqo even if you are not an EKS administrator.
The minimum **IAM** permissions required by a user to install Liqo are the following:

```json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": [
                "eks:DescribeCluster",
                "iam:CreateUser",
                "iam:CreateAccessKey",
                "ec2:DescribeVpcs"
            ],
            "Resource": "*"
        },
        {
            "Effect": "Allow",
            "Action": [
                "iam:CreatePolicy",
                "iam:GetPolicyVersion",
                "iam:GetPolicy",
                "iam:AttachUserPolicy",
                "iam:GetUser",
                "iam:TagUser",
                "iam:ListAccessKeys"
            ],
            "Resource": [
                "arn:aws:iam::*:user/liqo-*",
                "arn:aws:iam::*:policy/liqo-*"
            ]
        }
    ]
}
```

Before continuing, you should export the following variables with some information about your cluster:

```bash
# The name of the target cluster
export EKS_CLUSTER_NAME=cluster-name
# The AWS region where the cluster is deployed
export EKS_CLUSTER_REGION=cluster-region
```

Then, you should retrieve the cluster's kubeconfig (if you have not done it already) with the following CLI command:

```bash
aws eks --region ${EKS_CLUSTER_REGION} update-kubeconfig --name ${EKS_CLUSTER_NAME}
```

**Installation**

Liqo can be installed on an EKS cluster with the following command:

```bash
liqoctl install eks --eks-cluster-region=${EKS_CLUSTER_REGION} \
        --eks-cluster-name=${EKS_CLUSTER_NAME}
```

The id of the cluster will be equal to the one specified in the `--eks-cluster-name` parameter.
Alternatively, you can manually set a different id with the `--cluster-id` *liqoctl* flag.

````

````{tab-item} GKE

```{warning}
Liqo does NOT support:

* GKE Autopilot Clusters
* Intranode visibility: make sure this option is disabled or use the `--no-enable-intra-node-visibility` flag
* NodePort exposition **only on Dataplane V2**
* LoadBalancer exposition **only on Dataplane V2**
```

**Supported CNIs**

Liqo supports GKE clusters using [Dataplane V1](https://cloud.google.com/kubernetes-engine/docs/concepts/network-overview) and [Dataplane V2](https://cloud.google.com/kubernetes-engine/docs/concepts/dataplane-v2).

**Configuration**

To install Liqo on GKE, you should create a service account for *liqoctl*, granting the read rights for the GKE clusters (you may reduce the scope to a specific cluster if you prefer).

First, you should export the following variables with some information about your cluster and the service account to create:
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

Fourth, you should create and download a set of valid service accounts keys, as presented by the [official documentation](https://cloud.google.com/iam/docs/keys-create-delete?hl=it#creating).
The keys will be used by liqoctl to authenticate to GCP:

```bash
gcloud iam service-accounts keys create ${GKE_SERVICE_ACCOUNT_PATH} \
    --iam-account=${GKE_SERVICE_ACCOUNT_ID}@${GKE_PROJECT_ID}.iam.gserviceaccount.com
```

Finally, you should retrieve the cluster’s kubeconfig (if you have not done it already) with the following CLI command in case of **zonal** GKE clusters:

```bash
gcloud container clusters get-credentials ${GKE_CLUSTER_ID} \
        --zone ${GKE_CLUSTER_ZONE} --project ${GKE_PROJECT_ID}
```

or, in case of **regional** GKE clusters:

```bash
gcloud container clusters get-credentials ${GKE_CLUSTER_ID} \
        --region ${GKE_CLUSTER_REGION} --project ${GKE_PROJECT_ID}
```

The retrieved kubeconfig will be added to the currently selected file (i.e., based on the `KUBECONFIG` environment variable, with fallback to the default path `~/.kube/config`) or created otherwise.

**Installation**

Liqo can be installed on a zonal GKE cluster with the following command:

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

The id of the cluster will be equal to the one defined in GCP.
Alternatively, you can manually set a different id with the `--cluster-id` *liqoctl* flag.
````

````{tab-item} K3s

```{warning}
- Due to an issue with K3s certificates, the `kubectl exec' command doesn't work properly when used on a pod scheduled on a virtual node.
- Due to an issue with the [nftables golang library](https://github.com/google/nftables) and the pod running in *host network* in K3s, the firewall monitoring feature is disabled by default.
This means that the firewall rules on the node will not be monitored and enforced by Liqo. If these rules are deleted or changed, Liqo won't restore them.
```

```{admonition} Note
By default, the K3s installer stores the kubeconfig to access your cluster in the non-standard path `/etc/rancher/k3s/k3s.yaml`.
Make sure to properly refer to it when using *liqoctl* (e.g., setting the `KUBECONFIG` variable), and that the current user has permissions to read it.
```

**Installation**

Liqo can be installed on a K3s cluster with the following command:

```bash
liqoctl install k3s
```

You may additionally set the `--api-server-url` flag to override the Kubernetes API Server address used by remote clusters to contact the local one.
This operation is necessary in case the default address (`https://<control-plane-node-ip>:6443`) is unsuitable (e.g., the node IP is externally remapped).

The id of the cluster is automatically generated, then used during the peering and offloading processes.
Alternatively, you can manually specify a desired id with the `--cluster-id` flag.

````

````{tab-item} KinD

**Installation**

Liqo can be installed on a KinD cluster with the following command:

```bash
liqoctl install kind
```

The id of the cluster is automatically generated, then used during the peering and offloading processes.
Alternatively, you can manually specify a desired id with the `--cluster-id` flag.

````

````{tab-item} Other

**Configuration**

To install Liqo on alternative Kubernetes distributions, you should manually retrieve three main configuration parameters:

* **API Server URL**: the Kubernetes API Server URL (defaults to the one specified in the kubeconfig).
* **Pod CIDR**: the range of IP addresses used by the cluster for the pod network.
* **Service CIDR**: the range of IP addresses used by the cluster for service VIPs.

**Installation**

Once retrieved the above parameters, Liqo can be installed on a generic cluster with the following command:

```bash
liqoctl install --api-server-url=<API-SERVER-URL> \
      --pod-cidr=<POD-CIDR> --service-cidr=<SERVICE-CIDR>
```

The id of the cluster is automatically generated, then used during the peering and offloading processes.
Alternatively, you can manually specify a desired id with the `--cluster-id` flag.

````
`````

(InstallCustomization)=

## Customization options

This section lists the main **customization parameters** supported by the *liqoctl install* command, along with a brief description.

Before listing all the parameters, we start here with some general considerations:

* **Getting help**: You can type `liqoctl install --help` to get the list of available options.
* **Changing arbitrary parameters**: all the parameters defined in the Helm *values* file (the full list is provided on the dedicated [repository page](https://github.com/liqotech/liqo/tree/master/deployments/liqo)) can be modified either at install time or, if supported by the parameter itself, also at run-time using one of the following methods:
  * **`liqoctl install --values [file]`**: it accepts as input a file containing all the parameters that you want to set.
  * **`liqoctl install --set [param=value]`**: it changes a single parameter, using the standard Helm syntax. Multiple parameters can be changed by issuing multiple `set` commands on the command line.
  * **`liqoctl install --set-string [param=value]`**: it changes a single parameter (forcing it to be a string), using the standard Helm syntax. Multiple parameters can be changed by issuing multiple `set-string` commands on the command line.

  The following order, which is displayed in decreasing order of priority, is used to generate the final values:

  1. values provided with `--set` (or `--set-string`)
  2. values provided with `--values` file
  3. liqoctl specific flags (e.g., `--api-server-url` flag)
  4. provider specific values
  5. chart values

  For the parameters that are updated after the initial installation (either by updating their values and re-applying the Helm chart or by re-issuing the proper `liqoctl install [--values | --set | --set-string]` command), please note that not all parameters can be changed at run-time; hence, please check that the command triggered the desired effect.
  A precise list of commands that can be changed at run-time is left for our future work.

### Global

The main global flags, besides those concerning the installation of [development versions](InstallationDevelopmentVersions), include:

* `--enable-metrics`: exposes Liqo **metrics** through **Prometheus** (see the dedicated [Prometheus metrics page](/usage/prometheus-metrics.md) for additional details).
* `--timeout`: configures the timeout for the completion of the installation/upgrade process.
  Once expired, the process is aborted and Liqo is rolled back to the previous version.
* `--verbose`: enables verbose logs, providing additional information concerning the installation/upgrade process (e.g., for troubleshooting).
* `--disable-telemetry`: disables the collection of telemetry data, which is enabled by default.
  The telemetry is used to collect anonymous usage statistics, which are used to improve Liqo.
  Additional details are provided {{ env.config.html_context.generate_link_to_repo('here', 'pkg/telemetry/doc.go') }}.

(InstallControlPlaneFlags)=

### Control plane

The main control plane flags include:

* `--cluster-id`: configures a **name identifying the cluster** in Liqo.
This ID is propagated to remote clusters during the peering process, and used to identify the corresponding remote resource (e.g., virtual nodes, gateways, etc..) used. Additionally, the cluster ID is used as part of the suffix to ensure namespace name uniqueness during the offloading process. In case a cluster name is not specified, it is defaulted to that of the cluster in the cloud provider, if any, or it is automatically generated.
* `--cluster-labels`: a set of **labels** (i.e., key/value pairs) **identifying the cluster in Liqo** (e.g., geographical region, Kubernetes distribution, cloud provider, ...) and automatically propagated during the peering process to the corresponding virtual nodes.
These labels can be used later to **restrict workload offloading to a subset of clusters**, as detailed in the [namespace offloading usage section](/usage/namespace-offloading).

(NetworkFlags)=

### Networking

The main networking flags include:

* `--reserved-subnets`: the list of **private CIDRs to be excluded** from the ones used by Liqo to remap remote clusters in case of address conflicts, as already in use (e.g., the subnet of the cluster nodes).
The Pod CIDR and the Service CIDR shall not be manually specified, as automatically included in the reserved list.

### High availability components

Enables the support for **high-availability of the Liqo components**, starting multiple replicas of the same pod in an active/passive fashion.
This ensures that, even after eventual pod restarts or node failures, exactly one replica is always active while the remaining ones run on standby.
Refer to this [page](ServiceContinuityHA) to see which components you can configure in HA and how to set the the number of desired replicas.

(InstallationHelm)=

## Install with Helm

To install Liqo directly with Helm, you can proceed as follows:

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
   The current step is optional, but it relieves the user from the retrieval of the set of necessary parameters depending on the target provider/distribution.
   Alternatively, the upstream values file can be retrieved through:

   ```bash
   helm show values liqo/liqo > values.yaml
   ```
   ````

4. Appropriately configure the *values* file.
   The full list of options is provided on the dedicated [repository page](https://github.com/liqotech/liqo/tree/master/deployments/liqo).

5. Install Liqo:

    {{ env.config.html_context.generate_helm_install() }}

(InstallationDevelopmentVersions)=

## Install development versions

In addition to released versions (including alpha and beta candidates), *liqoctl* provides the possibility to install **development versions** of Liqo.
Development versions include:

* All commits merged into the master branch of Liqo.
* The commits of *pull requests* to the Liqo repository, whose images have been built through the appropriate bot command.

The installation of a development version of Liqo can be triggered by specifying a **commit *SHA*** through the `--version` flag.
In this case, *liqoctl* proceeds to **clone the repository** (either from the official repository, or from a fork configured through the `--repo-url` flag) at the given revision, and to leverage the Helm chart therein contained:

```bash
liqoctl install <provider> --version <commit-sha> --repo-url <forked-repo-url>
```

Alternatively, the Helm chart can be retrieved from a **local path**, as configured through the `--local-chart-path` flag:

```bash
liqoctl install <provider> --version <commit-sha> --local-chart-path <path-to-local-chart>
```

(InstallationCNIConfiguration)=

## Check installation

After the installation, you can check the status and info about the current instance of Liqo via `liqoctl`:

```bash
liqoctl info
```

The info command is a good way to check the:

* Health of the installation
* Current configuration
* Status and info about active peerings

By default the output is presented in a human-readable form.
However, to simplify automate retrieval of the data, via the `-o` option it is possible to format the output in **JSON or YAML format**.
Moreover via the `--get field.subfield` argument, each field of the reports can be individually retrieved.

For example:

```{code-block} bash
:caption: Get the output in JSON format
liqoctl info -o json
```

```{code-block} bash
:caption: Get the podCIDR of the local Liqo instance
liqoctl info --get network.podcidr
```

## CNIs

### Cilium

Liqo creates a new node for each remote cluster, however we do not schedule daemonsets on these nodes.

From version **1.14.2** cilium adds a taint to the nodes where the daemonset is not scheduled, so that pods are not scheduled on them.
This taint prevents also Liqo pods from being scheduled on the remote nodes.

To solve this issue we need to specify to cilium daemonsets to ignore the Liqo node.
This can be done by adding the following helm values to the cilium installation:

```yaml
affinity:
  nodeAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
        nodeSelectorTerms:
        - matchExpressions:
          - key: liqo.io/type
            operator: DoesNotExist
```

#### Kube-proxy replacement

Liqo networks present a limitation when used with cilium with *kube-proxy replacement*.
In particular you won't be able to expose an **offloaded pod** with a *NodePort* or *LoadBalancer* service.
This not limits the ability to expose **not offloaded pods** like you would normally do.

```{admonition} Note
Please consider that in kubernetes multicluster environments, the use of *NodePort* and *LoadBalancer*
services to expose directly a remote pod is not a [best practice](https://github.com/kubernetes/enhancements/tree/master/keps/sig-multicluster/1645-multi-cluster-services-api#service-types).
```

### Calico

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
