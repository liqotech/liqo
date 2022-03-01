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

package tunneloperator

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"reflect"
	"strings"
	"sync"

	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
	k8sApiErrors "k8s.io/apimachinery/pkg/api/errors"
	k8s "k8s.io/client-go/kubernetes"
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
	liqonetns "github.com/liqotech/liqo/pkg/liqonet/netns"
	liqorouting "github.com/liqotech/liqo/pkg/liqonet/routing"
	"github.com/liqotech/liqo/pkg/liqonet/tunnel"
	tunnelwg "github.com/liqotech/liqo/pkg/liqonet/tunnel/wireguard"
	"github.com/liqotech/liqo/pkg/liqonet/utils"
)

var (
	result = ctrl.Result{}
)

// TunnelController type of the tunnel controller.
type TunnelController struct {
	client.Client
	record.EventRecorder
	tunnel.Driver
	liqorouting.Routing
	iptables.IPTHandler
	k8sClient          k8s.Interface
	drivers            map[string]tunnel.Driver
	namespace          string
	podIP              string
	finalizer          string
	hostNetns          ns.NetNS
	gatewayNetns       ns.NetNS
	hostVeth           net.Interface
	gatewayVeth        net.Interface
	readyClustersMutex *sync.Mutex
	readyClusters      map[string]struct{}
}

// cluster-role
// +kubebuilder:rbac:groups=net.liqo.io,resources=tunnelendpoints,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=net.liqo.io,resources=tunnelendpoints/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=events,verbs=get;list;watch;create;update;patch;delete
// role
// +kubebuilder:rbac:groups=coordination.k8s.io,namespace="do-not-care",resources=leases,verbs=get;create;update
// +kubebuilder:rbac:groups=core,namespace="do-not-care",resources=secrets,verbs=get;list;watch;create;update;patch;delete

// NewTunnelController instantiates and initializes the tunnel controller.
func NewTunnelController(podIP, namespace string, er record.EventRecorder, k8sClient k8s.Interface, cl client.Client,
	readyClustersMutex *sync.Mutex, readyClusters map[string]struct{}, gatewayNetns, hostNetns ns.NetNS, mtu, port int) (*TunnelController, error) {
	tunnelEndpointFinalizer := strings.Join([]string{liqoconst.LiqoGatewayOperatorName, liqoconst.FinalizersSuffix}, ".")
	tc := &TunnelController{
		Client:             cl,
		EventRecorder:      er,
		k8sClient:          k8sClient,
		podIP:              podIP,
		namespace:          namespace,
		finalizer:          tunnelEndpointFinalizer,
		readyClustersMutex: readyClustersMutex,
		readyClusters:      readyClusters,
		gatewayNetns:       gatewayNetns,
		hostNetns:          hostNetns,
	}

	err := tc.SetUpTunnelDrivers(tunnel.Config{
		MTU:           mtu,
		ListeningPort: port,
	})
	if err != nil {
		return nil, err
	}
	link, err := netlink.LinkByName(liqoconst.DeviceName)
	if err != nil {
		return nil, err
	}
	if err = tc.setUpGWNetns(liqoconst.HostVethName, liqoconst.GatewayVethName, mtu); err != nil {
		return nil, err
	}
	// Move wireguard interface in the gateway network namespace.
	if err = netlink.LinkSetNsFd(link, int(tc.gatewayNetns.Fd())); err != nil {
		return nil, fmt.Errorf("failed to move wireguard interfacte to gateway netns: %w", err)
	}
	// After the wireguard device has been moved to the new netns we need to:
	// 1) set it up;
	// 2) replace the wgctl.Client with a new client spawned in the new netns.
	var configureWg = func(netnsNamespace ns.NetNS) error {
		link, err = netlink.LinkByName(liqoconst.DeviceName)
		if err != nil {
			return err
		}
		err = netlink.LinkSetUp(link)
		if err != nil {
			return fmt.Errorf("failed to set wireguard iface up in gateway netns: %w", err)
		}
		w := tc.drivers[liqoconst.DriverName]
		wg := w.(*tunnelwg.Wireguard)
		if err := wg.SetNewClient(); err != nil {
			return fmt.Errorf("an error occurred while setting new client in tunnel driver")
		}
		return nil
	}
	if err := tc.gatewayNetns.Do(configureWg); err != nil {
		return nil, err
	}
	err = tc.SetUpIPTablesHandler()
	if err != nil {
		return nil, err
	}
	if err := tc.SetUpRouteManager(); err != nil {
		return nil, err
	}

	return tc, nil
}

