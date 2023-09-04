// Copyright 2019-2023 The Liqo Authors
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
	"time"

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
	"github.com/liqotech/liqo/pkg/liqonet/iptables"
	"github.com/liqotech/liqo/pkg/liqonet/overlay"
	liqorouting "github.com/liqotech/liqo/pkg/liqonet/routing"
	liqonetutils "github.com/liqotech/liqo/pkg/liqonet/utils"
)

var (
	result = ctrl.Result{}
)

// RouteController reconciles a TunnelEndpoint object.
type RouteController struct {
	client.Client
	record.EventRecorder
	liqorouting.Routing
	vxlanDev     *overlay.VxlanDevice
	podIP        string
	firewallChan chan bool
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
	clusterIdentity := tep.Spec.ClusterIdentity
	_, remotePodCIDR := liqonetutils.GetPodCIDRS(tep)
	_, remoteExternalCIDR := liqonetutils.GetExternalCIDRS(tep)
	// Examine DeletionTimestamp to determine if object is under deletion.
	if tep.ObjectMeta.DeletionTimestamp.IsZero() {
		if !controllerutil.ContainsFinalizer(tep, liqoconst.LiqoRouteFinalizer(rc.podIP)) {
			// The object is not being deleted, so if it does not have our finalizer,
			// then lets add the finalizer and update the object. This is equivalent
			// registering our finalizer.
			controllerutil.AddFinalizer(tep, liqoconst.LiqoRouteFinalizer(rc.podIP))
			if err := rc.Update(ctx, tep); err != nil {
				if k8sApiErrors.IsConflict(err) {
					klog.V(4).Infof("%s -> unable to add finalizers to resource {%s}: %s", clusterIdentity, req.String(), err)
					return result, err
				}
				klog.Errorf("%s -> unable to add finalizers to resource {%s}: %s", clusterIdentity, req.String(), err)
				return result, err
			}
		}
	} else {
		// The object is being deleted, if we encounter an error while removing the routes than we record an
		// event on the resource to notify the user. The finalizer is not removed.
		if controllerutil.ContainsFinalizer(tep, liqoconst.LiqoRouteFinalizer(rc.podIP)) {
			klog.Infof("resource {%s} of type {%s} is being removed", tep.Name, tep.GroupVersionKind().String())
			deleted, err := rc.RemoveRoutesPerCluster(tep)
			if err != nil {
				klog.Errorf("%s -> unable to remove route for destinations {%s} and {%s}: %s",
					clusterIdentity, remotePodCIDR, remoteExternalCIDR, err)
				rc.Eventf(tep, "Warning", "Processing", "unable to remove route: %s", err.Error())
				return result, err
			}
			if deleted {
				klog.Infof("%s -> route for destinations {%s} and {%s} correctly removed",
					clusterIdentity, remotePodCIDR, remoteExternalCIDR)
				rc.Eventf(tep, "Normal", "Processing", "route for destination {%s} and {%s} correctly removed",
					remotePodCIDR, remoteExternalCIDR)
			}
			// remove the finalizer from the list and update it.
			controllerutil.RemoveFinalizer(tep, liqoconst.LiqoRouteFinalizer(rc.podIP))
			if err := rc.Update(ctx, tep); err != nil {
				if k8sApiErrors.IsConflict(err) {
					klog.V(4).Infof("%s -> unable to add finalizers to resource {%s}: %s", clusterIdentity, req.String(), err)
					return result, err
				}
				klog.Errorf("%s -> unable to remove finalizers from resource {%s}: %s", clusterIdentity, req.String(), err)
				return result, err
			}
		}
		return result, nil
	}
	added, err := rc.EnsureRoutesPerCluster(tep)
	if err != nil {
		klog.Errorf("%s -> unable to configure route for destinations {%s} and {%s}: %s",
			clusterIdentity, remotePodCIDR, remoteExternalCIDR, err)
		rc.Eventf(tep, "Warning", "Processing", "unable to configure route for destinations {%s} and {%s}: %s",
			remotePodCIDR, remoteExternalCIDR, err.Error())
		return result, err
	}
	if added {
		klog.Infof("%s -> route for destinations {%s} and {%s} correctly configured", clusterIdentity, remotePodCIDR, remoteExternalCIDR)
		rc.Eventf(tep, "Normal", "Processing", "route for destinations {%s} and {%s} configured", remotePodCIDR, remoteExternalCIDR)
	}
	return result, nil
}

// ConfigureFirewall launches a long-running go routine that ensures the firewall configuration.
func (rc *RouteController) ConfigureFirewall() error {
	ipt, err := iptables.NewIPTHandler()
	if err != nil {
		return err
	}
	iptHandler := &ipt.Ipt
	rc.firewallChan = make(chan bool)
	fwRules := generateRules(rc.vxlanDev.Link.Name)

	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C: // every five seconds we enforce the firewall rules.
				for i := range fwRules {
					if err := addRule(iptHandler, &fwRules[i]); err != nil {
						klog.Errorf("unable to insert firewall rule {%s}: %v", fwRules[i].String(), err)
					} else {
						klog.V(5).Infof("firewall rule {%s} configured", fwRules[i].String())
					}
				}
			case <-rc.firewallChan:
				for i := range fwRules {
					if err := deleteRule(iptHandler, &fwRules[i]); err != nil {
						klog.Errorf("unable to remove firewall rule {%s}: %v", fwRules[i].String(), err)
					} else {
						klog.V(5).Infof("firewall rule {%s} removed", fwRules[i].String())
					}
				}
				close(rc.firewallChan)
				return
			}
		}
	}()

	return nil
}

// cleanUp removes all the routes, rules and devices (if any) from the
// node inserted by the operator. It is called at exit time.
func (rc *RouteController) cleanUp() {
	if rc.firewallChan != nil {
		// send signal to clean firewall rules and close the go routine.
		rc.firewallChan <- true
		// wait for the go routine to clean up.
		<-rc.firewallChan
	}

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

	// Attempt to remove our finalizer from all tunnel endpoints. In case this operation fails,
	// the cleanup will be performed by tunnel-operator when a tunnel endpoint is going to be deleted.
	var teps netv1alpha1.TunnelEndpointList
	if err := rc.List(context.Background(), &teps); err != nil {
		klog.Errorf("an error occurred while listing tunnel endpoints: %v", err)
		return
	}

	for i := range teps.Items {
		original := teps.Items[i].DeepCopy()
		if controllerutil.RemoveFinalizer(&teps.Items[i], liqoconst.LiqoRouteFinalizer(rc.podIP)) {
			// Using patch instead of update, to prevent issues in case of conflicts.
			if err := rc.Client.Patch(context.Background(), &teps.Items[i], client.MergeFrom(original)); err != nil {
				klog.Errorf("%s -> unable to remove finalizer from tunnel endpoint %q: %v",
					original.Spec.ClusterIdentity, klog.KObj(&teps.Items[i]), err)
				continue
			}
			klog.V(4).Infof("%s -> finalizer successfully removed from tunnel endpoint %q", original.Spec.ClusterIdentity, klog.KObj(&teps.Items[i]))
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
func (rc *RouteController) SetupSignalHandlerForRouteOperator(ctx context.Context) context.Context {
	opctx, done := context.WithCancel(context.Background())
	go func(r *RouteController) {
		<-ctx.Done()
		// The received signal is SIGTERM, SIGINT, SIGKILL: we should stop
		// Since we are using a context to handle the signal, it's not possible to get the signal type.
		klog.Info("the operator received a shutdown signal: cleaning up")
		r.cleanUp()
		done()
	}(rc)
	return opctx
}
