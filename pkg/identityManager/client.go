package identitymanager

import "k8s.io/client-go/rest"

// get a rest config from the secret, given the remote clusterid.
func (certManager *certificateIdentityManager) GetConfig(remoteClusterID string, masterUrl string) (*rest.Config, error) {
	// TODO: implementation
	panic("TODO: GetConfig")
}
