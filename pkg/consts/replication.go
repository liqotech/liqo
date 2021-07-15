package consts

// OwnershipType indicates the type of ownership over a resource.
type OwnershipType string

const (
	// OwnershipLocal indicates that the resource is owned by the local cluster.
	OwnershipLocal OwnershipType = "Local"
	// OwnershipShared indicates that the ownership over the resource is shared between the two clusters.
	// In particular:
	// - the spec of the resource is owned by the local cluster.
	// - the status by the remote cluster.
	OwnershipShared OwnershipType = "Shared"
	// LocalPodLabelKey label key added to all the local pods that have been offloaded/replicated to a remote cluster.
	LocalPodLabelKey = "liqo.io/shadowPod"
	// LocalPodLabelValue value of the label added to the local pods that have been offloaded/replicated to a remote cluster.
	LocalPodLabelValue = "true"
)
