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
	"github.com/containernetworking/plugins/pkg/ip"
	"github.com/containernetworking/plugins/pkg/ns"
	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	utils "github.com/liqotech/liqo/pkg/liqonet"
	"github.com/liqotech/liqo/pkg/liqonet/overlay"
	"github.com/liqotech/liqo/pkg/liqonet/tunnel"
	tunnelwg "github.com/liqotech/liqo/pkg/liqonet/tunnel/wireguard"
	"github.com/liqotech/liqo/pkg/liqonet/wireguard"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog"
	"net"
	"os"
	"os/signal"
	"reflect"
	"runtime"
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
	isGKE        bool
	isConfigured bool
	configChan   chan bool
	stopPWChan   chan struct{}
	stopSWChan   chan struct{}
	hostNS       ns.NetNS
	gatewayNS    ns.NetNS
}

//cluster-role
// +kubebuilder:rbac:groups=net.liqo.io,resources=tunnelendpoints,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=net.liqo.io,resources=tunnelendpoints/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=events,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=config.liqo.io,resources=clusterconfigs,verbs=get;list;watch;create;update
//role
// +kubebuilder:rbac:groups=core,namespace="do-not-care",resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,namespace="do-not-care",resources=services,verbs=get;list;watch;update
// +kubebuilder:rbac:groups=core,namespace="do-not-care",resources=pods,verbs=get;list;watch;update

