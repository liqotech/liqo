# Virtual-Kubelet network configuration via controller-injected flags

## Summary

Stop the virtual-kubelet from querying the Kubernetes API server for `networkingv1beta1.Configuration`. Instead, the `liqo-controller-manager` fetches the network CIDR remappings and injects them as virtual-kubelet container arguments. When the `Configuration` changes, the VirtualNode controller updates the VK Deployment so a new virtual-kubelet pod is rolled out with the correct CIDRs.

## Motivation

Today, every virtual-kubelet pod needs direct read access to the `networking.liqo.io/configurations` resource to discover how remote pod and external CIDRs are remapped. Centralising this lookup in the VirtualNode controller has two benefits:

1. It reduces the RBAC surface of the virtual-kubelet.
2. It guarantees that a change in the network `Configuration` is propagated to running virtual-kubelets by re-forging the Deployment.

## Scope

- Limited to the `networkingv1beta1.Configuration` lookup in the virtual-kubelet.
- `ForeignCluster` and `VirtualNode` lookups are left unchanged.
- No new API types or CRD changes.
- Network CIDR flags are injected by the controller, **not** by the VirtualNode mutating webhook.

## Changes

### Virtual-kubelet flags and internal representation

- Added four new VK flags:
  - `--remote-pod-cidr`
  - `--remote-pod-cidr-remap`
  - `--remote-external-cidr`
  - `--remote-external-cidr-remap`
- Added `buildRemoteCIDR` to assemble a new internal `*networkconfig.RemoteCIDR` value from the flags.
- Updated the provider and pod reflectors to use the internal `RemoteCIDR` struct instead of `*networkingv1beta1.Configuration`.

### Controller-side arg forging

- Added `vkforge.SetNetworkConfigurationArgs(..., cfg)` to append the four network CIDR flag groups to a VK container.
- The VirtualNode controller first fetches the `ForeignCluster` for the VirtualNode's cluster ID.
  - If the `ForeignCluster` is missing or its `Status.Modules.Networking.Enabled` is `false`, the controller skips injecting network args.
  - Only when networking is enabled does it fetch the `Configuration` by cluster ID and pass it to the forge helper.
- If the `Configuration` is not found, no network args are injected.
- Added a watch on `Configuration` and `ForeignCluster` to re-enqueue affected `VirtualNodes` when the network configuration changes.

### Webhook cleanup

- Reverted the earlier network-arg injection from the VirtualNode webhook.
- Removed the `configurations` RBAC permission from the webhook ClusterRole.

### Tests

- Updated `cmd/virtual-kubelet/root/networkconfig_test.go` for the new flag builder.
- Added `pkg/vkMachinery/forge/networkconfig_test.go` covering arg forging.
- Updated the VirtualNode controller integration tests to create ready `Configuration` objects.
- Updated `pkg/virtualKubelet/reflection/workload/pod_test.go` and `podns_test.go` to use `*networkconfig.RemoteCIDR` and the new `func() []string` Kubernetes service IP getter.

## Verification

All relevant packages build and pass tests:

```bash
go test ./cmd/virtual-kubelet/root/... ./pkg/vkMachinery/forge/... ./pkg/utils/ipam/mapping/...
KUBEBUILDER_ASSETS=/tmp/envtest-binaries/k8s/1.36.2-darwin-arm64 go test ./pkg/liqo-controller-manager/offloading/virtualnode-controller/...
go test ./pkg/virtualKubelet/reflection/workload/...
go build ./cmd/virtual-kubelet
GOOS=linux go build ./cmd/webhook
GOOS=linux go build ./cmd/liqo-controller-manager
```

## Checklist

- [x] Code compiles and tests pass.
- [x] Manifests/RBACs regenerated (`make manifests`, `make rbacs`).
- [x] Scope limited to the requested change.
- [x] No new API types or CRD changes.
