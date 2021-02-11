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
package tunnel_operator

import (
	"context"
	"fmt"
	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	utils "github.com/liqotech/liqo/pkg/liqonet"
	"github.com/liqotech/liqo/pkg/liqonet/overlay"
	"github.com/liqotech/liqo/pkg/liqonet/tunnel"
	_ "github.com/liqotech/liqo/pkg/liqonet/tunnel/wireguard"
	"github.com/liqotech/liqo/pkg/liqonet/wireguard"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog"
	"os"
	"os/signal"
	"reflect"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"strings"
	"time"
)

var (
	result       = ctrl.Result{}
	resyncPeriod = 30 * time.Second
)

const (
	OperatorName            = "liqo-gateway"
	gatewayPodName          = "gateway-pod" //used to name the secret containing the keys of the overlay wireguard interface
	tunnelEndpointFinalizer = OperatorName + ".liqo.io"
)

// TunnelController reconciles a TunnelEndpoint object
type TunnelController struct {
	client.Client
	record.EventRecorder
	tunnel.Driver
	utils.NetLink
	utils.IPTablesHandler
	DefaultIface string
	k8sClient    *k8s.Clientset
	wg           *wireguard.Wireguard
	drivers      map[string]tunnel.Driver
	namespace    string
	podIP        string
	stopPWChan   chan struct{}
	stopSWChan   chan struct{}
}

//cluster-role
// +kubebuilder:rbac:groups=net.liqo.io,resources=tunnelendpoints,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=net.liqo.io,resources=tunnelendpoints/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=events,verbs=get;list;watch;create;update;patch;delete
//role
// +kubebuilder:rbac:groups=core,namespace="do-not-care",resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,namespace="do-not-care",resources=services,verbs=get;list;watch;update
// +kubebuilder:rbac:groups=core,namespace="do-not-care",resources=pods,verbs=get;list;watch;update

//Instantiates and initializes the tunnel controller
func NewTunnelController(mgr ctrl.Manager, wgc wireguard.Client, nl wireguard.Netlinker) (*TunnelController, error) {
	clientSet := k8s.NewForConfigOrDie(mgr.GetConfig())
	namespace, err := utils.GetPodNamespace()
	if err != nil {
		return nil, err
	}
	podIP, err := utils.GetPodIP()
	if err != nil {
		return nil, err
	}
	overlayIP := strings.Join([]string{overlay.GetOverlayIP(podIP.String()), "4"}, "/")
	//create overlay network interface
	wg, err := overlay.CreateInterface(gatewayPodName, namespace, overlayIP, clientSet, wgc, nl)
	if err != nil {
		return nil, err
	}
	//enable ip forwarding
	if err = utils.EnableIPForwarding(); err != nil {
		return nil, err
	}
	//get name of the default interface
	iface, err := utils.GetDefaultIfaceName()
	if err != nil {
		return nil, err
	}
	tc := &TunnelController{
		Client:        mgr.GetClient(),
		EventRecorder: mgr.GetEventRecorderFor(OperatorName),
		k8sClient:     clientSet,
		namespace:     namespace,
		podIP:         podIP.String(),
		DefaultIface:  iface,
		wg:            wg,
	}
	err = tc.SetUpTunnelDrivers()
	if err != nil {
		return nil, err
	}
	err = tc.SetUpIPTablesHandler()
	if err != nil {
		return nil, err
	}
	tc.SetUpRouteManager(tc.EventRecorder)
	return tc, nil
}

