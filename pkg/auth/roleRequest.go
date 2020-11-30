package auth

type IdentityRequest struct {
	ClusterID string `json:"clusterID"`
	Token     string `json:"token"`
}
