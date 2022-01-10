// Copyright 2019-2022 The Liqo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package routeoperator

import (
	"context"
	"os"
	"os/signal"
	"strings"

	"github.com/vishvananda/netlink"
	k8sApiErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/liqonet/overlay"
	liqorouting "github.com/liqotech/liqo/pkg/liqonet/routing"
	"github.com/liqotech/liqo/pkg/liqonet/utils"
)

var (
	result = ctrl.Result{}
)

// RouteController reconciles a TunnelEndpoint object.
type RouteController struct {
	client.Client
	record.EventRecorder
	liqorouting.Routing
	vxlanDev *overlay.VxlanDevice
	podIP    string
}

// NewRouteController returns a configured route controller ready to be started.
func NewRouteController(podIP string, vxlanDevice *overlay.VxlanDevice, router liqorouting.Routing, er record.EventRecorder,
	cl client.Client) *RouteController {
	r := &RouteController{
		Client:        cl,
		Routing:       router,
		vxlanDev:      vxlanDevice,
		EventRecorder: er,
		podIP:         podIP,
	}
	return r
}

// cluster-role
// +kubebuilder:rbac:groups=net.liqo.io,resources=tunnelendpoints,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=net.liqo.io,resources=tunnelendpoints/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=core,resources=nodes,verbs=get
// role
// +kubebuilder:rbac:groups=core,namespace="do-not-care",resources=secrets,verbs=create;update;patch;get;list;watch;delete
// +kubebuilder:rbac:groups=core,resources=pods,verbs=update;patch;get;list;watch
// +kubebuilder:rbac:groups=core,namespace="do-not-care",resources=services,verbs=update;patch;get;list;watch

// Reconcile handle requests on TunnelEndpoint object to create and configure routes on Nodes.
func (rc *RouteController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	tep := new(netv1alpha1.TunnelEndpoint)
	var err error
	// Name of our finalizer set on every tep instance processed by the operator.
	routeOperatorFinalizer := strings.Join([]string{liqoconst.LiqoRouteOperatorName, rc.podIP, "net.liqo.io"}, ".")
	if err = rc.Get(ctx, req.NamespacedName, tep); err != nil && !k8sApiErrors.IsNotFound(err) {
		klog.Errorf("unable to fetch resource {%s} :%v", req.String(), err)
		return result, err
	}
	// In case the resource does not exist anymore, we just forget it.
	if k8sApiErrors.IsNotFound(err) {
		return result, nil
	}
	// Here we check that the tunnelEndpoint resource has been fully processed. If not we do nothing.
	if tep.Status.GatewayIP == "" {
		return result, nil
	}
	clusterID := tep.Spec.ClusterID
	_, remotePodCIDR := utils.GetPodCIDRS(tep)
	_, remoteExternalCIDR := utils.GetExternalCIDRS(tep)
	// Examine DeletionTimestamp to determine if object is under deletion.
	if tep.ObjectMeta.DeletionTimestamp.IsZero() {
		if !controllerutil.ContainsFinalizer(tep, routeOperatorFinalizer) {
			// The object is not being deleted, so if it does not have our finalizer,
			// then lets add the finalizer and update the object. This is equivalent
			// registering our finalizer.
			controllerutil.AddFinalizer(tep, routeOperatorFinalizer)
			if err := rc.Update(ctx, tep); err != nil {
				if k8sApiErrors.IsConflict(err) {
					klog.V(4).Infof("%s -> unable to add finalizers to resource {%s}: %s", clusterID, req.String(), err)
					return result, err
				}
				klog.Errorf("%s -> unable to add finalizers to resource {%s}: %s", clusterID, req.String(), err)
				return result, err
			}
		}
	} else {
		// The object is being deleted, if we encounter an error while removing the routes than we record an
		// event on the resource to notify the user. The finalizer is not removed.
		if controllerutil.ContainsFinalizer(tep, routeOperatorFinalizer) {
			klog.Infof("resource {%s} of type {%s} is being removed", tep.Name, tep.GroupVersionKind().String())
			deleted, err := rc.RemoveRoutesPerCluster(tep)
			if err != nil {
				klog.Errorf("%s -> unable to remove route for destinations {%s} and {%s}: %s",
					clusterID, remotePodCIDR, remoteExternalCIDR, err)
				rc.Eventf(tep, "Warning", "Processing", "unable to remove route: %s", err.Error())
				return result, err
			}
			if deleted {
				klog.Infof("%s -> route for destinations {%s} and {%s} correctly removed",
					clusterID, remotePodCIDR, remoteExternalCIDR)
				rc.Eventf(tep, "Normal", "Processing", "route for destination {%s} and {%s} correctly removed",
					remotePodCIDR, remoteExternalCIDR)
			}
			// remove the finalizer from the list and update it.
			controllerutil.RemoveFinalizer(tep, routeOperatorFinalizer)
			if err := rc.Update(ctx, tep); err != nil {
				if k8sApiErrors.IsConflict(err) {
					klog.V(4).Infof("%s -> unable to add finalizers to resource {%s}: %s", clusterID, req.String(), err)
					return result, err
				}
				klog.Errorf("%s -> unable to remove finalizers from resource {%s}: %s", clusterID, req.String(), err)
				return result, err
			}
		}
		return result, nil
	}
	added, err := rc.EnsureRoutesPerCluster(tep)
	if err != nil {
		klog.Errorf("%s -> unable to configure route for destinations {%s} and {%s}: %s",
			clusterID, remotePodCIDR, remoteExternalCIDR, err)
		rc.Eventf(tep, "Warning", "Processing", "unable to configure route for destinations {%s} and {%s}: %s",
			remotePodCIDR, remoteExternalCIDR, err.Error())
		return result, err
	}
	if added {
		klog.Infof("%s -> route for destinations {%s} and {%s} correctly configured", clusterID, remotePodCIDR, remoteExternalCIDR)
		rc.Eventf(tep, "Normal", "Processing", "route for destinations {%s} and {%s} configured", remotePodCIDR, remoteExternalCIDR)
	}
	return result, nil
}