func (tc *TunnelController) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	var endpoint netv1alpha1.TunnelEndpoint
	//name of our finalizer
	if err := tc.Get(ctx, req.NamespacedName, &endpoint); err != nil {
		klog.Errorf("unable to fetch resource %s: %s", req.Name, err)
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	//we wait for the resource to be ready. The resource is created in two steps, first the spec and metadata fields
	//then the status field. so we wait for the status to be ready.
	if endpoint.Status.Phase != "Ready" {
		klog.Infof("%s -> resource %s is not ready", endpoint.Spec.ClusterID, endpoint.Name)
		return result, nil
	}
	// examine DeletionTimestamp to determine if object is under deletion
	if endpoint.ObjectMeta.DeletionTimestamp.IsZero() {
		if !utils.ContainsString(endpoint.ObjectMeta.Finalizers, tunnelEndpointFinalizer) {
			// The object is not being deleted, so if it does not have our finalizer,
			// then lets add the finalizer and update the object. This is equivalent
			// registering our finalizer.
			endpoint.ObjectMeta.Finalizers = append(endpoint.Finalizers, tunnelEndpointFinalizer)
			if err := tc.Update(ctx, &endpoint); err != nil {
				klog.Errorf("%s -> unable to update resource %s: %s", endpoint.Spec.ClusterID, endpoint.Name, err)
				return result, err
			}
		}
	} else {
		//the object is being deleted
		if utils.ContainsString(endpoint.Finalizers, tunnelEndpointFinalizer) {
			if err := tc.disconnectFromPeer(&endpoint); err != nil {
				return ctrl.Result{}, err
			}
			if err := tc.RemoveRoutesPerCluster(&endpoint); err != nil {
				return result, err
			}
			//remove the finalizer from the list and update it.
			endpoint.Finalizers = utils.RemoveString(endpoint.Finalizers, tunnelEndpointFinalizer)
			if err := tc.Update(ctx, &endpoint); err != nil {
				klog.Errorf("an error occurred while updating resource %s after the finalizer has been removed: %s", endpoint.Name, err)
				return result, err
			}
			return result, nil
		}
		//if object is being deleted and does not have a finalizer we just return
		return result, nil
	}
	con, err := tc.connectToPeer(&endpoint)
	if err != nil {
		return result, err
	}
	if err := tc.EnsureIPTablesRulesPerCluster(&endpoint); err != nil {
		return result, err
	}
	if err := tc.EnsureRoutesPerCluster("liqo-wg", &endpoint); err != nil {
		return result, err
	}
	if reflect.DeepEqual(*con, endpoint.Status.Connection) {
		return result, nil
	}
	endpoint.Status.Connection = *con
	if err = tc.Status().Update(context.Background(), &endpoint); err != nil {
		klog.Errorf("%s -> an error occurred while updating status of resource %s: %s", endpoint.Spec.ClusterID, endpoint.Name, err)
		return result, err
	}
	return result, nil
}

func (tc *TunnelController) connectToPeer(ep *netv1alpha1.TunnelEndpoint) (*netv1alpha1.Connection, error) {
	clusterID := ep.Spec.ClusterID
	//retrieve driver based on backend type
	driver, ok := tc.drivers[ep.Spec.BackendType]
	if !ok {
		klog.Errorf("%s -> no registered driver of type %s found for resources %s", clusterID, ep.Spec.BackendType, ep.Name)
		return nil, fmt.Errorf("no registered driver of type %s found", ep.Spec.BackendType)
	}
	con, err := driver.ConnectToEndpoint(ep)
	if err != nil {
		tc.Eventf(ep, "Warning", "Processing", "unable to establish connection: %v", err)
		klog.Errorf("%s -> an error occurred while establishing vpn connection: %v", clusterID, err)
		return nil, err
	}
	if reflect.DeepEqual(*con, ep.Status.Connection) {
		return con, nil
	}
	tc.Event(ep, "Normal", "Processing", "connection established")
	klog.Infof("%s -> vpn connection correctly established", clusterID)
	return con, nil
}

func (tc *TunnelController) disconnectFromPeer(ep *netv1alpha1.TunnelEndpoint) error {
	clusterID := ep.Spec.ClusterID
	//retrieve driver based on backend type
	driver, ok := tc.drivers[ep.Spec.BackendType]
	if !ok {
		klog.Errorf("%s -> no registered driver of type %s found for resources %s", clusterID, ep.Spec.BackendType, ep.Name)
		return fmt.Errorf("no registered driver of type %s found", ep.Spec.BackendType)
	}
	if err := driver.DisconnectFromEndpoint(ep); err != nil {
		//record an event and return
		tc.Eventf(ep, "Warning", "Processing", "unable to close connection: %v", err)
		klog.Errorf("%s -> an error occurred while closing vpn connection: %v", clusterID, err)
		return err
	}
	tc.Event(ep, "Normal", "Processing", "connection closed")
	klog.Infof("%s -> vpn connection correctly closed", clusterID)
	return nil
}

