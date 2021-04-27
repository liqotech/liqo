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
package route_operator

import (
	"context"
	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	utils "github.com/liqotech/liqo/pkg/liqonet"
	direct_routing "github.com/liqotech/liqo/pkg/liqonet/direct-routing"
	"github.com/liqotech/liqo/pkg/liqonet/overlay"
	tunnelwg "github.com/liqotech/liqo/pkg/liqonet/tunnel/wireguard"
	"github.com/liqotech/liqo/pkg/liqonet/wireguard"
	k8sApiErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog"
	"net"
	"os"
	"os/signal"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"strings"
	"time"
)

var (
	resyncPeriod = 30 * time.Second
	result       = ctrl.Result{}
	OperatorName = "liqo-route"
)

// RouteController reconciles a TunnelEndpoint object
type RouteController struct {
	client.Client
	record.EventRecorder
	utils.NetLink
	clientSet     *kubernetes.Clientset
	nodeName      string
	namespace     string
	podIP         string
	nodePodCIDR   string
	wg            *wireguard.Wireguard
	DynClient     dynamic.Interface
	vxlan         *overlay.VxlanDevice
	directRouting bool
}

func NewRouteController(mgr ctrl.Manager, wgc wireguard.Client, nl wireguard.Netlinker, directRouting bool) (*RouteController, error) {
	var wg *wireguard.Wireguard
	var vxlan *overlay.VxlanDevice
	var routeManager utils.NetLink
	var err error
	dynClient := dynamic.NewForConfigOrDie(mgr.GetConfig())
	clientSet := kubernetes.NewForConfigOrDie(mgr.GetConfig())
	//get node name
	nodeName, err := utils.GetNodeName()
	if err != nil {
		klog.Errorf("unable to create the controller: %v", err)
		return nil, err
	}
	eventRecorderName := strings.Join([]string{OperatorName, nodeName}, "-")
	podIP, err := utils.GetPodIP()
	if err != nil {
		klog.Errorf("unable to create the controller: %v", err)
		return nil, err
	}
	/*nodePodCIDR, err := utils.GetNodePodCIDR(nodeName, clientSet)
	if err != nil {
		klog.Errorf("unable to create the controller: %v", err)
		return nil, err
	}*/
	namespace, err := utils.GetPodNamespace()
	if err != nil {
		klog.Errorf("unable to create the controller: %v", err)
		return nil, err
	}
	if directRouting {
		if routeManager, err = direct_routing.NewDirectRouteManager(utils.RoutingTableName, utils.RoutingTableID, mgr.GetEventRecorderFor(eventRecorderName)); err != nil {
			klog.Errorf("unable to create the controller: %v", err)
			return nil, err
		}
	} else {
		routeManager = utils.NewRouteManager(mgr.GetEventRecorderFor(eventRecorderName))
		//create vxlan device
		vxlan, err = overlay.NewVXLANDevice(&overlay.VxlanDeviceAttrs{
			Vni:      200,
			Name:     "liqo.vxlan",
			VtepPort: 4789,
			VtepAddr: podIP,
			Mtu:      tunnelwg.LinkMTU,
		})
		if err != nil {
			klog.Errorf("unable to create the vxlan interface: %v", err)
			return nil, err
		}
		overlayIP := overlay.GetOverlayIP(podIP.String())
		ip, ipNet, err := net.ParseCIDR(overlayIP + "/4")
		if err != nil {
			klog.Errorf("un error occurred while parsing CIDR %s: %v", overlayIP, err)
			return nil, err
		}
		if err = vxlan.ConfigureIPAddress(ip, ipNet); err != nil {
			klog.Errorf("un error occurred while configuring ipaddr %s for interface %s: %v", ip.String(), vxlan.Link.Name, err)
			return nil, err
		}
		//enable rp filter for vxlan device
		if err = overlay.Enable_rp_filter(vxlan.Link.Name); err != nil {
			klog.Errorf("unable to enable rp filter for interface %s: %v", vxlan.Link.Name, err)
			return nil, err
		}
		if defaultIface, err := utils.GetDefaultIfaceName(); err != nil {
			klog.Errorf("unable to enable rp filter for interface %s: %v", defaultIface, err)
			return nil, err
		}
	}
	r := &RouteController{
		Client:    mgr.GetClient(),
		clientSet: clientSet,
		podIP:     podIP.String(),
		//nodePodCIDR:   nodePodCIDR,
		namespace:     namespace,
		wg:            wg,
		nodeName:      nodeName,
		DynClient:     dynClient,
		NetLink:       routeManager,
		vxlan:         vxlan,
		directRouting: directRouting,
	}
	return r, nil
}

//cluster-role
// +kubebuilder:rbac:groups=net.liqo.io,resources=tunnelendpoints,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=net.liqo.io,resources=tunnelendpoints/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=config.liqo.io,resources=clusterconfigs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=core,resources=nodes,verbs=get
//role
// +kubebuilder:rbac:groups=core,namespace="do-not-care",resources=secrets,verbs=create;update;patch;get;list;watch;delete
// +kubebuilder:rbac:groups=core,namespace="do-not-care",resources=pods,verbs=update;patch;get;list;watch
// +kubebuilder:rbac:groups=core,namespace="do-not-care",resources=services,verbs=update;patch;get;list;watch

