package auth

// ClusterInfo contains the information to be shared to a remote cluster to make the peering possible.
type ClusterInfo struct {
	ClusterID   string `json:"clusterId"`
	ClusterName string `json:"clusterName,omitempty"`
}
