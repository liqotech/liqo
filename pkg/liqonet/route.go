package liqonet

import (
	"net"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"

	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
)

type NetLink interface {
	EnsureRoutesPerCluster(iface string, tep *netv1alpha1.TunnelEndpoint) error
	RemoveRoutesPerCluster(tep *netv1alpha1.TunnelEndpoint) error
}

type RouteManager struct {
	record.EventRecorder
	routesPerRemoteCluster map[string]netlink.Route
}

func NewRouteManager(recorder record.EventRecorder) NetLink {
	return &RouteManager{
		EventRecorder:          recorder,
		routesPerRemoteCluster: make(map[string]netlink.Route),
	}
}

func (rm *RouteManager) EnsureRoutesPerCluster(iface string, tep *netv1alpha1.TunnelEndpoint) error {
	clusterID := tep.Spec.ClusterID
	_, remotePodCIDR := GetPodCIDRS(tep)
	existing, ok := rm.getRoute(clusterID)
	//check if the network parameters are the same and if we need to remove the old route and add the new one
	if ok {
		if existing.Dst.String() == remotePodCIDR {
			return nil
		}
		//remove the old route
		err := rm.delRoute(existing)
		if err != nil {
			klog.Errorf("%s -> unable to remove outdated route '%s': %s", clusterID, remotePodCIDR, err)
			rm.Eventf(tep, "Warning", "Processing", "unable to remove outdated route: %s", err.Error())
			return err
		}
	}
	route, err := rm.addRoute(remotePodCIDR, "", iface, false)
	if err != nil {
		klog.Errorf("%s -> unable to configure route: %s", clusterID, err)
		rm.Eventf(tep, "Warning", "Processing", "unable to configure route: %s", err.Error())
		return err
	}
	rm.setRoute(clusterID, route)
	rm.Event(tep, "Normal", "Processing", "route configured")
	klog.Infof("%s -> route '%s' correctly configured", clusterID, route.String())
	return nil
}

//used to remove the routes when a tunnelEndpoint CR is removed
func (rm *RouteManager) RemoveRoutesPerCluster(tep *netv1alpha1.TunnelEndpoint) error {
	clusterID := tep.Spec.ClusterID
	route, ok := rm.getRoute(clusterID)
	if ok {
		err := rm.delRoute(route)
		if err != nil {
			rm.Eventf(tep, "Warning", "Processing", "unable to remove route: %s", err.Error())
			klog.Errorf("%s -> unable to remove route '%s': %v", clusterID, route.String(), err)
			return err
		}
		rm.Event(tep, "Normal", "Processing", "route correctly removed")
		klog.Infof("%s -> route '%s' correctly removed", clusterID, route.String())
		//remove route from the map
		rm.deleteRouteFromCache(clusterID)
	}
	return nil
}

func (rm *RouteManager) getRoute(clusterID string) (netlink.Route, bool) {
	route, ok := rm.routesPerRemoteCluster[clusterID]
	return route, ok
}

func (rm *RouteManager) setRoute(clusterID string, route netlink.Route) {
	rm.routesPerRemoteCluster[clusterID] = route
}

func (rm *RouteManager) deleteRouteFromCache(clusterID string) {
	delete(rm.routesPerRemoteCluster, clusterID)
}

func (rm *RouteManager) addRoute(dst string, gw string, deviceName string, onLink bool) (netlink.Route, error) {
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

func (rm *RouteManager) delRoute(route netlink.Route) error {
	//try to remove all the routes for that ip
	err := netlink.RouteDel(&route)
	if err != nil {
		if err == unix.ESRCH {
			//it means the route does not exist so we are done
			return nil
		}
		return err
	}
	return nil
}