func (r *RouteController) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	var tep netv1alpha1.TunnelEndpoint
	var ifaceName string
	//name of our finalizer
	routeOperatorFinalizer := strings.Join([]string{OperatorName, r.podIP, "liqo.io"}, "-")
	var err error
	if err = r.Get(ctx, req.NamespacedName, &tep); err != nil && k8sApiErrors.IsNotFound(err) {
		klog.Errorf("unable to fetch resource %s :%v", req.String(), err)
		return result, err
	}
	if k8sApiErrors.IsNotFound(err) {
		return result, err
	}
	//Here we check that the tunnelEndpoint resource has been fully processed. If not we do nothing.
	if tep.Status.RemoteRemappedPodCIDR == "" {
		return result, nil
	}
	if !r.directRouting {
		ifaceName = r.vxlan.Link.Name
	}
	if tep.Status.GatewayPodIP == "" {
		return result, nil
	}
	if r.podIP == tep.Status.GatewayPodIP {
		klog.Infof("%s -> running on same host as gateway, setting name of ifaces at 'liqo.host'", tep.Spec.ClusterID)
		ifaceName = "liqo.host"
	}
	clusterID := tep.Spec.ClusterID
	// examine DeletionTimestamp to determine if object is under deletion
	if tep.ObjectMeta.DeletionTimestamp.IsZero() {
		if !utils.ContainsString(tep.ObjectMeta.Finalizers, routeOperatorFinalizer) {
			// The object is not being deleted, so if it does not have our finalizer,
			// then lets add the finalizer and update the object. This is equivalent
			// registering our finalizer.
			tep.ObjectMeta.Finalizers = append(tep.Finalizers, routeOperatorFinalizer)
			if err := r.Update(ctx, &tep); err != nil {
				klog.Errorf("%s -> unable to add finalizers to resource %s: %s", clusterID, req.String(), err)
				return result, err
			}
		}
	} else {
		//the object is being deleted
		//if we encounter an error while removing the routes than we record an
		//event on the resource to notify the user
		//the finalizer is not removed
		if utils.ContainsString(tep.Finalizers, routeOperatorFinalizer) {
			if err := r.RemoveRoutesPerCluster(&tep); err != nil {
				return result, err
			}
			//remove the finalizer from the list and update it.
			tep.Finalizers = utils.RemoveString(tep.Finalizers, routeOperatorFinalizer)
			if err := r.Update(ctx, &tep); err != nil {
				klog.Errorf("%s -> unable to remove finalizers from resource %s: %s", clusterID, req.String(), err)
				return result, err
			}
		}
		return result, nil
	}
	if err := r.EnsureRoutesPerCluster(ifaceName, &tep); err != nil {
		return result, err
	}
	return result, nil
}

//this function deletes the vxlan interface in host where the route operator is running
//the error is not returned because the function is called ad exit time
func (r *RouteController) deleteOverlayIFace() {
	err := utils.DeleteIFaceByIndex(r.wg.GetLinkIndex())
	if err != nil {
		klog.Errorf("unable to remove network interface %s: %s", r.wg.GetDeviceName(), err)
	}
}

func (r *RouteController) setUpRouteManager(recorder record.EventRecorder) {
	r.NetLink = utils.NewRouteManager(recorder)
}

func (r *RouteController) SetupWithManager(mgr ctrl.Manager) error {
	resourceToBeProccesedPredicate := predicate.Funcs{
		DeleteFunc: func(e event.DeleteEvent) bool {
			//finalizers are used to check if a resource is being deleted, and perform there the needed actions
			//we don't want to reconcile on the delete of a resource.
			return false
		},
	}
	return ctrl.NewControllerManagedBy(mgr).WithEventFilter(resourceToBeProccesedPredicate).
		For(&netv1alpha1.TunnelEndpoint{}).
		Complete(r)
}

// SetupSignalHandlerForRouteOperator registers for SIGTERM, SIGINT. A stop channel is returned
// which is closed on one of these signals.
func (r *RouteController) SetupSignalHandlerForRouteOperator() (stopCh <-chan struct{}) {
	stop := make(chan struct{})
	c := make(chan os.Signal, 1)
	signal.Notify(c, utils.ShutdownSignals...)
	go func(r *RouteController) {
		<-c
		//r.deleteOverlayIFace()
		_ = r.vxlan.DeleteVxLanIface()
		close(stop)
	}(r)
	return stop
}

func (r *RouteController) Watcher(sharedDynFactory dynamicinformer.DynamicSharedInformerFactory, resourceType schema.GroupVersionResource, handlerFuncs cache.ResourceEventHandlerFuncs, stopCh chan struct{}) {
	klog.Infof("starting watcher for %s", resourceType.String())
	dynInformer := sharedDynFactory.ForResource(resourceType)
	//adding handlers to the informer
	dynInformer.Informer().AddEventHandler(handlerFuncs)
	dynInformer.Informer().Run(stopCh)
}
