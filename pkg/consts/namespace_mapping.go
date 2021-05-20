package consts

const (
	// RemoteClusterID is used to obtain cluster-id from different Liqo resources.
	RemoteClusterID = "cluster-id" // "remote.liqo.io/clusterId"
	// MapNamespaceName is the namespace where NamespaceMap are created.
	MapNamespaceName = "default"
	// TypeLabel is the key of a Liqo label that identifies different types of nodes.
	TypeLabel = "liqo.io/type"
	// TypeNode is the value of a Liqo label that identifies Liqo virtual nodes.
	TypeNode = "virtual-node"
	// NamespaceMapControllerFinalizer is the finalizer inserted on NamespaceMap by NamespaceMap Controller.
	// todo: has to be removed after VirtualNode Controller refactor
	NamespaceMapControllerFinalizer = "namespacemap-controller.liqo.io/finalizer"
	// DocumentationURL is the URL to official Liqo Documentation.
	DocumentationURL = "https://doc.liqo.io/"
	// DefaultNamespaceOffloadingName is the default name of NamespaceOffloading resources. Every namespace that has
	// to be offloaded with Liqo, must have a NamespaceOffloading resource with this name.
	DefaultNamespaceOffloadingName = "offloading"
	// EnablingLiqoLabel is necessary in order to allow Pods to be scheduled on remote clusters.
	EnablingLiqoLabel = "liqo.io/enabled"
	// EnablingLiqoLabelValue unique value allowed for EnablingLiqoLabel.
	EnablingLiqoLabelValue = "true"
)
