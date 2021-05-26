// Package tunneloperator contains the tunnel controller.
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
package tunneloperator

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"reflect"
	"strings"
	"time"

	"github.com/containernetworking/plugins/pkg/ns"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/vishvananda/netlink"

	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
	utils "github.com/liqotech/liqo/pkg/liqonet"
	liqonetns "github.com/liqotech/liqo/pkg/liqonet/netns"
	"github.com/liqotech/liqo/pkg/liqonet/overlay"
	liqorouting "github.com/liqotech/liqo/pkg/liqonet/routing"
	"github.com/liqotech/liqo/pkg/liqonet/tunnel"

	// wireguard package is imported in order to run the init function contained in the package.
	_ "github.com/liqotech/liqo/pkg/liqonet/tunnel/wireguard"
	"github.com/liqotech/liqo/pkg/liqonet/wireguard"
)

var (
	result       = ctrl.Result{}
	resyncPeriod = 30 * time.Second
)

const (
	// OperatorName name of the operator.
	OperatorName            = "liqo-gateway"
	gatewayPodName          = "gateway-pod" // used to name the secret containing the keys of the overlay wireguard interface
	tunnelEndpointFinalizer = OperatorName + ".liqo.io"
	gatewayNetnsName        = "liqo-gateway"
	hostVethName            = "liqo.host"
	gatewayVethName         = "liqo.gateway"
)

// TunnelController type of the tunnel controller.
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
	isGKE        bool
	isConfigured bool
	configChan   chan bool
	stopPWChan   chan struct{}
	stopSWChan   chan struct{}
	hostNetns    ns.NetNS
	gatewayNetns ns.NetNS
}

// cluster-role
// +kubebuilder:rbac:groups=net.liqo.io,resources=tunnelendpoints,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=net.liqo.io,resources=tunnelendpoints/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=events,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=config.liqo.io,resources=clusterconfigs,verbs=get;list;watch;create;update
// role
// +kubebuilder:rbac:groups=core,namespace="do-not-care",resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,namespace="do-not-care",resources=services,verbs=get;list;watch;update
// +kubebuilder:rbac:groups=core,namespace="do-not-care",resources=pods,verbs=get;list;watch;update

// NewTunnelController instantiates and initializes the tunnel controller.
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
	// create overlay network interface
	wg, err := overlay.CreateInterface(gatewayPodName, namespace, overlayIP, clientSet, wgc, nl)
	if err != nil {
		return nil, err
	}
	// create new custom routing table for the overlay iFace
	if err = overlay.CreateRoutingTable(overlay.RoutingTableID, overlay.RoutingTableName); err != nil {
		return nil, err
	}
	// enable reverse path filter for the overlay interface
	if err = overlay.Enable_rp_filter(wg.GetDeviceName()); err != nil {
		return nil, err
	}
	// enable ip forwarding
	if err = utils.EnableIPForwarding(); err != nil {
		return nil, err
	}
	// populate the custom routing table with the default route
	if err = overlay.SetUpDefaultRoute(overlay.RoutingTableID, wg.GetLinkIndex()); err != nil {
		return nil, err
	}
	// get name of the default interface
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
		configChan:    make(chan bool),
	}
	err = tc.SetUpTunnelDrivers()
	if err != nil {
		return nil, err
	}
	err = tc.SetupIPTablesHandler()
	if err != nil {
		return nil, err
	}
	tc.SetupRouteManager(tc.EventRecorder)
	return tc, nil
}