//used to remove all the tunnel interfaces when the controller is closed
//it does not return an error, but just logs them, cause we can not recover from
//them at exit time
func (tc *TunnelController) RemoveAllTunnels() {
	for driverType, driver := range tc.drivers {
		err := driver.Close()
		if err == nil {
			klog.Infof("removed tunnel interface of type %s", driverType)
		} else {
			klog.Errorf("unable to delete tunnel network interface of type %s: %s", driverType, err)
		}
	}
}

func (tc *TunnelController) EnsureIPTablesRulesPerCluster(tep *netv1alpha1.TunnelEndpoint) error {
	clusterID := tep.Spec.ClusterID
	if err := tc.EnsureChainRulespecs(tep); err != nil {
		klog.Errorf("%s -> an error occurred while creating iptables chains for the remote peer: %v", clusterID, err)
		tc.Eventf(tep, "Warning", "Processing", "unable to insert iptables rules: %v", err)
		return err
	}
	if err := tc.EnsurePostroutingRules(true, tep); err != nil {
		klog.Errorf("%s -> an error occurred while inserting iptables postrouting rules for the remote peer: %v", clusterID, err)
		tc.Eventf(tep, "Warning", "Processing", "unable to insert iptables rules: %v", err)
		return err
	}
	if err := tc.EnsurePreroutingRules(tep); err != nil {
		klog.Errorf("%s -> an error occurred while inserting iptables prerouting rules for the remote peer: %v", clusterID, err)
		tc.Eventf(tep, "Warning", "Processing", "unable to insert iptables rules: %v", err)
		return err
	}
	tc.Event(tep, "Normal", "Processing", "iptables rules correctly inserted")
	return nil
}

// SetupSignalHandlerForRouteOperator registers for SIGTERM, SIGINT, SIGKILL. A stop channel is returned
// which is closed on one of these signals.
func (tc *TunnelController) SetupSignalHandlerForTunnelOperator() (stopCh <-chan struct{}) {
	stop := make(chan struct{})
	c := make(chan os.Signal, 1)
	signal.Notify(c, utils.ShutdownSignals...)
	go func(r *TunnelController) {
		sig := <-c
		klog.Infof("received signal %s: cleaning up", sig.String())
		close(tc.stopSWChan)
		close(tc.stopPWChan)
		r.RemoveAllTunnels()
		<-c
		close(stop)
	}(tc)
	return stop
}

func (tc *TunnelController) SetupWithManager(mgr ctrl.Manager) error {
	resourceToBeProccesedPredicate := predicate.Funcs{
		DeleteFunc: func(e event.DeleteEvent) bool {
			//finalizers are used to check if a resource is being deleted, and perform there the needed actions
			//we don't want to reconcile on the delete of a resource.
			return false
		},
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&netv1alpha1.TunnelEndpoint{}).WithEventFilter(resourceToBeProccesedPredicate).
		Complete(tc)
}

//for each registered tunnel implementation it creates and initializes the driver
func (tc *TunnelController) SetUpTunnelDrivers() error {
	tc.drivers = make(map[string]tunnel.Driver)
	for tunnelType, createDriverFunc := range tunnel.Drivers {
		klog.V(3).Infof("Creating driver for tunnel of type %s", tunnelType)
		d, err := createDriverFunc(tc.k8sClient, tc.namespace)
		if err != nil {
			return err
		}
		klog.V(3).Infof("Initializing driver for %s tunnel", tunnelType)
		err = d.Init()
		if err != nil {
			return err
		}
		klog.V(3).Infof("Driver for %s tunnel created and initialized", tunnelType)
		tc.drivers[tunnelType] = d
	}
	return nil
}

func (tc *TunnelController) SetUpIPTablesHandler() error {
	iptHandler, err := utils.NewIPTablesHandler()
	if err != nil {
		return err
	}
	tc.IPTablesHandler = iptHandler
	return nil
}

func (tc *TunnelController) SetUpRouteManager(recorder record.EventRecorder) {
	tc.NetLink = utils.NewRouteManager(recorder)
}
