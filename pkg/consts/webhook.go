package consts

const (
	// VirtualNodeTolerationKey all Pods that have to be scheduled on virtual nodes must have this toleration
	// to Liqo taint.
	VirtualNodeTolerationKey = "virtual-node.liqo.io/not-allowed"
)
