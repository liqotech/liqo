# Compatibility Matrix

This page provides information about Liqo's compatibility with different Kubernetes providers.

```{admonition} Note
While the following list includes providers that we have specifically tested, Liqo should work with any Kubernetes-compliant distribution, although some provider-specific configurations might be required.
```

## Legend

- ✅ Fully supported - All features work as expected
- 🟢 Mostly supported - Core features work well, with only minor limitations in specific scenarios
- 🔵 Partial support - Some Liqo features are functional, but others may require specific configurations or have major limitations in certain use cases

## Tested Provider Compatibility Table

| Provider | Status | Known Issues |
|----------|--------|--------------|
| Kubeadm (Calico) | ✅ | No known issues |
| Kubeadm (Cilium) | ✅ | No known issues |
| Kubeadm (Cilium with kube-proxy replacement) | 🟢 | `NodePortExposition` and `LoadBalancerExposition` |
| K3s | 🟢 | `RemoteExec` |
| K0s | ✅ | No known issues |
| AKS (Azure CNI Overlay) | 🟢 | `CrossClusterAPIServerInteraction` and `ExternalIPRemapping` |
| AKS (Azure CNI (Legacy)) | 🟢 | `CrossClusterAPIServerInteraction` and `NodePortExposition` |
| AKS (Kubenet) | 🟢 | `CrossClusterAPIServerInteraction` and `ExternalIPRemapping` |
| EKS | 🟢 | `CrossClusterAPIServerInteraction` and `ExternalIPRemapping` |
| GKE (Dataplane v1) | ✅ | No known issues |
| GKE (Dataplane v2) | 🟢 | `NodePortExposition` and `LoadBalancerExposition` |
| Aruba Cloud KaaS | ✅ | No known issues |
| OpenShift | 🔵 | **Work in progress**: all Liqo functionalities except for the networking module work as expected. The team is actively working on adding full networking support. You can still use it by [disabling the Network Module](AdvancedUseOnlyOffloadingDisableModule). |
| *Your K8s Distribution* | 🟢 | Liqo is designed to work with most Kubernetes-compliant distributions. Your provider is likely supported, but may require specific configurations. |

### Help Us Improve

Have you tested Liqo with a provider not listed here?
We'd love to hear about your experience!
Join our [Slack community](https://liqo-io.slack.com/join/shared_invite/zt-h20212gg-g24YvN6MKiD9bacFeqZttQ) and share your test results.
Your feedback helps us improve Liqo's compatibility across different environments.

## Issues Reference

The following issues are known to affect the compatibility of Liqo with different Kubernetes providers:

- `CrossClusterAPIServerInteraction`: It affects the capability of offloaded pods to properly interact with the Kubernetes API server of the consumer cluster. This ensures that applications running in provider clusters can still access and manipulate Kubernetes resources (such as ConfigMaps, Secrets, or other custom resources) in their original cluster. Limitations in this area may impact applications that rely on the Kubernetes API for normal operation. See [here](../advanced/kubernetes-api.md) for more details.

- `RemoteExec`: It affects the capability to execute commands in pods that have been offloaded to remote clusters, using `kubectl exec`. When limited, administrators may face challenges in directly interacting with offloaded workloads.

- `NodePortExposition`: It affects the capability to access remote offloaded pods through NodePort services. This capability allows network traffic addressed to NodePort services to reach every pods, even when those pods are running in remote clusters. Limitations here may affect external access to applications.

- `LoadBalancerExposition`: It affects the capability to expose remote offloaded pods via LoadBalancer services. This capability allows cloud provider load balancers to properly distribute traffic to offloaded pods. When this feature has limitations, it may impact the ability to use cloud load balancers with offloaded workloads. It is worth noting that this limitation does not affect a Load Balancer service associated to an Ingress controller; in this case, the external traffic requests are properly terminated to the proper serving pods (either local or remote).

- `ExternalIPRemapping`: It affects the capability to make external IPs (outside of the Kubernetes cluster network) accessible to pods running in remote and local clusters. This capability involves translating IP addresses between clusters with potentially overlapping network ranges, ensuring that pods in one cluster can access external resources that are only directly reachable from another cluster. Limitations here may affect connectivity to external services or resources. See [here](../advanced/external-ip-remapping.md) for more details.
