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

package routeoperator

import (
	"context"
	"os"
	"os/signal"
	"strings"
	"time"

	k8sApiErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	"github.com/liqotech/liqo/pkg/liqonet"
	"github.com/liqotech/liqo/pkg/liqonet/overlay"
	"github.com/liqotech/liqo/pkg/liqonet/utils"
	"github.com/liqotech/liqo/pkg/liqonet/wireguard"
)

var (
	resyncPeriod = 30 * time.Second
	result       = ctrl.Result{}
	// OperatorName holds the name of the route operator.
	OperatorName = "liqo-route"
)

// RouteController reconciles a TunnelEndpoint object.
type RouteController struct {
	client.Client
	record.EventRecorder
	liqonet.NetLink
	clientSet   *kubernetes.Clientset
	nodeName    string
	namespace   string
	podIP       string
	nodePodCIDR string
	wg          *wireguard.Wireguard
	DynClient   dynamic.Interface
}

// NewRouteController returns a configure route controller ready to be started.
func NewRouteController(mgr ctrl.Manager, wgc wireguard.Client, nl wireguard.Netlinker) (*RouteController, error) {
	dynClient := dynamic.NewForConfigOrDie(mgr.GetConfig())
	clientSet := kubernetes.NewForConfigOrDie(mgr.GetConfig())
	// get node name
	nodeName, err := utils.GetNodeName()
	if err != nil {
		klog.Errorf("unable to create the controller: %v", err)
		return nil, err
	}
	podIP, err := utils.GetPodIP()
	if err != nil {
		klog.Errorf("unable to create the controller: %v", err)
		return nil, err
	}
	nodePodCIDR, err := utils.GetNodePodCIDR(nodeName, clientSet)
	if err != nil {
		klog.Errorf("unable to create the controller: %v", err)
		return nil, err
	}
	namespace, err := utils.GetPodNamespace()
	if err != nil {
		klog.Errorf("unable to create the controller: %v", err)
		return nil, err
	}
	overlayIP := strings.Join([]string{overlay.GetOverlayIP(podIP.String()), "4"}, "/")
	wg, err := overlay.CreateInterface(nodeName, namespace, overlayIP, clientSet, wgc, nl)
	if err != nil {
		klog.Errorf("unable to create the controller: %v", err)
		return nil, err
	}
	r := &RouteController{
		Client:      mgr.GetClient(),
		clientSet:   clientSet,
		podIP:       podIP.String(),
		nodePodCIDR: nodePodCIDR,
		namespace:   namespace,
		wg:          wg,
		nodeName:    nodeName,
		DynClient:   dynClient,
	}
	r.setUpRouteManager(mgr.GetEventRecorderFor(strings.Join([]string{OperatorName, nodeName}, "-")))
	return r, nil
}

// cluster-role
// +kubebuilder:rbac:groups=net.liqo.io,resources=tunnelendpoints,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=net.liqo.io,resources=tunnelendpoints/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=config.liqo.io,resources=clusterconfigs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=core,resources=nodes,verbs=get
// role
// +kubebuilder:rbac:groups=core,namespace="do-not-care",resources=secrets,verbs=create;update;patch;get;list;watch;delete
// +kubebuilder:rbac:groups=core,namespace="do-not-care",resources=pods,verbs=update;patch;get;list;watch
// +kubebuilder:rbac:groups=core,namespace="do-not-care",resources=services,verbs=update;patch;get;list;watch