//Instantiates and initializes the tunnel controller
func NewTunnelController(mgr ctrl.Manager, wgc wireguard.Client, nl wireguard.Netlinker, directRouting bool) (*TunnelController, error) {
	var wg *wireguard.Wireguard
	var gatewayNS, hostNS ns.NetNS
	clientSet := k8s.NewForConfigOrDie(mgr.GetConfig())
	namespace, err := utils.GetPodNamespace()
	if err != nil {
		return nil, err
	}
	podIP, err := utils.GetPodIP()
	if err != nil {
		return nil, err
	}
	//get name of the default interface
	defaultIface, err := utils.GetDefaultIfaceName()
	if err != nil {
		return nil, err
	}
	//enable ip forwarding
	if err = utils.EnableIPForwarding(); err != nil {
		return nil, err
	}
	if true {
		hostNS, err = ns.GetCurrentNS()
		if err != nil {
			klog.Errorf("an error occurred while getting host namespace: %v", err)
			return nil, err
		}
		gatewayNS, err = createGatewayNS("liqo-gateway")
		if err != nil {
			klog.Errorf("an error occurred while getting host namespace: %v", err)
			return nil, err
		}
		if err := createVethPair(gatewayNS); err != nil {
			return nil, err
		}
		if err := utils.EnableProxyArp("liqo.host"); err != nil {
			return nil, err
		}
		if err := enableArpProxyGW("liqo.gateway", gatewayNS); err != nil {
			return nil, err
		}
		if err := enableIPForwardingGW(gatewayNS); err != nil {
			return nil, err
		}

		if err := addRouteGW("0.0.0.0/0", "", "liqo.gateway", false, gatewayNS); err != nil {
			return nil, err
		}
		if err := addrAdd("liqo.host", &netlink.Addr{IPNet: &net.IPNet{IP: net.IPv4(169, 254, 100, 1), Mask: net.CIDRMask(24, 32)}}); err != nil {
			return nil, err
		}
		if err := addrAddGW("liqo.gateway", &netlink.Addr{IPNet: &net.IPNet{IP: net.IPv4(169, 254, 100, 2), Mask: net.CIDRMask(24, 32)}}, gatewayNS); err != nil {
			return nil, err
		}

	} else {
		overlayIP := strings.Join([]string{overlay.GetOverlayIP(podIP.String()), "4"}, "/")
		//create overlay network interface
		wg, err = overlay.CreateInterface(gatewayPodName, namespace, overlayIP, clientSet, wgc, nl)
		if err != nil {
			return nil, err
		}
		//create new custom routing table for the overlay iFace
		if err = utils.CreateRoutingTable(overlay.RoutingTableID, overlay.RoutingTableName); err != nil {
			return nil, err
		}
		//enable reverse path filter for the overlay interface
		if err = overlay.Enable_rp_filter(wg.GetDeviceName()); err != nil {
			return nil, err
		}
		//populate the custom routing table with the default route
		if err = overlay.SetUpDefaultRoute(overlay.RoutingTableID, wg.GetLinkIndex()); err != nil {
			return nil, err
		}
	}
	tc := &TunnelController{
		Client:        mgr.GetClient(),
		EventRecorder: mgr.GetEventRecorderFor(OperatorName),
		k8sClient:     clientSet,
		namespace:     namespace,
		podIP:         podIP.String(),
		DefaultIface:  defaultIface,
		wg:            wg,
		configChan:    make(chan bool),
		hostNS:        hostNS,
		gatewayNS:     gatewayNS,
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
	if !tc.isConfigured {
		<-tc.configChan
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

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
	if err := tc.gatewayNS.Set(); err != nil {
		klog.Errorf("%s -> an error occurred while changing netns to gateway: %v", endpoint.Spec.ClusterID, err)
		return ctrl.Result{}, err
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
	if reflect.DeepEqual(*con, endpoint.Status.Connection) && tc.podIP == endpoint.Status.GatewayPodIP {
		return result, nil
	}
	endpoint.Status.Connection = *con
	endpoint.Status.GatewayPodIP = tc.podIP
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
	go func(tc *TunnelController) {
		sig := <-c
		klog.Infof("received signal %s: cleaning up", sig.String())
		//close(tc.stopSWChan)
		//close(tc.stopPWChan)
		_ = removeGatewayNS("liqo-gateway")
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
	//mv wireguard interface in the gateway network namespace
	link, err := netlink.LinkByName("liqo-wg")
	if err != nil {
		return err
	}
	if err = netlink.LinkSetNsFd(link, int(tc.gatewayNS.Fd())); err != nil {
		return fmt.Errorf("failed to move wireguard interfacte to gateway netns: %v", err)
	}
	runtime.LockOSThread()
	if err != tc.gatewayNS.Set() {
		return fmt.Errorf("failed to set gateway netns:%v", err)
	}
	link, err = netlink.LinkByName("liqo-wg")
	if err != nil {
		return err
	}
	err = netlink.LinkSetUp(link)
	if err != nil {
		return fmt.Errorf("failed to set wireguard iface up in gateway netns:%v", err)
	}
	w := tc.drivers["wireguard"]
	wg := w.(*tunnelwg.Wireguard)
	if err := wg.SetNewClient(); err != nil {
		return fmt.Errorf("an error occurred while setting new client in tunnel driver")
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

func createVethPair(gatewayNS ns.NetNS) error {
	var vethPair = func(hostNS ns.NetNS) error {
		_, _, err := ip.SetupVethWithName("liqo.gateway", "liqo.host", tunnelwg.LinkMTU, hostNS)
		if err != nil {
			klog.Errorf("an error occurred while creating veth pair between host and gateway namespace")
			return err
		}
		return nil
	}
	if err := gatewayNS.Do(vethPair); err != nil {
		return err
	}
	return nil
}

func addRouteGW(dst string, gw string, deviceName string, onLink bool, gatewayNS ns.NetNS) error {
	var addRoute = func(hostNS ns.NetNS) error {
		_, err := AddRoute(dst, gw, deviceName, onLink)
		return err
	}
	if err := gatewayNS.Do(addRoute); err != nil {
		return err
	}
	return nil
}

func AddRoute(dst string, gw string, deviceName string, onLink bool) (netlink.Route, error) {
	var route netlink.Route
	//convert destination in *net.IPNet
	_, destinationNet, err := net.ParseCIDR(dst)
	if err != nil {
		return route, err
	}
	gateway := net.ParseIP(gw)
	iface, err := netlink.LinkByName(deviceName)
	if err != nil {
		return route, err
	}
	if onLink {
		route = netlink.Route{LinkIndex: iface.Attrs().Index, Dst: destinationNet, Gw: gateway, Flags: unix.RTNH_F_ONLINK}

		if err := netlink.RouteAdd(&route); err != nil && err != unix.EEXIST {
			return route, err
		}
	} else {
		route = netlink.Route{LinkIndex: iface.Attrs().Index, Dst: destinationNet, Gw: gateway}
		if err := netlink.RouteAdd(&route); err != nil && err != unix.EEXIST {
			return route, err
		}
	}
	return route, nil
}

func enableArpProxyGW(iFaceName string, gatewayNS ns.NetNS) error {
	var proxyArp = func(hostNS ns.NetNS) error {
		return utils.EnableProxyArp(iFaceName)
	}
	if err := gatewayNS.Do(proxyArp); err != nil {
		return err
	}
	return nil
}
func enableIPForwardingGW(gatewayNS ns.NetNS) error {
	var ipForwarding = func(host ns.NetNS) error {
		return utils.EnableIPForwarding()
	}
	if err := gatewayNS.Do(ipForwarding); err != nil {
		return err
	}
	return nil
}

func createGatewayNS(name string) (ns.NetNS, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	//get host namespace
	origin, err := netns.Get()
	if err != nil {
		return nil, err
	}
	gatewayNS, err := netns.NewNamed(name)
	if err != nil {
		klog.Errorf("an error occurred while creating the gateway namespace: %v", err)
		er, ok := err.(*os.PathError)
		if ok {
			if er.Err == unix.EEXIST {
				klog.Infof("please manually remove the network namespace named %s", name)
			}
		}
		return nil, err
	}
	//set the new created namespace
	if err := netns.Set(gatewayNS); err != nil {
		return nil, err
	}
	//we get the gateway net namespace as an ns.NetNS type
	gatewayNetNS, err := ns.GetCurrentNS()
	if err != nil {
		klog.Errorf("an error occurred while retrieving the gateway namespace: %v", err)
		return nil, err
	}
	if err := netns.Set(origin); err != nil {
		return nil, err
	}
	return gatewayNetNS, nil
}

func removeGatewayNS(name string) error {
	if err := netns.DeleteNamed(name); err != nil {
		klog.Errorf("an error occurred while removing network namespace with name %s: %v", name, err)
		return err
	}
	return nil
}

func addrAdd(name string, addr *netlink.Addr) error {
	//first get link by name
	link, err := netlink.LinkByName(name)
	if err != nil {
		return fmt.Errorf("an error occurred while getting link %s while adding address: %v", name, err)
	}
	if err := netlink.AddrAdd(link, addr); err != nil {
		return fmt.Errorf("an error occurred while adding addr %s to link %s: %v", addr.String(), link.Attrs().Name, err)
	}
	return nil
}

func addrAddGW(name string, addr *netlink.Addr, gatewayNS ns.NetNS) error {
	var addAdr = func(hostNS ns.NetNS) error {
		err := addrAdd(name, addr)
		return err
	}
	if err := gatewayNS.Do(addAdr); err != nil {
		return err
	}
	return nil
}
