# liqoctl install

Install/upgrade Liqo in the selected cluster

## Description

### Synopsis

Install/upgrade Liqo in the selected cluster.

This command wraps the Helm command to install/upgrade Liqo in the selected
cluster, appropriately configuring it based on the provided flags. Additional
default values can be overridden through the --values and or --set flag.
Alternatively, it can be configured to only output a pre-configured values file,
which can be further customized and used for a manual installation with Helm.

By default, the command installs the latest released version of Liqo, although
this behavior can be tuned through the appropriate flags. In case a development
version is selected, and a local chart path is not specified, the command
proceeds cloning the Liqo repository (or the specified fork) at that version,
and leverages the included Helm chart. This is useful to install unreleased
versions, and during the local testing process.

Instead of directly using this generic command, it is suggested to leverage the
subcommand corresponding to the type of the target cluster (on-premise
distribution or cloud provider), which automatically retrieves most parameters
based on the cluster configuration.



```
liqoctl install [flags]
```

### Examples


```bash
  $ liqoctl install --pod-cidr 10.0.0.0/16 --service-cidr 10.1.0.0/16 \
      --reserved-subnets 172.16.0.0/16,192.16.254.0/24
```

or (configure the cluster id and labels)

```bash
  $ liqoctl install --cluster-id engaged-weevil --pod-cidr 10.0.0.0/16 --service-cidr 10.1.0.0/16 \
      --reserved-subnets 172.16.0.0/16,192.16.254.0/24 --cluster-labels region=europe,environment=staging
```

or (generate and output the values file, instead of performing the installation)

```bash
  $ liqoctl install --pod-cidr 10.0.0.0/16 --service-cidr 10.1.0.0/16 --only-output-values
```

or (install a specific Liqo version)

```bash
  $ liqoctl install --pod-cidr 10.0.0.0/16 --service-cidr 10.1.0.0/16 --version v0.4.0
```

or (install a development version, using the default Helm chart)

```bash
  $ liqoctl install --pod-cidr 10.0.0.0/16 --service-cidr 10.1.0.0/16 \
      --version 2058543d90482baf6f839eb57cbf3a9e81e20abe
```

or (install a development version, using a local Helm chart)

```bash
  $ liqoctl install --pod-cidr 10.0.0.0/16 --service-cidr 10.1.0.0/16 \
      --version 2058543d90482baf6f839eb57cbf3a9e81e20abe --local-chart-path ./liqo/deployments/liqo
```

or (install a development version, cloning the Helm chart from a fork)

```bash
  $ liqoctl install --pod-cidr 10.0.0.0/16 --service-cidr 10.1.0.0/16 \
      --version 2058543d90482baf6f839eb57cbf3a9e81e20abe --repo-url https://github.com/fork/liqo.git
```





### Options
`--api-server-url` _string_:

>The Kubernetes API Server URL (defaults to the one specified in the kubeconfig)

`--cluster-id` _clusterID_:

>The id identifying the cluster in Liqo

`--cluster-labels` _stringMap_:

>The set of labels (i.e., key/value pairs, separated by comma) identifying the current cluster, and propagated to the virtual nodes

`--disable-api-server-sanity-check`

>Disable the sanity checks concerning the retrieved Kubernetes API server URL (default false)

`--disable-kernel-version-check`

>Disable the check of the minimum kernel version required to run the wireguard interface (default false)

`--disable-telemetry`

>Disable the anonymous and aggregated Liqo telemetry collection (default false)

`--dry-run`

>Simulate the installation process (default false)

`--dump-values-path` _string_:

>The path where the generated values file is saved (only in case --only-output-values is set). Default: './values.yaml'

`--enable-metrics`

>Enable metrics exposition through prometheus (default false)

`--local-chart-path` _string_:

>The local path used to retrieve the Helm chart, instead of the upstream one

`-n`, `--namespace` _string_:

>The namespace where Liqo is installed in **(default "liqo")**

`--only-output-values`

>Generate the pre-configured values file for further customization, instead of installing Liqo (default false)

`--pod-cidr` _string_:

>The Pod CIDR of the cluster

`--repo-url` _string_:

