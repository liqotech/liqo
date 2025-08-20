# liqoctl generate

Generate Liqo resources

## Description

### Synopsis

Generate Liqo resources.


## liqoctl generate configuration

Generate a Configuration

### Synopsis

Generate the local network configuration to be applied to other clusters.


```
liqoctl generate configuration [flags]
```

### Options
`-o`, `--output` _string_:

>Output format of the resulting Configuration resource. Supported formats: json, yaml **(default "yaml")**


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

## liqoctl generate identity

Generate a Identity

### Synopsis

Generate the Identity resource to be applied on the remote consumer cluster.

The Identity is generated from the Tenant associated with the provided remote clusterID.
It is intended to be applied on the remote consumer cluster.
This command generates only Identities used by the Liqo control plane for authentication purposes (e.g., CRDReplicator).



```
liqoctl generate identity [flags]
```

### Examples


```bash
  $ liqoctl generate identity --remote-cluster-id remote-cluster-id
```


### Options
`-o`, `--output` _string_:

>Output format of the resulting Identity resource. Supported formats: json, yaml **(default "yaml")**

`--remote-cluster-id` _clusterID_:

>The ID of the remote cluster

`--remote-tenant-namespace` _string_:

>The remote tenant namespace where the Identity will be applied, if not sure about the value, you can omit this flag it when the manifest is applied


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

## liqoctl generate peering-user

Generate a new user with the permissions to peer with this cluster

### Synopsis

Generate a new user with the permissions to peer with this cluster.

This command generates a user with the minimum permissions to peer with this cluster, from the cluster with
the given cluster ID, and returns a kubeconfig to be used to create or destroy the peering.



```
liqoctl generate peering-user [flags]
```

### Examples


```bash
  $ liqoctl generate peering-user --consumer-cluster-id=<cluster-id>
```


### Options
`--consumer-cluster-id` _clusterID_:

>The cluster ID of the cluster from which peering will be performed

`--tls-compatibility-mode` _string_:

>TLS compatibility mode for peering-user keys: one of auto,true,false. If set to true keys are generated with a widely supported algorithm (RSA) to ensure compatibility with systems that do not yet support Ed25519 (default) as signature algorithm. When auto, liqoctl attempts to detect the system configuration. **(default "auto")**


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

## liqoctl generate publickey

Generate a Public Key

### Synopsis

Generate the PublicKey of a Gateway Server or Client to be applied to other clusters.


```
liqoctl generate publickey [flags]
```

### Options
`--gateway-name` _string_:

>The name of the gateway (server or client) to pull the PublicKey from

`--gateway-type` _string_:

>The type of gateway resource. Allowed values: [server client]

`-o`, `--output` _string_:

>Output format of the resulting PublicKey resource. Supported formats: json, yaml **(default "yaml")**


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

## liqoctl generate tenant

Generate a Tenant

### Synopsis

Generate the Tenant resource to be applied on the remote provider cluster.

This commands generates a Tenant filled with all the authentication parameters needed to authenticate with the remote cluster.
It signs the nonce provided by the remote cluster and generates the CSR.
The Nonce can be provided as a flag or it can be retrieved from the secret in the tenant namespace (if existing).



```
liqoctl generate tenant [flags]
```

### Examples


```bash
  $ liqoctl generate tenant --remote-cluster-id remote-cluster-id
```


### Options
`--nonce` _string_:

>The nonce to sign for the authentication with the remote cluster

`-o`, `--output` _string_:

>Output format of the resulting Tenant resource. Supported formats: json, yaml **(default "yaml")**

`--proxy-url` _string_:

>The URL of the proxy to use for the communication with the remote cluster

`--remote-cluster-id` _clusterID_:

>The ID of the remote cluster

`--remote-tenant-namespace` _string_:

>The namespace on the remote cluster where the Tenant will be applied, if not sure about the value, you can omit this flag and define it when the manifest is applied


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