// Reconcile reconciles requests occurred on TunnelEndpoint objects.
func (tc *TunnelController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	if !tc.isConfigured {
		<-tc.configChan
	}
	var endpoint netv1alpha1.TunnelEndpoint
	// name of our finalizer
	if err := tc.Get(ctx, req.NamespacedName, &endpoint); err != nil {
		klog.Errorf("unable to fetch resource %s: %s", req.Name, err)
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	// we wait for the resource to be ready. The resource is created in two steps, first the spec and metadata fields
	// then the status field. so we wait for the status to be ready.
	if endpoint.Status.Phase != liqoconst.TepReady {
		klog.Infof("%s -> resource %s is not ready", endpoint.Spec.ClusterID, endpoint.Name)
		return result, nil
	}
	_, remotePodCIDR := utils.GetPodCIDRS(&endpoint)
	// examine DeletionTimestamp to determine if object is under deletion
	if endpoint.ObjectMeta.DeletionTimestamp.IsZero() {
		if !utils.ContainsString(endpoint.ObjectMeta.Finalizers, tunnelEndpointFinalizer) {
			// The object is not being deleted, so if it does not have our finalizer,
			// then lets add the finalizer and update the object. This is equivalent
			// registering our finalizer.
			endpoint.Finalizers = append(endpoint.Finalizers, tunnelEndpointFinalizer)
			if err := tc.Update(ctx, &endpoint); err != nil {
				klog.Errorf("%s -> unable to update resource %s: %s", endpoint.Spec.ClusterID, endpoint.Name, err)
				return result, err
			}
		}
	} else {
		// the object is being deleted
		if utils.ContainsString(endpoint.Finalizers, tunnelEndpointFinalizer) {
			if err := tc.disconnectFromPeer(&endpoint); err != nil {
				return ctrl.Result{}, err
			}
			if err := tc.RemoveRoutesPerCluster(&endpoint); err != nil {
				return result, err
			}
			if tc.isGKE {
				if err := overlay.RemovePolicyRoutingRule(overlay.RoutingTableID, remotePodCIDR); err != nil {
					klog.Errorf("%s -> an error occurred while removing policy rule: %s", endpoint.Spec.ClusterID, err)
					return result, err
				}
			}
			// remove the finalizer from the list and update it.
			endpoint.Finalizers = utils.RemoveString(endpoint.Finalizers, tunnelEndpointFinalizer)
			if err := tc.Update(ctx, &endpoint); err != nil {
				klog.Errorf("an error occurred while updating resource %s after the finalizer has been removed: %s", endpoint.Name, err)
				return result, err
			}
			return result, nil
		}
		// if object is being deleted and does not have a finalizer we just return
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
	if tc.isGKE {
		if err = overlay.InsertPolicyRoutingRule(overlay.RoutingTableID, remotePodCIDR); err != nil {
			klog.Errorf("%s -> an error occurred while inserting policy rule: %s", endpoint.Spec.ClusterID, err)
			return result, err
		}
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
	// retrieve driver based on backend type
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
	// retrieve driver based on backend type
	driver, ok := tc.drivers[ep.Spec.BackendType]
	if !ok {
		klog.Errorf("%s -> no registered driver of type %s found for resources %s", clusterID, ep.Spec.BackendType, ep.Name)
		return fmt.Errorf("no registered driver of type %s found", ep.Spec.BackendType)
	}
	if err := driver.DisconnectFromEndpoint(ep); err != nil {
		// record an event and return
		tc.Eventf(ep, "Warning", "Processing", "unable to close connection: %v", err)
		klog.Errorf("%s -> an error occurred while closing vpn connection: %v", clusterID, err)
		return err
	}
	tc.Event(ep, "Normal", "Processing", "connection closed")
	klog.Infof("%s -> vpn connection correctly closed", clusterID)
	return nil
}

// RemoveAllTunnels used to remove all the tunnel interfaces when the controller is closed.
// It does not return an error, but just logs them, cause we can not recover from
// them at exit time.
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

// EnsureIPTablesRulesPerCluster ensures the iptables rules needed to configure the network for
// a given remote cluster.
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

// SetupSignalHandlerForTunnelOperator registers for SIGTERM, SIGINT, SIGKILL. A context is returned
// which is closed on one of these signals.
func (tc *TunnelController) SetupSignalHandlerForTunnelOperator() context.Context {
	ctx, done := context.WithCancel(context.Background())
	c := make(chan os.Signal, 1)
	signal.Notify(c, utils.ShutdownSignals...)
	go func(r *TunnelController) {
		sig := <-c
		klog.Infof("received signal %s: cleaning up", sig.String())
		close(tc.stopSWChan)
		close(tc.stopPWChan)
		r.RemoveAllTunnels()
		<-c
		done()
	}(tc)
	return ctx
}

// SetupWithManager configures the current controller to be managed by the given manager.
func (tc *TunnelController) SetupWithManager(mgr ctrl.Manager) error {
	resourceToBeProccesedPredicate := predicate.Funcs{
		DeleteFunc: func(e event.DeleteEvent) bool {
			// finalizers are used to check if a resource is being deleted, and perform there the needed actions
			// we don't want to reconcile on the delete of a resource.
			return false
		},
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&netv1alpha1.TunnelEndpoint{}).WithEventFilter(resourceToBeProccesedPredicate).
		Complete(tc)
}

// SetUpTunnelDrivers for each registered tunnel implementation it creates and initializes the driver.
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

// SetupIPTablesHandler configures the client which interacts with the iptables module on the system.
func (tc *TunnelController) SetupIPTablesHandler() error {
	iptHandler, err := utils.NewIPTablesHandler()
	if err != nil {
		return err
	}
	tc.IPTablesHandler = iptHandler
	return nil
}

// SetupRouteManager configure the route manager used to setup the routes for a remote cluster.
func (tc *TunnelController) SetupRouteManager(recorder record.EventRecorder) {
	tc.NetLink = utils.NewRouteManager(recorder)
}

func (tc *TunnelController) setUpGWNetns(netnsName, hostVethName, gatewayVethName, gatewayVethIPAddr string, vethMtu int) error {
	// Get current netns (hostNetns).
	var err error
	tc.hostNetns, err = ns.GetCurrentNS()
	if err != nil {
		return err
	}
	// Create new network namespace for the gateway (gatewayNetns).
	// If the namespace already exists it will be deleted and recreated.
	tc.gatewayNetns, err = liqonetns.CreateNetns(netnsName)
	if err != nil {
		return err
	}
	// Create veth pair to connect the two namespaces.
	err = liqonetns.CreateVethPair(hostVethName, gatewayVethName, tc.hostNetns, tc.gatewayNetns, vethMtu)
	if err != nil {
		klog.Errorf("an error occurred while creating the veth pair: %s", err)
		return err
	}
	if err = configureGatewayNetns(gatewayVethName, gatewayVethIPAddr, tc.gatewayNetns); err != nil {
		return err
	}
	// Enable arp proxy for liqo.host veth interface.
	if err = liqorouting.EnableProxyArp(hostVethName); err != nil {
		return err
	}
	return nil
}

func configureGatewayNetns(ifaceName, ipAddress string, gatewayNs ns.NetNS) error {
	configuration := func(netNamespace ns.NetNS) error {
		// Get veth interface.
		veth, err := netlink.LinkByName(ifaceName)
		if err != nil {
			return err
		}
		// Parse the ipAddres.
		_, ipNet, err := net.ParseCIDR(ipAddress)
		if err != nil {
			return err
		}
		// Add address to interface
		if err := netlink.AddrAdd(veth, &netlink.Addr{IPNet: ipNet}); err != nil {
			return err
		}
		// Add default route to use the veth interface.
		if _, err := liqorouting.AddRoute("0.0.0.0/0", "", veth.Attrs().Index, 0); err != nil {
			return err
		}
		// Enable arp proxy for liqo.gateway veth interface in liqo-gateway network.
		if err := liqorouting.EnableProxyArp(gatewayVethName); err != nil {
			return err
		}
		// Enable ip forwarding in the gateway namespace.
		if err := liqorouting.EnableIPForwarding(); err != nil {
			return err
		}
		return nil
	}
	return gatewayNs.Do(configuration)
}