>The URL of the git repository used to retrieve the Helm chart, if a non released version is specified **(default "https://github.com/liqotech/liqo")**

`--reserved-subnets` _cidrList_:

>The private CIDRs to be excluded, as already in use (e.g., the subnet of the cluster nodes); PodCIDR and ServiceCIDR shall not be included.

`--service-cidr` _string_:

>The Service CIDR of the cluster

`--set` _stringArray_:

>Set additional values on the command line (can specify multiple times or separate values with commas: key1=val1,key2=val2)

`--set-string` _stringArray_:

>Set additional string values on the command line (can specify multiple times or separate values with commas: key1=val1,key2=val2)

`--skip-validation`

>Skip the validation of the arguments (PodCIDR, ServiceCIDR). This is useful when you are sure of what you are doing and the amount of pods and services in your cluster is very large (default false)

`--timeout` _duration_:

>The timeout for the completion of the installation process **(default 10m0s)**

`--values` _stringArray_:

>Specify values in a YAML file or a URL (can specify multiple)

`--version` _string_:

>The version of Liqo to be installed, among releases and commit SHAs. Defaults to the liqoctl version **(default "unknown")**


### Global options

`--cluster` _string_:

>The name of the kubeconfig cluster to use

`--context` _string_:

>The name of the kubeconfig context to use

`--global-annotations` _stringToString_:

>Global annotations to be added to all created resources (key=value)

`--global-labels` _stringToString_:

>Global labels to be added to all created resources (key=value)

`--kubeconfig` _string_:

>Path to the kubeconfig file to use for CLI requests

`--skip-confirm`

>Skip the confirmation prompt (suggested for automation)

`--user` _string_:

>The name of the kubeconfig user to use

`-v`, `--verbose`

>Enable verbose logs (default false)

## liqoctl install aks

Install Liqo in the selected aks cluster

### Synopsis

Install/upgrade Liqo in the selected aks cluster.

This command wraps the Helm command to install/upgrade Liqo in the selected
aks cluster, automatically retrieving most parameters based on the cluster
configuration.

Please, refer to the help of the root *liqoctl install* command for additional
information and examples concerning its behavior and the common flags.



```
liqoctl install aks [flags]
```

### Examples


```bash
  $ liqoctl install aks --resource-name foo --resource-group-name bar --subscription-id ***
```

or

```bash
  $ liqoctl install aks --resource-name foo --resource-group-name bar --subscription-name ***
```








### Options
`--fqdn` _string_:

>The private AKS cluster fqdn

`--pod-cidr` _string_:

>Pod CIDR of the cluster, only used for AzureCNI (legacy) clusters with no defined subnet

`--private-link`

>Use the private FQDN for the API server

`--resource-group-name` _string_:

>The Azure ResourceGroup name of the cluster

`--resource-name` _string_:

>The Azure Name of the cluster

`--subscription-id` _string_:

>The ID of the Azure Subscription of the cluster (alternative to --subscription-name, takes precedence)

`--subscription-name` _string_:

>The name of the Azure Subscription of the cluster (alternative to --subscription-id)

`--vnet-resource-group-name` _string_:

>The Azure ResourceGroup name of the Virtual Network (defaults to --resource-group-name if not provided)


### Global options

`--cluster` _string_:

>The name of the kubeconfig cluster to use

`--cluster-id` _clusterID_:

>The id identifying the cluster in Liqo

`--cluster-labels` _stringMap_:

>The set of labels (i.e., key/value pairs, separated by comma) identifying the current cluster, and propagated to the virtual nodes

`--context` _string_:

>The name of the kubeconfig context to use

`--disable-api-server-sanity-check`

>Disable the sanity checks concerning the retrieved Kubernetes API server URL (default false)

`--disable-kernel-version-check`

>Disable the check of the minimum kernel version required to run the wireguard interface (default false)

`--disable-telemetry`

>Disable the anonymous and aggregated Liqo telemetry collection (default false)

`--dry-run`

>Simulate the installation process (default false)

`--dump-values-path` _string_:

>The path where the generated values file is saved (only in case --only-output-values is set). Default: './values.yaml'

`--enable-metrics`

