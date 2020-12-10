package auth

type ClusterInfo struct {
	ClusterID      string `json:"clusterId"`
	ClusterName    string `json:"clusterName,omitempty"`
	GuestNamespace string `json:"guestNamespace"`
}
