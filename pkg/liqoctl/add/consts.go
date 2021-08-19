package add

const (
	// LiqoctlAddShortHelp contains the short help string for liqoctl add command.
	LiqoctlAddShortHelp = "Enable a peering with a remote cluster"
	// LiqoctlAddLongHelp contains the Long help string for liqoctl add command.
	LiqoctlAddLongHelp = `Enable a peering with a remote cluster

$ liqoctl add cluster my-cluster --auth-url https://my-cluster --id e8e3cdec-b007-48ad-b2d5-64a8f03dc5f4 --token 525972c1d0a791...

`
	// AuthURLFlagName contains the name of auth-url flag.
	AuthURLFlagName = "auth-url"
	// ClusterNameFlagName contains the name of cluster name flag.
	ClusterNameFlagName = "name"
	// ClusterIDFlagName contains the name of cluster-id flag.
	ClusterIDFlagName = "id"
	// ClusterTokenFlagName contains the name for the token flag.
	ClusterTokenFlagName = "token"
	// UseCommand contains the verb of the add command.
	UseCommand = "add"
	// ClusterResourceName contains the name of the resource added in liqoctl add.
	ClusterResourceName = "cluster"
	// ClusterLiqoNamespaceFlagName contains the default namespace where Liqo is installed.
	ClusterLiqoNamespaceFlagName = "namespace"
	// ClusterLiqoNamespace contains the default namespace where Liqo is installed.
	ClusterLiqoNamespace = "liqo"
	sameClusterError     = "the ClusterID of the adding cluster is the same of the local cluster"
)
