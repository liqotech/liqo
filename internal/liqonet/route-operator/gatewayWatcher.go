package route_operator

import (
	"github.com/liqotech/liqo/pkg/liqonet/overlay"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

var (
	PodGatewayLabelKey   = "app.kubernetes.io/name"
	PodGatewayLabelValue = "gateway"
)

func (r *RouteController) StartGatewayWatcher() {
	go r.gatewayWatcher()
}

func (r *RouteController) gatewayWatcher() {
	factory := informers.NewSharedInformerFactoryWithOptions(r.clientSet, resyncPeriod, informers.WithNamespace(r.namespace), informers.WithTweakListOptions(setGWPodSelectorLabel))
	inf := factory.Core().V1().Pods().Informer()
	inf.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    r.gatewayHandlerAdd,
		UpdateFunc: r.gatewayHandlerUpdate,
		DeleteFunc: r.gatewayHandlerDelete,
	})
	inf.Run(make(chan struct{}))
}

func (r *RouteController) gatewayHandlerAdd(obj interface{}) {
	var podName string
	p, ok := obj.(*corev1.Pod)
	if !ok {
		klog.Errorf("an error occurred while converting interface to unstructured object")
		return
	}
	podName = p.Name
	//check if the node.PodCIDR has been set
	if p.Status.PodIP == "" {
		klog.Infof("ip address for gateway pod %s not set", podName)
		return
	}
	peer := overlay.OverlayPeer{
		Name:   podName,
		IpAddr: p.Status.PodIP,
	}
	err := r.overlay.AddPeer(peer)
	if err != nil {
		klog.Infof("an error occurred while adding gateway peer %s to the overlay: %v", podName, err)
		return
	}
	klog.Infof("creating GRE tunnel with node %s having ip %s", podName, p.Status.PodIP)
}

func (r *RouteController) gatewayHandlerUpdate(oldObj interface{}, newObj interface{}) {
	r.gatewayHandlerAdd(newObj)
}

func (r *RouteController) gatewayHandlerDelete(obj interface{}) {
	var podName string
	p, ok := obj.(*corev1.Pod)
	if !ok {
		klog.Errorf("an error occurred while converting interface to unstructured object")
		return
	}
	podName = p.Name
	peer := overlay.OverlayPeer{
		Name: podName,
	}
	err := r.overlay.RemovePeer(peer)
	if err != nil {
		klog.Infof("an error occurred while removing GRE tunnel for gateway peer %s: %v", podName, err)
		return
	}
	klog.Infof("GRE tunnel for gateway %s removed", podName)
}
