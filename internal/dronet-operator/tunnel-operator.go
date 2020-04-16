/*

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package controllers

import (
	"context"
	"github.com/go-logr/logr"
	"github.com/netgroup-polito/dronev2/api/tunnel-endpoint/v1"
	dronet_operator "github.com/netgroup-polito/dronev2/pkg/dronet-operator"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TunnelController reconciles a TunnelEndpoint object
type TunnelController struct {
	client.Client
	Log           logr.Logger
	Scheme        *runtime.Scheme
	RouteOperator bool
}

// +kubebuilder:rbac:groups=dronet.drone.com,resources=tunnelendpoints,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=dronet.drone.com,resources=tunnelendpoints/status,verbs=get;update;patch

func (r *TunnelController) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("endpoint", req.NamespacedName)
	var endpoint v1.TunnelEndpoint
	//name of our finalizer
	tunnelEndpointFinalizer := "tunnelEndpointFinalizer.dronet.drone.com"

	if err := r.Get(ctx, req.NamespacedName, &endpoint); err != nil {
		log.Error(err, "unable to fetch endpoint, probably it has been deleted")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	// examine DeletionTimestamp to determine if object is under deletion
	if endpoint.ObjectMeta.DeletionTimestamp.IsZero() {
		if !containsString(endpoint.ObjectMeta.Finalizers, tunnelEndpointFinalizer) {
			// The object is not being deleted, so if it does not have our finalizer,
			// then lets add the finalizer and update the object. This is equivalent
			// registering our finalizer.
			endpoint.ObjectMeta.Finalizers = append(endpoint.Finalizers, tunnelEndpointFinalizer)
			if err := r.Update(ctx, &endpoint); err != nil {
				log.Error(err, "unable to update endpoint")
				return ctrl.Result{}, err
			}
		}
	} else {
		//the object is being deleted
		if containsString(endpoint.Finalizers, tunnelEndpointFinalizer) {
			if err := dronet_operator.RemoveGreTunnel(&endpoint); err != nil {
				return ctrl.Result{}, err
			}
			log.Info("tunnel iface removed")
			//remove the finalizer from the list and update it.
			endpoint.Finalizers = removeString(endpoint.Finalizers, tunnelEndpointFinalizer)
			if err := r.Update(ctx, &endpoint); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	//update the status of the endpoint custom resource
	//and install the tunnel only
	//check if the CR is newly created
	if endpoint.Status.TunnelIFaceIndex == 0 && endpoint.Status.TunnelIFaceName == "" && endpoint.Status.LocalTunnelPrivateIP == "" && endpoint.Status.LocalTunnelPublicIP == "" {
		iFaceIndex, iFaceName, err := dronet_operator.InstallGreTunnel(&endpoint)
		if err != nil {
			log.Error(err, "unable to create the gre tunnel")
			return ctrl.Result{}, err
		}
		//update the status of CR
		localTunnelPublicIP, err := dronet_operator.GetLocalTunnelPublicIPToString()
		if err != nil {
			log.Error(err, "unable to get localTunnelPublicIP")
		}
		localTunnelPrivateIP, err := dronet_operator.GetLocalTunnelPrivateIPToString()
		if err != nil {
			log.Error(err, "unable to get localTunnelPrivateIP")
		}
		endpoint.Status.TunnelIFaceName = iFaceName
		endpoint.Status.TunnelIFaceIndex = iFaceIndex
		endpoint.Status.LocalTunnelPrivateIP = localTunnelPrivateIP
		endpoint.Status.LocalTunnelPublicIP = localTunnelPublicIP
		err = r.Client.Status().Update(ctx, &endpoint)
		if err != nil {
			log.Error(err, "unable to update status field: tunnelIfaceIndex")
			//if the operator fails to update the status then we also remove the tunnel
			if err = dronet_operator.DeleteIFaceByIndex(iFaceIndex); err !=nil{
				log.Error(err, "unable to remove the tunnel interface")
			}
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *TunnelController) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.TunnelEndpoint{}).
		Complete(r)
}

// Helper functions to check and remove string from a slice of strings.
func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func removeString(slice []string, s string) (result []string) {
	for _, item := range slice {
		if item == s {
			continue
		}
		result = append(result, item)
	}
	return
}
