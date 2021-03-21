package direct_routing

import (
	"fmt"
	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	"github.com/liqotech/liqo/pkg/liqonet"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog"
	"net"
	"reflect"
)


type DirectRouteManager struct {
	record.EventRecorder
	routesPerRemoteCluster map[string]netlink.Route
	rulesPerRemoteCluster map[string]*netlink.Rule
	routingTableID int
}

func NewDirectRouteManager(routingTableName string, routingTableID int, recorder record.EventRecorder) (liqonet.NetLink, error) {
	//first we create the routing table
	if err := liqonet.CreateRoutingTable(routingTableID, routingTableName); err != nil{
		klog.Errorf("un error occurred while creating routing table with ID %d and name %s", routingTableID, routingTableName)
		return nil, err
	}
	return &DirectRouteManager{
		EventRecorder:          recorder,
		routesPerRemoteCluster: make(map[string]netlink.Route),
		rulesPerRemoteCluster: make(map[string]*netlink.Rule),
		routingTableID: routingTableID,
	}, nil
}

func (rm *DirectRouteManager) getRoute(clusterID string) (netlink.Route, bool) {
	route, ok := rm.routesPerRemoteCluster[clusterID]
	return route, ok
}

func (rm *DirectRouteManager) setRoute(clusterID string, route netlink.Route) {
	rm.routesPerRemoteCluster[clusterID] = route
}

func (rm *DirectRouteManager) deleteRoute(clusterID string) {
	delete(rm.routesPerRemoteCluster, clusterID)
}

func (rm *DirectRouteManager) getRule(clusterID string)(*netlink.Rule, bool){
	rule, ok := rm.rulesPerRemoteCluster[clusterID]
	return rule, ok
}

func (rm *DirectRouteManager) setRule(clusterID string, rule *netlink.Rule) {
	rm.rulesPerRemoteCluster[clusterID] = rule
}

func (rm *DirectRouteManager) deleteRule(clusterID string){
	delete(rm.rulesPerRemoteCluster, clusterID)
}

func (rm *DirectRouteManager) EnsureRoutesPerCluster(iface string, tep *netv1alpha1.TunnelEndpoint) error {
	//first we get the required parameters from the tep
	var dstNet *net.IPNet
	var err error
	clusterID := tep.Spec.ClusterID
	gwPodIP := net.ParseIP(tep.Status.GatewayPodIP)
	_, remotePodCIDR := liqonet.GetPodCIDRS(tep)
	if _, dstNet, err = net.ParseCIDR(remotePodCIDR); err != nil{
		klog.Errorf("%s -> unable to parse remote podCIDR %s: %v", clusterID, remotePodCIDR, err)
		return err
	}
	//we try to get the gateway ip and link index used to reach the gatewayPod
	rGwIP, rIfIndex, rFlags, err := GetNextHop(gwPodIP.String())
	if err != nil{
		klog.Errorf("%s -> an error occurred while getting the route to the gateway pod: %v", clusterID, err)
		return err
	}
	existingRoute, ok := rm.getRoute(clusterID)
	//check if the network parameters are the same and if we need to remove the old route and add the new one
	if ok {
		if !reflect.DeepEqual(existingRoute.Dst, dstNet) && !reflect.DeepEqual(existingRoute.Gw, rGwIP) && existingRoute.LinkIndex != rIfIndex && existingRoute.Flags != rFlags{
			//remove the old route
			if err := netlink.RouteDel(&existingRoute);err != nil && err != unix.ESRCH{
				klog.Errorf("%s -> unable to remove outdated route '%s': %s", clusterID, remotePodCIDR, err)
				rm.Eventf(tep, "Warning", "Processing", "unable to remove outdated route: %s", err.Error())
				return err
			}
			rm.deleteRoute(clusterID)
		}
	}
	route := netlink.Route{
		LinkIndex: rIfIndex,
		Dst:       dstNet,
		Gw:        rGwIP,
		Table:     liqonet.RoutingTableID,
		Flags:     rFlags,
	}
	if err := netlink.RouteAdd(&route); err != nil && err != unix.EEXIST{
		klog.Errorf("%s -> unable to configure route: %s", clusterID, err)
		rm.Eventf(tep, "Warning", "Processing", "unable to configure route: %s", err.Error())
		return err
	}else if err == nil{
		rm.setRoute(clusterID, route)
		rm.Event(tep, "Normal", "Processing", "route configured")
		klog.Infof("%s -> route '%s' correctly configured", clusterID, route.String())
	}

	existingRule, ok := rm.getRule(clusterID)
	if ok{
		if !reflect.DeepEqual(existingRule.Dst, dstNet){
			if err := liqonet.RemovePolicyRoutingRule(liqonet.RoutingTableID, nil, dstNet); err != nil{
				klog.Errorf("%s -> unable to remove outdated policy routing rule: %s", clusterID, err)
				rm.Eventf(tep, "Warning", "Processing", "unable to remove outdated policy routing rule: %s", err.Error())
				return err
			}
			rm.deleteRule(clusterID)
		}
	}
	if rule, err := liqonet.InsertPolicyRoutingRule(liqonet.RoutingTableID, nil, dstNet); err != nil && err != unix.EEXIST{
		klog.Errorf("%s -> unable to configure policy routing rule: %v", clusterID, err)
		rm.Eventf(tep, "Warning", "Processing", "unable to configure policy routing rule: %s", err.Error())
		return err
	}else if err == nil{
		rm.setRule(clusterID, rule)
		rm.Event(tep, "Normal", "Processing", "policy routing rule configured")
		klog.Infof("%s -> policy routing rule '%s' correctly configured", clusterID, rule.String())
	}
	return nil
}

