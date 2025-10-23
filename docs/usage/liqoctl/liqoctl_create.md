# liqoctl create

Create Liqo resources

## Description

### Synopsis

Create Liqo resources.


## liqoctl create configuration

Create a Configuration

### Synopsis

Create a Configuration.

The Configuration resource is used to represent a remote cluster network configuration.



```
liqoctl create configuration [flags]
```

### Examples


```bash
  $ liqoctl create configuration my-cluster --remote-cluster-id remote-cluster-id \
  --pod-cidr 10.0.0.0/16 --external-cidr 10.10.0.0/16
```


### Options
`--external-cidr` _cidr_:

>The external CIDR of the remote cluster **(default <nil>)**

`-o`, `--output` _string_:

>Output format of the resulting Configuration resource. Supported formats: json, yaml

`--pod-cidr` _cidr_:

>The pod CIDR of the remote cluster **(default <nil>)**

`--remote-cluster-id` _clusterID_:

>The cluster ID of the remote cluster

`--wait`

>Wait for the Configuration to be ready


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

`--liqo-namespace` _string_:

>The namespace where Liqo is installed in **(default "liqo")**

`-n`, `--namespace` _string_:

>The namespace scope for this request

`--skip-confirm`

>Skip the confirmation prompt (suggested for automation)

`--user` _string_:

>The name of the kubeconfig user to use

`-v`, `--verbose`

>Enable verbose logs (default false)

## liqoctl create gatewayclient

Create a Gateway Client

### Synopsis

Create a Gateway Client.

The GatewayClient resource is used to define a Gateway Client for the external network.



```
liqoctl create gatewayclient [flags]
```

### Examples


```bash
  $ liqoctl create gatewayclient my-gw-client \
  --remote-cluster-id remote-cluster-id \
  --type networking.liqo.io/v1beta1/wggatewayclients
```


### Options
`--addresses` _strings_:

>Addresses of Gateway Server

`--mtu` _int_:

>MTU of Gateway Client **(default 1340)**

`-o`, `--output` _string_:

>Output the resulting GatewayClient resource, instead of applying it. Supported formats: json, yaml

`--port` _int32_:

>Port of Gateway Server

`--protocol` _string_:

>Gateway Protocol **(default "UDP")**

`--remote-cluster-id` _clusterID_:

>The cluster ID of the remote cluster

`--template-name` _string_:

>Name of the Gateway Client template **(default "wireguard-client")**

`--template-namespace` _string_:

>Namespace of the Gateway Client template

`--type` _string_:

>Type of Gateway Client. Default: wireguard **(default "networking.liqo.io/v1beta1/wggatewayclienttemplates")**

`--wait`

>Wait for the Gateway Client to be ready


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

`--liqo-namespace` _string_:

>The namespace where Liqo is installed in **(default "liqo")**

`-n`, `--namespace` _string_:

>The namespace scope for this request

`--skip-confirm`

>Skip the confirmation prompt (suggested for automation)

`--user` _string_:

>The name of the kubeconfig user to use

`-v`, `--verbose`

>Enable verbose logs (default false)

## liqoctl create gatewayserver

Create a Gateway Server

### Synopsis

Create a Gateway Server.

The GatewayServer resource is used to define a Gateway Server for the external network.



```
liqoctl create gatewayserver [flags]
```

### Examples


```bash
  $ liqoctl create gatewayserver my-gw-server \
  --remote-cluster-id remote-cluster-id \
  --type networking.liqo.io/v1beta1/wggatewayservers --service-type LoadBalancer
```


### Options
`--load-balancer-ip` _string_:

>Force LoadBalancer IP of the Gateway Server. Leave empty to use the one provided by the LoadBalancer provider

`--mtu` _int_:

>MTU of Gateway Server **(default 1340)**

`--node-port` _int32_:

>Force the NodePort of the Gateway Server. Leave empty to let Kubernetes allocate a random NodePort

`-o`, `--output` _string_:

>Output the resulting GatewayServer resource, instead of applying it. Supported formats: json, yaml

`--port` _int32_:

>Port of Gateway Server **(default 51840)**

`--remote-cluster-id` _clusterID_:

>The cluster ID of the remote cluster

`--service-type` _string_:

>Service type of Gateway Server. Default: LoadBalancer **(default "LoadBalancer")**

`--template-name` _string_:

>Name of the Gateway Server template **(default "wireguard-server")**

`--template-namespace` _string_:

>Namespace of the Gateway Server template

`--type` _string_:

>Type of Gateway Server. Leave empty to use default Liqo implementation of WireGuard **(default "networking.liqo.io/v1beta1/wggatewayservertemplates")**

`--wait`

>Wait for the Gateway Server to be ready


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

`--liqo-namespace` _string_:

>The namespace where Liqo is installed in **(default "liqo")**

`-n`, `--namespace` _string_:

>The namespace scope for this request

`--skip-confirm`

>Skip the confirmation prompt (suggested for automation)

`--user` _string_:

>The name of the kubeconfig user to use

`-v`, `--verbose`

>Enable verbose logs (default false)

## liqoctl create nonce

Create a nonce

### Synopsis

Create a Nonce.

The Nonce secret is used to authenticate the remote cluster to the local cluster.



```
liqoctl create nonce [flags]
```

### Examples


```bash
  $ liqoctl create nonce --remote-cluster-id remote-cluster-id
```


### Options
`-o`, `--output` _string_:

>Output the resulting Nonce secret, with no additional output. Supported formats: json, yaml

`--remote-cluster-id` _clusterID_:

>The cluster ID of the remote cluster


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

`--liqo-namespace` _string_:

>The namespace where Liqo is installed in **(default "liqo")**

