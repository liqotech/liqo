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
	NamespaceMapControllerFinalizer = "namespacemap-controller.liqo.io/finalizer"
	// DocumentationURL is the URL to official Liqo Documentation.
	DocumentationURL = "https://doc.liqo.io/"
)