>Enable metrics exposition through prometheus (default false)

`--global-annotations` _stringToString_:

>Global annotations to be added to all created resources (key=value)

`--global-labels` _stringToString_:

>Global labels to be added to all created resources (key=value)

`--kubeconfig` _string_:

>Path to the kubeconfig file to use for CLI requests

`--local-chart-path` _string_:

>The local path used to retrieve the Helm chart, instead of the upstream one

`-n`, `--namespace` _string_:

>The namespace where Liqo is installed in **(default "liqo")**

`--only-output-values`

>Generate the pre-configured values file for further customization, instead of installing Liqo (default false)

`--repo-url` _string_:

>The URL of the git repository used to retrieve the Helm chart, if a non released version is specified **(default "https://github.com/liqotech/liqo")**

`--reserved-subnets` _cidrList_:

>The private CIDRs to be excluded, as already in use (e.g., the subnet of the cluster nodes); PodCIDR and ServiceCIDR shall not be included.

`--set` _stringArray_:

>Set additional values on the command line (can specify multiple times or separate values with commas: key1=val1,key2=val2)

`--set-string` _stringArray_:

>Set additional string values on the command line (can specify multiple times or separate values with commas: key1=val1,key2=val2)

`--skip-confirm`

>Skip the confirmation prompt (suggested for automation)

`--skip-validation`

>Skip the validation of the arguments (PodCIDR, ServiceCIDR). This is useful when you are sure of what you are doing and the amount of pods and services in your cluster is very large (default false)

`--timeout` _duration_:

>The timeout for the completion of the installation process **(default 10m0s)**

`--user` _string_:

>The name of the kubeconfig user to use

`--values` _stringArray_:

>Specify values in a YAML file or a URL (can specify multiple)

`-v`, `--verbose`

>Enable verbose logs (default false)

`--version` _string_:

>The version of Liqo to be installed, among releases and commit SHAs. Defaults to the liqoctl version **(default "unknown")**

## liqoctl install eks

Install Liqo in the selected eks cluster

### Synopsis

Install/upgrade Liqo in the selected eks cluster.

This command wraps the Helm command to install/upgrade Liqo in the selected
eks cluster, automatically retrieving most parameters based on the cluster
configuration.

Please, refer to the help of the root *liqoctl install* command for additional
information and examples concerning its behavior and the common flags.



```
liqoctl install eks [flags]
```

### Examples


```bash
  $ liqoctl install eks --eks-cluster-region us-east-2 --eks-cluster-name foo
```

or

```bash
  $ liqoctl install eks --eks-cluster-region us-east-2 --eks-cluster-name foo \
      --user-name custom --policy-name custom-policy --access-key-id *** --secret-access-key ***
```








### Options
`--access-key-id` _string_:

>The IAM AccessKeyID for the Liqo user (optional)

`--eks-cluster-name` _string_:

>The EKS cluster name of the cluster

`--eks-cluster-region` _string_:

>The EKS region where the cluster is running

`--policy-name` _string_:

>The name of the policy assigned to the Liqo user (optional) **(default "liqo-cluster-policy")**

`--secret-access-key` _string_:

>The IAM SecretAccessKey for the Liqo user (optional)

`--user-name` _string_:

>The username of the Liqo user (automatically created if no access keys are provided) **(default "liqo-cluster-user")**


### Global options

`--cluster` _string_:

>The name of the kubeconfig cluster to use

`--cluster-id` _clusterID_:

>The id identifying the cluster in Liqo

`--cluster-labels` _stringMap_:

>The set of labels (i.e., key/value pairs, separated by comma) identifying the current cluster, and propagated to the virtual nodes

`--context` _string_:

>The name of the kubeconfig context to use

`--disable-api-server-sanity-check`

>Disable the sanity checks concerning the retrieved Kubernetes API server URL (default false)

`--disable-kernel-version-check`

>Disable the check of the minimum kernel version required to run the wireguard interface (default false)

`--disable-telemetry`

>Disable the anonymous and aggregated Liqo telemetry collection (default false)

`--dry-run`

>Simulate the installation process (default false)

`--dump-values-path` _string_:

