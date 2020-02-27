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
	"github.com/vishvananda/netlink"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TunnelController reconciles a TunnelEndpoint object
type TunnelController struct {
	client.Client
	Log           logr.Logger
	Scheme        *runtime.Scheme
	clientset     kubernetes.Clientset
	RouteOperator bool
	ClientSet     *kubernetes.Clientset
	EndpointMap   map[string]int
}

// +kubebuilder:rbac:groups=dronet.drone.com,resources=tunnelendpoints,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=dronet.drone.com,resources=tunnelendpoints/status,verbs=get;update;patch

func (r *TunnelController) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("endpoint", req.NamespacedName)
	var endpoint v1.TunnelEndpoint

		if err := r.Get(ctx, req.NamespacedName, &endpoint); err != nil {
			val, found := r.EndpointMap[req.NamespacedName.String()];
			if client.IgnoreNotFound(err) == nil && found{
				existingIface, err := netlink.LinkByIndex(val)
				if err != nil {
					log.Error(err, "unable to delete the tunnel after the tunnelEndpoint CR has been removet")
				}
				//Remove the existing gre interface
				if err = netlink.LinkDel(existingIface); err != nil {
					log.Error(err, "unable to delete the tunnel after the tunnelEndpoint CR has been removet")
				}

			}

			log.Error(err, "unable to fetch endpoint")
			return ctrl.Result{}, client.IgnoreNotFound(err)

		}
		_, gatewayHostIP, err := dronet_operator.GetGateway()
		if err != nil {
			log.Error(err, "unable to get the gatewayHostIP")
		}

		//update the status of the endpoint custom resource
		//and install the tunnel only
		if(endpoint.Status.EndpointNodeIP != gatewayHostIP.String()){
			endpoint.Status.EndpointNodeIP = gatewayHostIP.String()
			err := r.Client.Status().Update(ctx, &endpoint)
			if err != nil {
				log.Error(err, "unable to update status field: gatewayHostIP")
			}
			ifaceIndex, err := dronet_operator.InstallGreTunnel(&endpoint)
			if err != nil {
				log.Error(err, "unable to create the gre tunnel")
			}
			r.EndpointMap[req.NamespacedName.String()] = ifaceIndex
		}

		return ctrl.Result{}, nil
}

func (r *TunnelController) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.TunnelEndpoint{}).
		Complete(r)
}