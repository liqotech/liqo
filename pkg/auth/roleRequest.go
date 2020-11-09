package auth

type RoleRequest struct {
	ClusterID string `json:"clusterID"`
	Token     string `json:"token"`
}
