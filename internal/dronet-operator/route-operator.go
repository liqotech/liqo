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

// RouteController reconciles a TunnelEndpoint object
type RouteController struct {
	client.Client
	Log           logr.Logger
	Scheme        *runtime.Scheme
	clientset     kubernetes.Clientset
	RouteOperator bool
	ClientSet     *kubernetes.Clientset
	RoutesMap     map[string]netlink.Route
}

// +kubebuilder:rbac:groups=dronet.drone.com,resources=tunnelendpoints,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=dronet.drone.com,resources=tunnelendpoints/status,verbs=get;update;patch

func (r *RouteController) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("endpoint", req.NamespacedName)
	var endpoint v1.TunnelEndpoint
	var route netlink.Route

	if err := r.Get(ctx, req.NamespacedName, &endpoint); err != nil {
		log.Error(err, "unable to fetch endpoint")
		if client.IgnoreNotFound(err) == nil {
			val, found := r.RoutesMap[req.NamespacedName.String()]
			if !found {
				return ctrl.Result{}, client.IgnoreNotFound(err)
			}
			err = dronet_operator.DelRoute(val)
			if err != nil {
				log.Error(err, "unable to delete route "+r.RoutesMap[req.NamespacedName.String()].String())
			} else {
				log.Info("deleted the route: " + r.RoutesMap[req.NamespacedName.String()].String())
				delete(r.RoutesMap, req.NamespacedName.String())
			}
		}
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	dst, gw, err := dronet_operator.ValidateCRAndReturn(&endpoint)
	if err != nil {
		log.Error(err, "unable to validate the endpoint custom resource")
		return ctrl.Result{}, nil
	}
	linkIndex, _, err := dronet_operator.GetGateway()
	if err != nil {
		log.Error(err, "unable to get the link index in route-operator")
	}
	val, found := r.RoutesMap[req.NamespacedName.String()]

	if !found {
		route, err = dronet_operator.AddRoute(dst, gw, linkIndex)
		if err != nil {
			log.Error(err, "unable to instantiate the route")
			return ctrl.Result{}, nil
		}
		r.RoutesMap[req.NamespacedName.String()] = route
	}
	//check if the route is the same
	if found {
		//if yes, do nothing
		if val.Dst.String() == dst.String() && val.Gw.String() == gw.String() {
			log.Info("route controller : route already existes for endpoint" + req.NamespacedName.String())
			_, found := r.RoutesMap[req.NamespacedName.String()]
			if !found {
				r.RoutesMap[req.NamespacedName.String()] = val
			}
			return ctrl.Result{}, nil
		} else {
			//if no, remove the old one and install the new one
			dronet_operator.DelRoute(val)
			route, err := dronet_operator.AddRoute(dst, gw, linkIndex)
			if err != nil {
				log.Error(err, "unable to instantiate the route")
				return ctrl.Result{}, nil
				r.RoutesMap[req.NamespacedName.String()] = route
			}
		}

	}
	return ctrl.Result{}, nil
}

func (r *RouteController) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.TunnelEndpoint{}).
		Complete(r)
}