//used to remove the routes when a tunnelEndpoint CR is removed
func (rm *DirectRouteManager) RemoveRoutesPerCluster(tep *netv1alpha1.TunnelEndpoint) error {
	clusterID := tep.Spec.ClusterID
	route, ok := rm.getRoute(clusterID)
	if ok {
		if err := netlink.RouteDel(&route); err != nil && err != unix.ESRCH {
			rm.Eventf(tep, "Warning", "Processing", "unable to remove route: %s", err.Error())
			klog.Errorf("%s -> unable to remove route '%s': %v", clusterID, route.String(), err)
			return err
		}
		rm.Event(tep, "Normal", "Processing", "route correctly removed")
		klog.Infof("%s -> route '%s' correctly removed", clusterID, route.String())
		//remove route from the map
		rm.deleteRoute(clusterID)
	}
	rule, ok := rm.getRule(clusterID)
	if ok {
		err := liqonet.RemovePolicyRoutingRule(rule.Table, rule.Src, rule.Dst)
		if err != nil {
			rm.Eventf(tep, "Warning", "Processing", "unable to remove policy routing rule: %s", err.Error())
			klog.Errorf("%s -> unable to remove policy routing rule '%s': %v", clusterID, route.String(), err)
			return err
		}
		rm.Event(tep, "Normal", "Processing", "policy routing rule correctly removed")
		klog.Infof("%s -> policy routing rule correctly removed", clusterID)
		//remove route from the map
		rm.deleteRule(clusterID)
	}
	return nil
}

//for a given ip address the function returns the gateway ip address
func GetNextHop(ip string) (gw net.IP, linkIndex int, routeFlags int, err error) {
	dst := net.ParseIP(ip)
	//first we get all the routing rules from the main routing table
	rules, err := netlink.RouteList(nil, unix.AF_INET)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("an error occurred while listing routing rules: %v", err)
	}
	for _, r := range rules {
		if r.Dst != nil && r.Dst.Contains(dst) {
			return r.Gw, r.LinkIndex, r.Flags, nil
		}
	}
	return nil, 0, 0, fmt.Errorf("no routing rule found for ip address %s", ip)
}
