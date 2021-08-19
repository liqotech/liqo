package consts

const (
	// ClusterIDConfigMapName is the name of the configmap where the cluster-id is stored.
	ClusterIDConfigMapName = "cluster-id"
	// MasterLabel contains the label used to identify the master nodes.
	MasterLabel = "node-role.kubernetes.io/master"
	// ServiceAccountNamespacePath contains the path where the namespace is stored in the serviceaccount volume mount.
	ServiceAccountNamespacePath = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
	// ClusterIDLabelName is the name of the label key to use with Cluster ID.
	ClusterIDLabelName = "clusterID"
	// ClusterIDConfigMapKey is the key of the configmap where the cluster-id is stored.
	ClusterIDConfigMapKey = "cluster-id"
	// ClusterConfigResourceName contains the name of the default ClusterConfig object.
	ClusterConfigResourceName = "liqo-configuration"
)
