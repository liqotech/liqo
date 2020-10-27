package client

import (
	clusterConfig "github.com/liqotech/liqo/apis/config/v1alpha1"
)

//createClusterConfigController creates a new CRDController for the Liqo ClusterConfig CRD.
func createClusterConfigController(kubeconfig string) (*CRDController, error) {
	controller := &CRDController{}
	//init client
	newClient, err := clusterConfig.CreateClusterConfigClient(kubeconfig, false)
	if err != nil {
		return nil, err
	}
	controller.CRDClient = newClient
	controller.resource = string(CRClusterConfig)
	return controller, nil
}