`-n`, `--namespace` _string_:

>The namespace scope for this request

`--skip-confirm`

>Skip the confirmation prompt (suggested for automation)

`--user` _string_:

>The name of the kubeconfig user to use

`-v`, `--verbose`

>Enable verbose logs (default false)

## liqoctl create publickey

Create a Public Key

### Synopsis

Create a PublicKey.

The PublicKey resource is used to define a PublicKey for the external network.



```
liqoctl create publickey [flags]
```

### Examples


```bash
  $ liqoctl create publickey my-public-key --remote-cluster-id remote-cluster-id --type server --gateway-name my-gateway
```


### Options
`-o`, `--output` _string_:

>Output the resulting PublicKey resource, instead of applying it. Supported formats: json, yaml

`--public-key` _bytesBase64_:

>The public key to be used for the Gateway

`--remote-cluster-id` _clusterID_:

>The cluster ID of the remote cluster


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

`--liqo-namespace` _string_:

>The namespace where Liqo is installed in **(default "liqo")**

`-n`, `--namespace` _string_:

>The namespace scope for this request

`--skip-confirm`

>Skip the confirmation prompt (suggested for automation)

`--user` _string_:

>The name of the kubeconfig user to use

`-v`, `--verbose`

>Enable verbose logs (default false)

## liqoctl create resourceslice

Create a ResourceSlice

### Synopsis

Create a ResourceSlice.

The ResourceSlice resource is used to represent a slice of resources that can be shared across clusters.



```
liqoctl create resourceslice [flags]
```

### Examples


```bash
  $ liqoctl create resourceslice my-slice --remote-cluster-id remote-cluster-id \
  --cpu 4 --memory 8Gi --pods 30
  $ liqoctl create resourceslice my-slice --remote-cluster-id remote-cluster-id \
  --cpu 4 --memory 8Gi --pods 30 --resource nvidia.com/gpu=2
```


### Options
`--class` _string_:

>The class of the ResourceSlice **(default "default")**

`--cpu` _string_:

>The amount of CPU requested in the resource slice

`--memory` _string_:

>The amount of memory requested in the resource slice

`--no-virtual-node`

>Prevent the automatic creation of a VirtualNode for the ResourceSlice. Default: false

`-o`, `--output` _string_:

>Output the resulting ResourceSlice resource, instead of applying it. Supported formats: json, yaml

`--pods` _string_:

>The amount of pods requested in the resource slice

`--remote-cluster-id` _clusterID_:

>The cluster ID of the remote cluster

`--resource` _stringToString_:

>Other resources requested in the resource slice (e.g., 'resource=nvidia.com/gpu=2')


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

`--liqo-namespace` _string_:

>The namespace where Liqo is installed in **(default "liqo")**

`-n`, `--namespace` _string_:

>The namespace scope for this request

`--skip-confirm`

>Skip the confirmation prompt (suggested for automation)

`--user` _string_:

>The name of the kubeconfig user to use

`-v`, `--verbose`

>Enable verbose logs (default false)

## liqoctl create virtualnode

Create a virtual node

### Synopsis

Create a VirtualNode.

The VirtualNode resource is used to represent a remote cluster in the local cluster.



```
liqoctl create virtualnode [flags]
```

### Examples


```bash
  $ liqoctl create virtualnode my-cluster --cluster-id remote-cluster-id \
  --kubeconfig-secret-name my-cluster-kubeconfig
```

  Or, if creating a VirtualNode from a ResourceSlice:

```bash
  $ liqoctl create virtualnode my-cluster --cluster-id remote-cluster-id \
  --resource-slice-name my-resourceslice
```


### Options
`--cpu` _string_:

>The amount of CPU available in the virtual node **(default "2")**

`--create-node`

>Create a node to target the remote cluster (and schedule on it) **(default true)**

`--disable-network-check`

>Disable the network status check

`--ingress-classes` _strings_:

>The ingress classes offered by the remote cluster. The first one will be used as default

`--kubeconfig-secret-name` _string_:

>The name of the secret containing the kubeconfig of the remote cluster. Mutually exclusive with --resource-slice-name

`--labels` _stringToString_:

>The labels to be added to the virtual node

`--load-balancer-classes` _strings_:

>The load balancer classes offered by the remote cluster. The first one will be used as default

`--memory` _string_:

>The amount of memory available in the virtual node **(default "4Gi")**

`--node-selector` _stringToString_:

>The node selector to be applied to offloaded pods

`-o`, `--output` _string_:

>Output the resulting VirtualNode resource, instead of applying it. Supported formats: json, yaml

`--pods` _string_:

>The amount of pods available in the virtual node **(default "110")**

`--remote-cluster-id` _clusterID_:

>The cluster ID of the remote cluster

`--resource` _stringToString_:

>Other resources available in the virtual node (e.g., 'resource=nvidia.com/gpu=2')

`--resource-slice-name` _string_:

>The name of the resourceslice to be used to create the virtual node. Mutually exclusive with --kubeconfig-secret-name

`--runtime-class-name` _string_:

>The runtimeClass the pods should have on the target remote cluster

`--storage-classes` _strings_:

>The storage classes offered by the remote cluster. The first one will be used as default

`--vk-options-template` _string_:

>Namespaced name of the virtual-kubelet options template. Leave empty to use the default template installed with Liqo


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

`--liqo-namespace` _string_:

>The namespace where Liqo is installed in **(default "liqo")**

`-n`, `--namespace` _string_:

>The namespace scope for this request

`--skip-confirm`

>Skip the confirmation prompt (suggested for automation)

`--user` _string_:

>The name of the kubeconfig user to use

`-v`, `--verbose`

>Enable verbose logs (default false)

