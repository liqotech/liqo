# Provision with Terraform

Terraform is a widely used Infrastructure as Code (IaC) tool that allows engineers to define their software infrastructure in code.

This tutorial aims at presenting how to set up an environment with Liqo installed via Terraform.

You will learn how to create a *virtual cluster* by peering two Kubernetes clusters and offload a namespace using the *Generate*, *Peer* and *Offload* resources provided by the Liqo provider.

## Provision the infrastructure

First, check that you are compliant with the [requirements](/examples/requirements.md).
Additionally, this example requires [Terraform](https://developer.hashicorp.com/terraform/tutorials/aws-get-started/install-cli) to be installed in your system.

Then, let's open a terminal on your machine and launch the following script, which creates the infrastructure used in this example (i.e., two KinD clusters, peered with Liqo), with all playground already set up.

This tutorial will present a detailed description about how this result is achieved, by analyzing the most notable parts of the Terraform infrastructure definition file that refer to Liqo.

{{ env.config.html_context.generate_clone_example_tf('provision-with-terraform') }}

## Analyze the infrastructure and code

Inspecting `Terraform main` file within the examples/provision-with-terraform folder you can see the Terraform configuration file analyzed below.
With the previous command you created two KinD clusters, installed Liqo and established an outgoing peering from local to remote cluster.
Furthermore, you offloaded a namespace to virtual node (i.e., remote cluster).
In this way the namespace will leverage on both local and remote clusters resources following offloading configuration.

This example is provisioned on KinD, since it requires no particular configurations (e.g., concerning accounts), and does not lead to resource costs.
Yet, all the presented functionalities work also on other clusters, e.g., the ones operated by public cloud providers.

### Provision the clusters

The first step executed by Terraform is the creation of the two KinD clusters: the resource in charge of building them is the `kind_cluster` resource of the provider [tehcyx](https://registry.terraform.io/providers/tehcyx/kind/latest/docs) that, starting from configuration parameters (such as *cluster_name*, *service_subnet/pod_subnet*), generates the clusters and related config files needed by other providers to set up the infrastructure.

You can inspect the deployed clusters by typing on your workstation:

```bash
kind get clusters
```

You should see a couple of entries:

```text
milan
rome
```

This means that two KinD clusters are deployed and running on your host.

Then, you can simply inspect the status of the clusters.
To do so, you can export the `KUBECONFIG` variable to specify the identity file for *kubectl* and *liqoctl*, and then contact the cluster.

By default, the kubeconfigs of the two clusters are stored in the current directory (`./rome-config`, `./milan-config`).
You can export the appropriate environment variables leveraged for the rest of the tutorial (i.e., `KUBECONFIG` and `KUBECONFIG_MILAN`), and referring to their location, through the following:

```bash
export KUBECONFIG="$PWD/rome-config"
export KUBECONFIG_MILAN="$PWD/milan-config"
```

```{admonition} Note
We suggest exporting the kubeconfig of the first cluster as default (i.e., `KUBECONFIG`), since it will be the entry point of the virtual cluster and you will mainly interact with it.
```

### Install Liqo

After creating the two KinD clusters, Terraform will install Liqo using the `helm_release` resource of the [Helm provider](https://registry.terraform.io/providers/hashicorp/helm/latest/docs) configured with the cluster config files.
Once the installation is complete, you should see the Liqo system pods up and running on both clusters:

```bash
kubectl get pods -n liqo
```

```text
NAME                                       READY   STATUS    RESTARTS   AGE
liqo-auth-74c795d84c-x2p6h                 1/1     Running   0          2m8s
liqo-controller-manager-6c688c777f-4lv9d   1/1     Running   0          2m8s
liqo-crd-replicator-6c64df5457-bq4tv       1/1     Running   0          2m8s
liqo-gateway-78cf7bb86b-pkdpt              1/1     Running   0          2m8s
liqo-metric-agent-5667b979c7-snmdg         1/1     Running   0          2m8s
liqo-network-manager-5b5cdcfcf7-scvd9      1/1     Running   0          2m8s
liqo-proxy-6674dd7bbd-kr2ls                1/1     Running   0          2m8s
liqo-route-7wsrx                           1/1     Running   0          2m8s
liqo-route-sz75m                           1/1     Running   0          2m8s
```

### Extract the peering parameters

Once the Liqo installation in the remote cluster is complete, Terraform will extract the authentication parameters required to peer the local (i.e., *Rome*) cluster with the remote one (i.e., *Milan*).

This is achieved with the `liqo_generate` resource of the [liqo provider](https://registry.terraform.io/providers/liqotech/liqo/latest/docs) instance, configured with either the config file or the full list of parameters of the remote cluster:

```terraform
provider "liqo" {
  alias = "milan"
  kubernetes = {
    config_path = kind_cluster.milan.kubeconfig_path
  }
}

resource "liqo_generate" "generate" {

  depends_on = [
    helm_release.install_liqo_milan
  ]

  provider = liqo.milan

}
```

### Run the peering procedure

Once the `generate_resource` is created, Terraform will continue with the [out-of-band peering](/features/peering.md) procedure leveraging the output parameters of the previous resource.

This is achieved with the `liqo_peer` resource of the [liqo provider](https://registry.terraform.io/providers/liqotech/liqo/latest/docs) instance, configured with either the config file or the full list of parameters of the local cluster:

```terraform
provider "liqo" {
  alias = "rome"
  kubernetes = {
    config_path = kind_cluster.rome.kubeconfig_path
  }
}

resource "liqo_peer" "peer" {

  depends_on = [
    helm_release.install_liqo_rome
  ]

  provider = liqo.rome

  cluster_id      = liqo_generate.generate.cluster_id
  cluster_name    = liqo_generate.generate.cluster_name
  cluster_authurl = liqo_generate.generate.auth_ep
  cluster_token   = liqo_generate.generate.local_token

}
```

You can check the peering status by running:

```bash
kubectl get foreignclusters
```

The output should look like the following, indicating that the cross-cluster network tunnel has been established, and an outgoing peering is currently active (i.e., the *Rome* cluster can offload workloads to the *Milan* one, but not vice versa):

```text
NAME    TYPE        OUTGOING PEERING   INCOMING PEERING   NETWORKING    AUTHENTICATION   AGE
milan   OutOfBand   Established        None               Established   Established      12s
```

At the same time, you should see a virtual node (`liqo-milan`) in addition to your physical nodes:

```bash
kubectl get nodes
```

```text
NAME                 STATUS   ROLES                  AGE     VERSION
liqo-milan           Ready    agent                  14s     v1.25.0
rome-control-plane   Ready    control-plane,master   7m56s   v1.25.0
rome-worker          Ready    <none>                 7m25s   v1.25.0
```

### Offload a namespace

If you want to deploy an application that is scheduled on a Liqo virtual node (hence, it is offloaded on a remote cluster), you should first create a namespace where your pod will be started.

This can be achieved with the `kubernetes_namespace` resource of the [kubernetes provider](https://registry.terraform.io/providers/hashicorp/kubernetes/latest/docs), configured with either the config file or the full list of parameters of the local cluster.
Then tell Liqo to make this namespace eligible for the pod offloading.

The resource in charge of doing this is `liqo_offload` of the same [liqo provider](https://registry.terraform.io/providers/liqotech/liqo/latest/docs) instance of `liqo_peer` resource:

```terraform
resource "liqo_offload" "offload" {

  depends_on = [
    helm_release.install_liqo_rome,
    kubernetes_namespace.namespace
  ]

  provider = liqo.rome

  namespace = "liqo-demo"

}
```

```{admonition} Note
Liqo virtual nodes have a [taint](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/) that prevents the pods from being scheduled on them.
The Liqo webhook will add the toleration for this taint to the pods created in the liqo-enabled namespaces.
```

Since no further configuration is provided, Liqo will add a suffix to the namespace name to make it unique on the remote cluster (see the dedicated [usage page](/usage/namespace-offloading.md) for additional information concerning namespace offloading configurations).

You can now test the infrastructure you have just created by deploying an application.
For this, you can follow the [proper example section](ExamplesStartHelloWorldApplication) in the Quick Start page.

## Tear down the infrastructure

To tear down all the infrastrucure you only need to run the following command:

```bash
terraform destroy
```

This command will destroy all the resources starting from the last one created, from bottom to top.

If you want to destroy a specific resource (for example to unpeer a cluster or to unoffload a namespace) you can leverage on the `-target` flag of `destroy` command.
For example, you can run the following command to unpeer two clusters:

```bash
terraform destroy -target="liqo_peer.peer"
```

```{warning}
The Terraform `destroy` command will destroy all resources that have a dependence on the one that has to be destroyed.
```