// Reconcile handle requests on TunnelEndpoint object to create and configure routes on Nodes.
func (r *RouteController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var tep netv1alpha1.TunnelEndpoint
	// name of our finalizer
	routeOperatorFinalizer := strings.Join([]string{OperatorName, r.nodeName, "liqo.io"}, "-")
	var err error
	if err = r.Get(ctx, req.NamespacedName, &tep); err != nil && k8sApiErrors.IsNotFound(err) {
		klog.Errorf("unable to fetch resource %s :%v", req.String(), err)
		return result, err
	}
	if k8sApiErrors.IsNotFound(err) {
		return result, err
	}
	// Here we check that the tunnelEndpoint resource has been fully processed. If not we do nothing.
	if tep.Status.RemoteNATPodCIDR == "" {
		return result, nil
	}
	clusterID := tep.Spec.ClusterID
	// examine DeletionTimestamp to determine if object is under deletion
	if tep.ObjectMeta.DeletionTimestamp.IsZero() {
		if !controllerutil.ContainsFinalizer(&tep, routeOperatorFinalizer) {
			// The object is not being deleted, so if it does not have our finalizer,
			// then lets add the finalizer and update the object. This is equivalent
			// registering our finalizer.
			controllerutil.AddFinalizer(&tep, routeOperatorFinalizer)
			if err := r.Update(ctx, &tep); err != nil {
				klog.Errorf("%s -> unable to add finalizers to resource %s: %s", clusterID, req.String(), err)
				return result, err
			}
		}
	} else {
		// the object is being deleted
		// if we encounter an error while removing the routes than we record an
		// event on the resource to notify the user
		// the finalizer is not removed
		if controllerutil.ContainsFinalizer(&tep, routeOperatorFinalizer) {
			if err := r.RemoveRoutesPerCluster(&tep); err != nil {
				return result, err
			}
			// remove the finalizer from the list and update it.
			controllerutil.RemoveFinalizer(&tep, routeOperatorFinalizer)
			if err := r.Update(ctx, &tep); err != nil {
				klog.Errorf("%s -> unable to remove finalizers from resource %s: %s", clusterID, req.String(), err)
				return result, err
			}
		}
		return result, nil
	}
	if err := r.EnsureRoutesPerCluster(r.wg.GetDeviceName(), &tep); err != nil {
		return result, err
	}
	return result, nil
}

// this function deletes the vxlan interface in host where the route operator is running
// the error is not returned because the function is called ad exit time.
func (r *RouteController) deleteOverlayIFace() {
	err := utils.DeleteIFaceByIndex(r.wg.GetLinkIndex())
	if err != nil {
		klog.Errorf("unable to remove network interface %s: %s", r.wg.GetDeviceName(), err)
	}
}

func (r *RouteController) setUpRouteManager(recorder record.EventRecorder) {
	r.NetLink = liqonet.NewRouteManager(recorder)
}

// SetupWithManager used to set up the controller with a given manager.
func (r *RouteController) SetupWithManager(mgr ctrl.Manager) error {
	resourceToBeProccesedPredicate := predicate.Funcs{
		DeleteFunc: func(e event.DeleteEvent) bool {
			// finalizers are used to check if a resource is being deleted, and perform there the needed actions
			// we don't want to reconcile on the delete of a resource.
			return false
		},
	}
	return ctrl.NewControllerManagedBy(mgr).WithEventFilter(resourceToBeProccesedPredicate).
		For(&netv1alpha1.TunnelEndpoint{}).
		Complete(r)
}

// SetupSignalHandlerForRouteOperator registers for SIGTERM, SIGINT. A stop channel is returned
// which is closed on one of these signals.
func (r *RouteController) SetupSignalHandlerForRouteOperator() context.Context {
	ctx, done := context.WithCancel(context.Background())
	c := make(chan os.Signal, 1)
	signal.Notify(c, utils.ShutdownSignals...)
	go func(r *RouteController) {
		<-c
		r.deleteOverlayIFace()
		done()
	}(r)
	return ctx
}

// Watcher initializes a dynamic informer for a resourceType passed as parameter with the handlerFuncs passed as parameters.
func (r *RouteController) Watcher(sharedDynFactory dynamicinformer.DynamicSharedInformerFactory,
	resourceType schema.GroupVersionResource, handlerFuncs cache.ResourceEventHandlerFuncs, stopCh chan struct{}) {
	klog.Infof("starting watcher for %s", resourceType.String())
	dynInformer := sharedDynFactory.ForResource(resourceType)
	// adding handlers to the informer
	dynInformer.Informer().AddEventHandler(handlerFuncs)
	dynInformer.Informer().Run(stopCh)
}
