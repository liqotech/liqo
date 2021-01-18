package client

import (
	advertisementApi "github.com/liqotech/liqo/apis/sharing/v1alpha1"
)

//createAdvertisementController creates a new CRDController for the Liqo Advertisement CRD.
func createAdvertisementController(kubeconfig string) (*CRDController, error) {
	controller := &CRDController{}
	//init client
	newClient, err := advertisementApi.CreateAdvertisementClient(kubeconfig, nil, false)
	if err != nil {
		return nil, err
	}
	controller.CRDClient = newClient
	controller.resource = string(CRAdvertisement)
	return controller, nil
}
