package consts

// NodeFinalizer is the finalizer added on a ResourceOffer when the related VirtualNode is up.
// (managed by the VirtualKubelet).
const NodeFinalizer = "liqo.io/node"

// VirtualKubeletFinalizer is the finalizer added on a ResourceOffer when the related VirtualKubelet is up.
// (managed by the ResourceOffer Operator).
const VirtualKubeletFinalizer = "liqo.io/virtualkubelet"
