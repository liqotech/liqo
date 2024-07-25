# Access K8S API from offloaded pods

This section describes the possible configurations for accessing the Kubernetes API from offloaded pods.

## Overview

The offloaded Pods can be configured to access the Kubernetes API of the home cluster (the one they are originating from), or of the foreign cluster (the one they are running on), or to be completely disabled.

This feature can be configured per pod, by setting the `liqo.io/api-server-support: disabled | remote` annotation.
Leave the annotation unset to use the default configuration.

## Default configuration

By default, the offloaded Pods are configured to access the Kubernetes API of the home cluster.
When the Virtual Kubelet offloads a Pod, it injects the required environment variables to access the Kubernetes API of the home cluster, sets the DNS entries to access the Kubernetes API of the home cluster, and mounts the required certificates to access the Kubernetes API of the home cluster.
In this way, the offloaded Pods can access the Kubernetes API of the home cluster using the standard Kubernetes client libraries as if they were running on the home cluster.

### Overriding the default Kubernetes API server

In particular scenarios, it may be necessary to override the default Kubernetes API server.
By default, Liqo will make it available through the `liqo-proxy` deployment leveraging the cluster network interconnection.
You can override the default Kubernetes API server by setting the `--home-api-server-host=<your API server host>` and/or `--home-api-server-port=<your API server port>` as extra arguments to the Virtual Kubelet deployments.

## Accessing the Kubernetes API of the foreign cluster

The offloaded Pods can be configured to access the Kubernetes API of the foreign cluster.
When the Virtual Kubelet offloads a Pod, the mounted ServiceAccount will not be mutated, and the offloaded Pods will be able to access the Kubernetes API of the foreign cluster using the standard Kubernetes client libraries as native Pods running on the foreign cluster.

By default, the offloaded Pods will mount a ServiceAccount with the same name as the ServiceAccount set in the `serviceAccountName` field of the PodSpec in the home cluster.
If the ServiceAccount does not exist in the foreign cluster, the offloaded Pods will remain in the `Pending` state.

The offloaded Pods can be configured to mount a different ServiceAccount by adding the `liqo.io/remote-service-account-name: <your service account name>` annotation to the home cluster Pod.
