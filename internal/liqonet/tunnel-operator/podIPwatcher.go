package tunnel_operator

import (
	"github.com/liqotech/liqo/pkg/liqonet"
	"github.com/liqotech/liqo/pkg/liqonet/overlay"
	"github.com/vishvananda/netlink"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	"net"
)

func (tc *TunnelController) StartPodIPWatcher() {
	go tc.podIPWatcher()
}

func (tc *TunnelController) podIPWatcher() {
	factory := informers.NewSharedInformerFactoryWithOptions(tc.k8sClient, resyncPeriod)
	inf := factory.Core().V1().Pods().Informer()
	inf.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    tc.podIPHandlerAdd,
		UpdateFunc: tc.podIPHandlerUpdate,
		DeleteFunc: tc.podIPHandlerDelete,
	})
	inf.Run(make(chan struct{}))
}

func (tc *TunnelController) podIPHandlerAdd(obj interface{}) {
	var podName, podIP, nodeName, nodeIP string
	p, ok := obj.(*corev1.Pod)
	if !ok {
		klog.Errorf("an error occurred while converting interface to unstructured object")
		return
	}
	podName = p.Name
	nodeName = p.Spec.NodeName
	podIP = p.Status.PodIP
	//check if the node.PodCIDR has been set
	if p.Status.PodIP == "" {
		klog.Infof("ip address for pod %s running on node %s not set", podName, nodeName)
		return
	}
	//get ip of the node where the pod is running
	nodeIP, ok = tc.nodeIPs[nodeName]
	if !ok {
		return
	}
	//check that the current pod is not in host network, if so return
	if nodeIP == podIP {
		return
	}
	_, dst, err := net.ParseCIDR(podIP + "/32")
	if err != nil {
		klog.Errorf("an error occurred while parsing podIP %s: %v", err)
		return
	}
	//check if the route for the pod ip exists
	routes, err := netlink.RouteListFiltered(netlink.FAMILY_V4, &netlink.Route{Table: liqonet.RoutingTableID, Dst: dst}, netlink.RT_FILTER_DST|netlink.RT_FILTER_TABLE)
	if err != nil {
		klog.Errorf("unable to get routes for pod %s: %v")
	}
	if len(routes) == 1 {
		if routes[0].Gw.String() == overlay.GetOverlayIP(nodeIP) {
			return
		}
		if err := netlink.RouteDel(&routes[0]); err != nil {
			klog.Errorf("an error occurred while deleting outdated route for pod %s with ip %s running on node %s", podName, podIP, nodeName)
			return
		}
	}
	vxlan, err := netlink.LinkByName("liqo.vxlan")
	if err != nil {
		klog.Errorf("an error occurred while getting iface liqo.vxlan by name: %v", err)
		return
	}

	gw := net.ParseIP(overlay.GetOverlayIP(nodeIP))
	route := netlink.Route{
		LinkIndex: vxlan.Attrs().Index,
		Dst:       dst,
		Gw:        gw,
		Table:     liqonet.RoutingTableID,
	}
	if err := netlink.RouteAdd(&route); err != nil {
		klog.Errorf("an error occurred while adding route for pod %s with ip %s and gateway node %s: %v", podName, podIP, nodeName, err)
		return
	}
}

func (tc *TunnelController) podIPHandlerUpdate(oldObj interface{}, newObj interface{}) {
	tc.podIPHandlerAdd(newObj)
}

func (tc *TunnelController) podIPHandlerDelete(obj interface{}) {
	var podName, podIP, nodeName string
	p, ok := obj.(*corev1.Pod)
	if !ok {
		klog.Errorf("an error occurred while converting interface to unstructured object")
		return
	}
	podName = p.Name
	nodeName = p.Spec.NodeName
	podIP = p.Status.PodIP
	//check if the node.PodCIDR has been set
	if p.Status.PodIP == "" {
		klog.Infof("ip address for pod %s running on node %s not set", podName, nodeName)
		return
	}
	_, dst, err := net.ParseCIDR(podIP + "/32")
	if err != nil {
		klog.Errorf("an error occurred while parsing podIP %s: %v", err)
		return
	}
	route := &netlink.Route{Dst: dst, Table: liqonet.RoutingTableID}
	if err := netlink.RouteDel(route); err != nil{
		klog.Errorf("unable to remove route for pod %s with ip %s: %v")
		return
	}
}