// cleanUp removes all the routes, rules and devices (if any) from the
// node inserted by the operator. It is called at exit time.
func (rc *RouteController) cleanUp() {
	if rc.Routing != nil {
		if err := rc.Routing.CleanRoutingTable(); err != nil {
			klog.Errorf("un error occurred while cleaning up routes: %v", err)
		}
		if err := rc.Routing.CleanPolicyRules(); err != nil {
			klog.Errorf("un error occurred while cleaning up policy routing rules: %v", err)
		}
	}
	if rc.vxlanDev != nil {
		err := netlink.LinkDel(rc.vxlanDev.Link)
		if err != nil && err.Error() != "Link not found" {
			klog.Errorf("an error occurred while deleting vxlan device {%s}: %v", rc.vxlanDev.Link.Name, err)
		}
	}
}

// SetupWithManager used to set up the controller with a given manager.
func (rc *RouteController) SetupWithManager(mgr ctrl.Manager) error {
	resourceToBeProccesedPredicate := predicate.Funcs{
		DeleteFunc: func(e event.DeleteEvent) bool {
			// Finalizers are used to check if a resource is being deleted, and perform there the needed actions
			// we don't want to reconcile on the delete of a resource.
			return false
		},
	}
	return ctrl.NewControllerManagedBy(mgr).WithEventFilter(resourceToBeProccesedPredicate).
		For(&netv1alpha1.TunnelEndpoint{}).
		Complete(rc)
}

// SetupSignalHandlerForRouteOperator registers for SIGTERM, SIGINT. Interrupt. A stop context is returned
// which is closed on one of these signals.
func (rc *RouteController) SetupSignalHandlerForRouteOperator() context.Context {
	ctx, done := context.WithCancel(context.Background())
	c := make(chan os.Signal, 1)
	signal.Notify(c, utils.ShutdownSignals...)
	go func(r *RouteController) {
		sig := <-c
		klog.Infof("the operator received signal {%s}: cleaning up", sig.String())
		r.cleanUp()
		done()
	}(rc)
	return ctx
}