// Reconcile reconciles requests occurred on TunnelEndpoint objects.
func (tc *TunnelController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var tep = new(netv1alpha1.TunnelEndpoint)
	var err error
	var clusterID, remotePodCIDR string
	var con *netv1alpha1.Connection

	var configGWNetns = func(netNamespace ns.NetNS) error {
		con, err = tc.connectToPeer(tep)
		if err != nil {
			return err
		}
		if err = tc.EnsureIPTablesRulesPerCluster(tep); err != nil {
			return err
		}
		// Set cluster tunnel as ready
		tc.readyClustersMutex.Lock()
		defer tc.readyClustersMutex.Unlock()
		tc.readyClusters[tep.Spec.ClusterID] = struct{}{}
		added, err := tc.EnsureRoutesPerCluster(tep)
		if err != nil {
			klog.Errorf("%s -> unable to configure route '%s': %s", clusterID, remotePodCIDR, err)
			tc.Eventf(tep, "Warning", "Processing", "unable to remove outdated route: %s", err.Error())
			return err
		}
		if added {
			tc.Event(tep, "Normal", "Processing", "route configured")
			klog.Infof("%s -> route for destination {%s} correctly configured", clusterID, remotePodCIDR)
		}
		return nil
	}
	var unconfigGWNetns = func(netNamespace ns.NetNS) error {
		if err := tc.IPTHandler.RemoveIPTablesConfigurationPerCluster(tep); err != nil {
			klog.Errorf("%s -> unable to remove iptables configuration: %s",
				tep.Spec.ClusterID, err.Error())
			return err
		}
		if err := tc.disconnectFromPeer(tep); err != nil {
			return err
		}
		deleted, err := tc.RemoveRoutesPerCluster(tep)
		if err != nil {
			tc.Eventf(tep, "Warning", "Processing", "unable to remove route: %s", err.Error())
			klog.Errorf("%s -> unable to remove route for destination '%s': %v", clusterID, remotePodCIDR, err)
			return err
		}
		if deleted {
			tc.Event(tep, "Normal", "Processing", "route correctly removed")
			klog.Infof("%s -> route for destination '%s' correctly removed", clusterID, remotePodCIDR)
		}
		return nil
	}

	if err = tc.Get(ctx, req.NamespacedName, tep); err != nil && !k8sApiErrors.IsNotFound(err) {
		klog.Errorf("unable to fetch resource %s: %s", req.String(), err)
		return ctrl.Result{}, err
	}
	// In case the resource does not exist anymore, we just forget it.
	if k8sApiErrors.IsNotFound(err) {
		return result, nil
	}
	clusterID = tep.Spec.ClusterID

	_, remotePodCIDR = utils.GetPodCIDRS(tep)
	// Examine DeletionTimestamp to determine if object is under deletion.
	if tep.ObjectMeta.DeletionTimestamp.IsZero() {
		if !controllerutil.ContainsFinalizer(tep, tc.finalizer) {
			// The object is not being deleted, so if it does not have our finalizer,
			// then lets add the finalizer and update the object. This is equivalent
			// registering our finalizer.
			controllerutil.AddFinalizer(tep, tc.finalizer)
			if err := tc.Update(ctx, tep); err != nil {
				if k8sApiErrors.IsConflict(err) {
					klog.V(4).Infof("%s -> unable to add finalizers to resource %s: %s", clusterID, req.String(), err)
					return result, err
				}
				klog.Errorf("%s -> unable to update resource %s: %s", tep.Spec.ClusterID, tep.Name, err)
				return result, err
			}
		}
	} else {
		// The object is being deleted.
		if controllerutil.ContainsFinalizer(tep, tc.finalizer) {
			if err = tc.gatewayNetns.Do(unconfigGWNetns); err != nil {
				return result, err
			}

			// Remove the finalizer from the list and update it.
			controllerutil.RemoveFinalizer(tep, tc.finalizer)
			if err := tc.Update(ctx, tep); err != nil {
				if k8sApiErrors.IsConflict(err) {
					klog.V(4).Infof("%s -> unable to add finalizers to resource %s: %s", clusterID, req.String(), err)
					return result, err
				}
				klog.Errorf("an error occurred while updating resource %s after the finalizer has been removed: %s", tep.Name, err)
				return result, err
			}
		}
		// If object is being deleted and does not have a finalizer we just return.
		return result, nil
	}
	if err := tc.gatewayNetns.Do(configGWNetns); err != nil {
		return result, err
	}

	return result, tc.updateStatus(con, tep)
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
	if err := tc.EnsureChainsPerCluster(tep.Spec.ClusterID); err != nil {
		klog.Errorf("%s -> an error occurred while creating iptables chains for the remote peer: %s", clusterID, err.Error())
		tc.Eventf(tep, "Warning", "Processing", "unable to insert iptables rules: %v", err)
		return err
	}
	if err := tc.EnsureChainRulesPerCluster(tep); err != nil {
		klog.Errorf("%s -> an error occurred while inserting iptables chain rules for the remote peer: %s", clusterID, err.Error())
		tc.Eventf(tep, "Warning", "Processing", "unable to insert iptables rules: %v", err)
		return err
	}
	if err := tc.EnsurePostroutingRules(tep); err != nil {
		klog.Errorf("%s -> an error occurred while inserting iptables postrouting rules for the remote peer: %s", clusterID, err.Error())
		tc.Eventf(tep, "Warning", "Processing", "unable to insert iptables rules: %v", err)
		return err
	}
	if err := tc.EnsurePreroutingRulesPerTunnelEndpoint(tep); err != nil {
		klog.Errorf("%s -> an error occurred while inserting iptables prerouting rules for the remote peer: %s", clusterID, err.Error())
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
	go func(tc *TunnelController) {
		sig := <-c
		klog.Infof("the operator received signal {%s}: cleaning up", sig.String())
		// Here, the error is not checked, as at exit time is not possible to recover. Errors are just logged.
		tc.CleanUpConfiguration(liqoconst.GatewayNetnsName)
		done()
	}(tc)
	return ctx
}

// SetupWithManager configures the current controller to be managed by the given manager.
func (tc *TunnelController) SetupWithManager(mgr ctrl.Manager) error {
	resourceToBeProccesedPredicate := predicate.Funcs{
		DeleteFunc: func(e event.DeleteEvent) bool {
			// Finalizers are used to check if a resource is being deleted, and perform there the needed actions
			// we don't want to reconcile on the delete of a resource.
			return false
		},
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&netv1alpha1.TunnelEndpoint{}).WithEventFilter(resourceToBeProccesedPredicate).
		Complete(tc)
}

// SetUpTunnelDrivers for each registered tunnel implementation it creates and initializes the driver.
func (tc *TunnelController) SetUpTunnelDrivers(config tunnel.Config) error {
	tc.drivers = make(map[string]tunnel.Driver)
	for tunnelType, createDriverFunc := range tunnel.Drivers {
		klog.V(3).Infof("Creating driver for tunnel of type %s", tunnelType)
		d, err := createDriverFunc(tc.k8sClient, tc.namespace, config)
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

// SetUpIPTablesHandler initializes the IPTables handler of TunnelController.
func (tc *TunnelController) SetUpIPTablesHandler() error {
	iptHandler, err := iptables.NewIPTHandler()
	if err != nil {
		return err
	}
	var init = func(netNamespace ns.NetNS) error {
		if err = iptHandler.Init(); err != nil {
			klog.Errorf("an error occurred while creating iptables handler: %v", err)
			return err
		}
		return nil
	}
	if err := tc.gatewayNetns.Do(init); err != nil {
		return err
	}
	tc.IPTHandler = iptHandler
	return nil
}

// SetUpRouteManager initializes the Route manager of TunnelController.
func (tc *TunnelController) SetUpRouteManager() error {
	// Todo make the gateway routing manager to support more than one vpn technology at the same time.
	// Todo it should use the right tunnel based on the backend type set inside the tep.
	grm, err := liqorouting.NewGatewayRoutingManager(unix.RT_TABLE_MAIN, tc.drivers[liqoconst.DriverName].GetLink())
	if err != nil {
		return err
	}
	tc.Routing = grm
	return nil
}

func (tc *TunnelController) setUpGWNetns(hostVethName, gatewayVethName string, vethMtu int) error {
	var err error

	// Create veth pair to connect the two namespaces.
	hostVeth, gatewayVeth, err := liqonetns.CreateVethPair(hostVethName, gatewayVethName, tc.hostNetns, tc.gatewayNetns, vethMtu)
	if err != nil {
		return err
	}
	klog.Infof("created veth device {%s} with index {%d} in host netns with path {%s}",
		hostVeth.Name, hostVeth.Index, tc.hostNetns.Path())
	klog.Infof("created veth device {%s} with index {%d} in gateway netns with path {%s}",
		gatewayVeth.Name, gatewayVeth.Index, tc.gatewayNetns.Path())

	tc.hostVeth = hostVeth
	tc.gatewayVeth = gatewayVeth

	// Configure hostveth.
	if err = liqonetns.ConfigureVeth(&hostVeth, liqoconst.GatewayVethIPAddr, gatewayVeth.HardwareAddr, tc.hostNetns); err != nil {
		return err
	}

	// Configure gatewayveth.
	return liqonetns.ConfigureVeth(&gatewayVeth, liqoconst.HostVethIPAddr, hostVeth.HardwareAddr, tc.gatewayNetns)
}

func (tc *TunnelController) updateStatus(con *netv1alpha1.Connection, tep *netv1alpha1.TunnelEndpoint) error {
	if reflect.DeepEqual(*con, tep.Status.Connection) && tep.Status.GatewayIP == tc.podIP &&
		tep.Status.VethIFaceIndex == tc.hostVeth.Index && tep.Status.VethIP == liqoconst.GatewayVethIPAddr {
		return nil
	}

	tep.Status.Connection = *con
	tep.Status.GatewayIP = tc.podIP
	tep.Status.VethIFaceIndex = tc.hostVeth.Index
	tep.Status.VethIFaceName = tc.hostVeth.Name
	tep.Status.VethIP = liqoconst.GatewayVethIPAddr

	if err := tc.Status().Update(context.Background(), tep); err != nil {
		if k8sApiErrors.IsConflict(err) {
			klog.V(4).Infof("%s -> unable to update status for resource %s: %v", tep.Spec.ClusterID, tep.Name, err)
			return nil
		}
		klog.Errorf("%s -> an error occurred while updating status for resource %s: %v", tep.Spec.ClusterID, tep.Name, err)
		return err
	}

	return nil
}

// CleanUpConfiguration removes the veth pair existing in the host network and then removes the
// custom network namespace.
func (tc *TunnelController) CleanUpConfiguration(netnsName string) {
	klog.Infof("cleaning up...")

	klog.V(4).Infof("deleting neigh entry with mac {%s} and dst {%s} on device {%s}",
		tc.gatewayVeth.HardwareAddr.String(), liqoconst.GatewayVethIPAddr, tc.hostVeth.Name)
	if _, err := liqonetns.DelNeigh(net.ParseIP(liqoconst.GatewayVethIPAddr), tc.gatewayVeth.HardwareAddr, &tc.hostVeth); err != nil {
		klog.Errorf("an error occurred while deleting neigh entry with mac {%s} and dst {%s} on device {%s}: %v",
			tc.gatewayVeth.HardwareAddr.String(), liqoconst.GatewayVethIPAddr, tc.hostVeth.Name, err)
	}

	klog.V(4).Infof("deleting interface {%s} with index {%d} in namespace {%s}",
		tc.gatewayVeth.Name, tc.gatewayVeth.Index, tc.hostNetns.Path())
	if err := netlink.LinkDel(&netlink.Veth{LinkAttrs: netlink.LinkAttrs{Index: tc.hostVeth.Index}}); err != nil {
		klog.Errorf("an error occurred while deleting interface {%s} with index {%d} in namespace {%s}: %v",
			tc.gatewayVeth.Name, tc.gatewayVeth.Index, tc.hostNetns.Path(), err)
	}

	klog.V(4).Infof("deleting network namespace {%s} with path {%s}", netnsName, tc.gatewayNetns.Path())
	if err := liqonetns.DeleteNetns(netnsName); err != nil {
		klog.Errorf("an error occurred while deleting network namespace {%s} with path {%s}: %v",
			netnsName, tc.gatewayNetns.Path(), err)
	}
}
