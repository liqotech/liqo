package controller

import (
	"drone-operator/drone-operator/pkg/controller/dronefederateddeployment"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, dronefederateddeployment.Add)
}