>The path where the generated values file is saved (only in case --only-output-values is set). Default: './values.yaml'

`--enable-metrics`

>Enable metrics exposition through prometheus (default false)

`--global-annotations` _stringToString_:

>Global annotations to be added to all created resources (key=value)

`--global-labels` _stringToString_:

>Global labels to be added to all created resources (key=value)

`--kubeconfig` _string_:

>Path to the kubeconfig file to use for CLI requests

`--local-chart-path` _string_:

>The local path used to retrieve the Helm chart, instead of the upstream one

`-n`, `--namespace` _string_:

>The namespace where Liqo is installed in **(default "liqo")**

`--only-output-values`

>Generate the pre-configured values file for further customization, instead of installing Liqo (default false)

`--repo-url` _string_:

>The URL of the git repository used to retrieve the Helm chart, if a non released version is specified **(default "https://github.com/liqotech/liqo")**

`--reserved-subnets` _cidrList_:

>The private CIDRs to be excluded, as already in use (e.g., the subnet of the cluster nodes); PodCIDR and ServiceCIDR shall not be included.

`--set` _stringArray_:

>Set additional values on the command line (can specify multiple times or separate values with commas: key1=val1,key2=val2)

`--set-string` _stringArray_:

>Set additional string values on the command line (can specify multiple times or separate values with commas: key1=val1,key2=val2)

`--skip-confirm`

>Skip the confirmation prompt (suggested for automation)

`--skip-validation`

>Skip the validation of the arguments (PodCIDR, ServiceCIDR). This is useful when you are sure of what you are doing and the amount of pods and services in your cluster is very large (default false)

`--timeout` _duration_:

>The timeout for the completion of the installation process **(default 10m0s)**

`--user` _string_:

>The name of the kubeconfig user to use

`--values` _stringArray_:

>Specify values in a YAML file or a URL (can specify multiple)

`-v`, `--verbose`

>Enable verbose logs (default false)

`--version` _string_:

>The version of Liqo to be installed, among releases and commit SHAs. Defaults to the liqoctl version **(default "unknown")**

## liqoctl install gke

Install Liqo in the selected gke cluster

### Synopsis

Install/upgrade Liqo in the selected gke cluster.

This command wraps the Helm command to install/upgrade Liqo in the selected
gke cluster, automatically retrieving most parameters based on the cluster
configuration.

Please, refer to the help of the root *liqoctl install* command for additional
information and examples concerning its behavior and the common flags.



```
liqoctl install gke [flags]
```

### Examples


```bash
  $ liqoctl install gke --credentials-path ~/.liqo/gcp_service_account \
      --cluster-id foo --project-id bar --zone europe-west-1b
```

or (regional cluster)

```bash
  $ liqoctl install gke --credentials-path ~/.liqo/gcp_service_account \
      --cluster-id foo --project-id bar --region europe-west-1
```








### Options
`--cluster-id` _string_:

>The GKE clusterID of the cluster

`--credentials-path` _string_:

>The path to the GCP credentials JSON file (c.f. https://cloud.google.com/docs/authentication/production#create_service_account

`--project-id` _string_:

>The GCP project where the cluster is deployed in

`--region` _string_:

>The GCP region where the cluster is running

`--zone` _string_:

>The GCP zone where the cluster is running


### Global options

`--cluster` _string_:

>The name of the kubeconfig cluster to use

`--cluster-labels` _stringMap_:

>The set of labels (i.e., key/value pairs, separated by comma) identifying the current cluster, and propagated to the virtual nodes

`--context` _string_:

>The name of the kubeconfig context to use

`--disable-api-server-sanity-check`

>Disable the sanity checks concerning the retrieved Kubernetes API server URL (default false)

`--disable-kernel-version-check`

>Disable the check of the minimum kernel version required to run the wireguard interface (default false)

`--disable-telemetry`

>Disable the anonymous and aggregated Liqo telemetry collection (default false)

`--dry-run`

>Simulate the installation process (default false)

`--dump-values-path` _string_:

>The path where the generated values file is saved (only in case --only-output-values is set). Default: './values.yaml'

`--enable-metrics`

>Enable metrics exposition through prometheus (default false)

`--global-annotations` _stringToString_:

>Global annotations to be added to all created resources (key=value)

`--global-labels` _stringToString_:

>Global labels to be added to all created resources (key=value)

`--kubeconfig` _string_:

>Path to the kubeconfig file to use for CLI requests

`--local-chart-path` _string_:

>The local path used to retrieve the Helm chart, instead of the upstream one

`-n`, `--namespace` _string_:

>The namespace where Liqo is installed in **(default "liqo")**

`--only-output-values`

>Generate the pre-configured values file for further customization, instead of installing Liqo (default false)

`--repo-url` _string_:

>The URL of the git repository used to retrieve the Helm chart, if a non released version is specified **(default "https://github.com/liqotech/liqo")**

`--reserved-subnets` _cidrList_:

>The private CIDRs to be excluded, as already in use (e.g., the subnet of the cluster nodes); PodCIDR and ServiceCIDR shall not be included.

`--set` _stringArray_:

>Set additional values on the command line (can specify multiple times or separate values with commas: key1=val1,key2=val2)

`--set-string` _stringArray_:

>Set additional string values on the command line (can specify multiple times or separate values with commas: key1=val1,key2=val2)

`--skip-confirm`

>Skip the confirmation prompt (suggested for automation)

`--skip-validation`

>Skip the validation of the arguments (PodCIDR, ServiceCIDR). This is useful when you are sure of what you are doing and the amount of pods and services in your cluster is very large (default false)

`--timeout` _duration_:

>The timeout for the completion of the installation process **(default 10m0s)**

`--user` _string_:

>The name of the kubeconfig user to use

`--values` _stringArray_:

>Specify values in a YAML file or a URL (can specify multiple)

`-v`, `--verbose`

>Enable verbose logs (default false)

`--version` _string_:

>The version of Liqo to be installed, among releases and commit SHAs. Defaults to the liqoctl version **(default "unknown")**

## liqoctl install k3s

Install Liqo in the selected k3s cluster

### Synopsis

Install/upgrade Liqo in the selected k3s cluster.

This command wraps the Helm command to install/upgrade Liqo in the selected
k3s cluster, automatically retrieving most parameters based on the cluster
configuration.

Please, refer to the help of the root *liqoctl install* command for additional
information and examples concerning its behavior and the common flags.



```
liqoctl install k3s [flags]
```

### Examples


```bash
  $ liqoctl install k3s --api-server-url https://liqo.example.local:6443 \
      --cluster-labels region=europe,environment=staging \
      --reserved-subnets 172.16.0.0/16,192.16.254.0/24
```

or

```bash
  $ liqoctl install k3s --api-server-url https://liqo.example.local:6443 \
      --cluster-labels region=europe,environment=staging \
      --pod-cidr 10.0.0.0/16 --service-cidr 10.1.0.0/16 \
      --reserved-subnets 172.16.0.0/16,192.16.254.0/24
```








### Options
`--api-server-url` _string_:

>The Kubernetes API Server URL (defaults to the one specified in the kubeconfig)

`--pod-cidr` _string_:

>The Pod CIDR of the cluster **(default "10.42.0.0/16")**

`--service-cidr` _string_:

>The Service CIDR of the cluster **(default "10.43.0.0/16")**


### Global options

`--cluster` _string_:

>The name of the kubeconfig cluster to use

`--cluster-id` _clusterID_:

>The id identifying the cluster in Liqo

`--cluster-labels` _stringMap_:

>The set of labels (i.e., key/value pairs, separated by comma) identifying the current cluster, and propagated to the virtual nodes

`--context` _string_:

>The name of the kubeconfig context to use

`--disable-api-server-sanity-check`

>Disable the sanity checks concerning the retrieved Kubernetes API server URL (default false)

`--disable-kernel-version-check`

>Disable the check of the minimum kernel version required to run the wireguard interface (default false)

`--disable-telemetry`

>Disable the anonymous and aggregated Liqo telemetry collection (default false)

`--dry-run`

>Simulate the installation process (default false)

`--dump-values-path` _string_:

>The path where the generated values file is saved (only in case --only-output-values is set). Default: './values.yaml'

`--enable-metrics`

>Enable metrics exposition through prometheus (default false)

`--global-annotations` _stringToString_:

>Global annotations to be added to all created resources (key=value)

`--global-labels` _stringToString_:

>Global labels to be added to all created resources (key=value)

`--kubeconfig` _string_:

>Path to the kubeconfig file to use for CLI requests

`--local-chart-path` _string_:

>The local path used to retrieve the Helm chart, instead of the upstream one

`-n`, `--namespace` _string_:

>The namespace where Liqo is installed in **(default "liqo")**

`--only-output-values`

>Generate the pre-configured values file for further customization, instead of installing Liqo (default false)

`--repo-url` _string_:

>The URL of the git repository used to retrieve the Helm chart, if a non released version is specified **(default "https://github.com/liqotech/liqo")**

`--reserved-subnets` _cidrList_:

>The private CIDRs to be excluded, as already in use (e.g., the subnet of the cluster nodes); PodCIDR and ServiceCIDR shall not be included.

`--set` _stringArray_:

>Set additional values on the command line (can specify multiple times or separate values with commas: key1=val1,key2=val2)

`--set-string` _stringArray_:

>Set additional string values on the command line (can specify multiple times or separate values with commas: key1=val1,key2=val2)

`--skip-confirm`

>Skip the confirmation prompt (suggested for automation)

`--skip-validation`

>Skip the validation of the arguments (PodCIDR, ServiceCIDR). This is useful when you are sure of what you are doing and the amount of pods and services in your cluster is very large (default false)

`--timeout` _duration_:

>The timeout for the completion of the installation process **(default 10m0s)**

`--user` _string_:

>The name of the kubeconfig user to use

`--values` _stringArray_:

>Specify values in a YAML file or a URL (can specify multiple)

`-v`, `--verbose`

>Enable verbose logs (default false)

`--version` _string_:

>The version of Liqo to be installed, among releases and commit SHAs. Defaults to the liqoctl version **(default "unknown")**

## liqoctl install kind

Install Liqo in the selected kind cluster

### Synopsis

Install/upgrade Liqo in the selected kind cluster.

This command wraps the Helm command to install/upgrade Liqo in the selected
kind cluster, automatically retrieving most parameters based on the cluster
configuration.

Please, refer to the help of the root *liqoctl install* command for additional
information and examples concerning its behavior and the common flags.



```
liqoctl install kind [flags]
```

### Examples


```bash
  $ liqoctl install kind --cluster-labels region=europe,environment=staging
```








### Options

### Global options

`--cluster` _string_:

>The name of the kubeconfig cluster to use

`--cluster-id` _clusterID_:

>The id identifying the cluster in Liqo

`--cluster-labels` _stringMap_:

>The set of labels (i.e., key/value pairs, separated by comma) identifying the current cluster, and propagated to the virtual nodes

`--context` _string_:

>The name of the kubeconfig context to use

`--disable-api-server-sanity-check`

>Disable the sanity checks concerning the retrieved Kubernetes API server URL (default false)

`--disable-kernel-version-check`

>Disable the check of the minimum kernel version required to run the wireguard interface (default false)

`--disable-telemetry`

>Disable the anonymous and aggregated Liqo telemetry collection (default false)

`--dry-run`

>Simulate the installation process (default false)

`--dump-values-path` _string_:

>The path where the generated values file is saved (only in case --only-output-values is set). Default: './values.yaml'

`--enable-metrics`

>Enable metrics exposition through prometheus (default false)

`--global-annotations` _stringToString_:

>Global annotations to be added to all created resources (key=value)

`--global-labels` _stringToString_:

>Global labels to be added to all created resources (key=value)

`--kubeconfig` _string_:

>Path to the kubeconfig file to use for CLI requests

`--local-chart-path` _string_:

>The local path used to retrieve the Helm chart, instead of the upstream one

`-n`, `--namespace` _string_:

>The namespace where Liqo is installed in **(default "liqo")**

`--only-output-values`

>Generate the pre-configured values file for further customization, instead of installing Liqo (default false)

`--repo-url` _string_:

>The URL of the git repository used to retrieve the Helm chart, if a non released version is specified **(default "https://github.com/liqotech/liqo")**

`--reserved-subnets` _cidrList_:

>The private CIDRs to be excluded, as already in use (e.g., the subnet of the cluster nodes); PodCIDR and ServiceCIDR shall not be included.

`--set` _stringArray_:

>Set additional values on the command line (can specify multiple times or separate values with commas: key1=val1,key2=val2)

`--set-string` _stringArray_:

>Set additional string values on the command line (can specify multiple times or separate values with commas: key1=val1,key2=val2)

`--skip-confirm`

>Skip the confirmation prompt (suggested for automation)

`--skip-validation`

>Skip the validation of the arguments (PodCIDR, ServiceCIDR). This is useful when you are sure of what you are doing and the amount of pods and services in your cluster is very large (default false)

`--timeout` _duration_:

>The timeout for the completion of the installation process **(default 10m0s)**

`--user` _string_:

>The name of the kubeconfig user to use

`--values` _stringArray_:

>Specify values in a YAML file or a URL (can specify multiple)

`-v`, `--verbose`

>Enable verbose logs (default false)

`--version` _string_:

>The version of Liqo to be installed, among releases and commit SHAs. Defaults to the liqoctl version **(default "unknown")**

## liqoctl install kubeadm

Install Liqo in the selected kubeadm cluster

### Synopsis

Install/upgrade Liqo in the selected kubeadm cluster.

This command wraps the Helm command to install/upgrade Liqo in the selected
kubeadm cluster, automatically retrieving most parameters based on the cluster
configuration.

Please, refer to the help of the root *liqoctl install* command for additional
information and examples concerning its behavior and the common flags.



```
liqoctl install kubeadm [flags]
```

### Examples


```bash
  $ liqoctl install kubeadm --cluster-labels region=europe,environment=staging \
      --reserved-subnets 172.16.0.0/16,192.16.254.0/24
```








### Options

### Global options

`--cluster` _string_:

>The name of the kubeconfig cluster to use

`--cluster-id` _clusterID_:

>The id identifying the cluster in Liqo

`--cluster-labels` _stringMap_:

>The set of labels (i.e., key/value pairs, separated by comma) identifying the current cluster, and propagated to the virtual nodes

`--context` _string_:

>The name of the kubeconfig context to use

`--disable-api-server-sanity-check`

>Disable the sanity checks concerning the retrieved Kubernetes API server URL (default false)

`--disable-kernel-version-check`

>Disable the check of the minimum kernel version required to run the wireguard interface (default false)

`--disable-telemetry`

>Disable the anonymous and aggregated Liqo telemetry collection (default false)

`--dry-run`

>Simulate the installation process (default false)

`--dump-values-path` _string_:

>The path where the generated values file is saved (only in case --only-output-values is set). Default: './values.yaml'

`--enable-metrics`

>Enable metrics exposition through prometheus (default false)

`--global-annotations` _stringToString_:

>Global annotations to be added to all created resources (key=value)

`--global-labels` _stringToString_:

>Global labels to be added to all created resources (key=value)

`--kubeconfig` _string_:

>Path to the kubeconfig file to use for CLI requests

`--local-chart-path` _string_:

>The local path used to retrieve the Helm chart, instead of the upstream one

`-n`, `--namespace` _string_:

>The namespace where Liqo is installed in **(default "liqo")**

`--only-output-values`

>Generate the pre-configured values file for further customization, instead of installing Liqo (default false)

`--repo-url` _string_:

>The URL of the git repository used to retrieve the Helm chart, if a non released version is specified **(default "https://github.com/liqotech/liqo")**

`--reserved-subnets` _cidrList_:

>The private CIDRs to be excluded, as already in use (e.g., the subnet of the cluster nodes); PodCIDR and ServiceCIDR shall not be included.

`--set` _stringArray_:

>Set additional values on the command line (can specify multiple times or separate values with commas: key1=val1,key2=val2)

`--set-string` _stringArray_:

>Set additional string values on the command line (can specify multiple times or separate values with commas: key1=val1,key2=val2)

`--skip-confirm`

>Skip the confirmation prompt (suggested for automation)

`--skip-validation`

>Skip the validation of the arguments (PodCIDR, ServiceCIDR). This is useful when you are sure of what you are doing and the amount of pods and services in your cluster is very large (default false)

`--timeout` _duration_:

>The timeout for the completion of the installation process **(default 10m0s)**

`--user` _string_:

>The name of the kubeconfig user to use

`--values` _stringArray_:

>Specify values in a YAML file or a URL (can specify multiple)

`-v`, `--verbose`

>Enable verbose logs (default false)

`--version` _string_:

>The version of Liqo to be installed, among releases and commit SHAs. Defaults to the liqoctl version **(default "unknown")**

## liqoctl install openshift

Install Liqo in the selected openshift cluster

### Synopsis

Install/upgrade Liqo in the selected openshift cluster.

This command wraps the Helm command to install/upgrade Liqo in the selected
openshift cluster, automatically retrieving most parameters based on the cluster
configuration.

Please, refer to the help of the root *liqoctl install* command for additional
information and examples concerning its behavior and the common flags.



```
liqoctl install openshift [flags]
```

### Examples


```bash
  $ liqoctl install openshift --cluster-labels region=europe,environment=staging \
      --reserved-subnets 172.16.0.0/16,192.16.254.0/24
```








### Options

### Global options

`--cluster` _string_:

>The name of the kubeconfig cluster to use

`--cluster-id` _clusterID_:

>The id identifying the cluster in Liqo

`--cluster-labels` _stringMap_:

>The set of labels (i.e., key/value pairs, separated by comma) identifying the current cluster, and propagated to the virtual nodes

`--context` _string_:

>The name of the kubeconfig context to use

`--disable-api-server-sanity-check`

>Disable the sanity checks concerning the retrieved Kubernetes API server URL (default false)

`--disable-kernel-version-check`

>Disable the check of the minimum kernel version required to run the wireguard interface (default false)

`--disable-telemetry`

>Disable the anonymous and aggregated Liqo telemetry collection (default false)

`--dry-run`

>Simulate the installation process (default false)

`--dump-values-path` _string_:

>The path where the generated values file is saved (only in case --only-output-values is set). Default: './values.yaml'

`--enable-metrics`

>Enable metrics exposition through prometheus (default false)

`--global-annotations` _stringToString_:

>Global annotations to be added to all created resources (key=value)

`--global-labels` _stringToString_:

>Global labels to be added to all created resources (key=value)

`--kubeconfig` _string_:

>Path to the kubeconfig file to use for CLI requests

`--local-chart-path` _string_:

>The local path used to retrieve the Helm chart, instead of the upstream one

`-n`, `--namespace` _string_:

>The namespace where Liqo is installed in **(default "liqo")**

`--only-output-values`

>Generate the pre-configured values file for further customization, instead of installing Liqo (default false)

`--repo-url` _string_:

>The URL of the git repository used to retrieve the Helm chart, if a non released version is specified **(default "https://github.com/liqotech/liqo")**

`--reserved-subnets` _cidrList_:

>The private CIDRs to be excluded, as already in use (e.g., the subnet of the cluster nodes); PodCIDR and ServiceCIDR shall not be included.

`--set` _stringArray_:

>Set additional values on the command line (can specify multiple times or separate values with commas: key1=val1,key2=val2)

`--set-string` _stringArray_:

>Set additional string values on the command line (can specify multiple times or separate values with commas: key1=val1,key2=val2)

`--skip-confirm`

>Skip the confirmation prompt (suggested for automation)

`--skip-validation`

>Skip the validation of the arguments (PodCIDR, ServiceCIDR). This is useful when you are sure of what you are doing and the amount of pods and services in your cluster is very large (default false)

`--timeout` _duration_:

>The timeout for the completion of the installation process **(default 10m0s)**

`--user` _string_:

>The name of the kubeconfig user to use

`--values` _stringArray_:

>Specify values in a YAML file or a URL (can specify multiple)

`-v`, `--verbose`

>Enable verbose logs (default false)

`--version` _string_:

>The version of Liqo to be installed, among releases and commit SHAs. Defaults to the liqoctl version **(default "unknown")**

