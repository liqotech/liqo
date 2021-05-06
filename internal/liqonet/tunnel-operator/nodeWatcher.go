package tunnel_operator

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

func (tc *TunnelController) StartNodeWatcher() {
	go tc.nodeWatcher()
}

func (tc *TunnelController) nodeWatcher() {
	factory := informers.NewSharedInformerFactoryWithOptions(tc.k8sClient, resyncPeriod)
	inf := factory.Core().V1().Nodes().Informer()
	inf.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    tc.nodeHandlerAdd,
		UpdateFunc: tc.nodeHandlerUpdate,
		DeleteFunc: tc.nodeHandlerDelete,
	})
	inf.Run(make(chan struct{}))
}

func (tc *TunnelController) nodeHandlerAdd(obj interface{}) {
	var nodeName, nodeIP string
	n, ok := obj.(*corev1.Node)
	if !ok {
		klog.Errorf("an error occurred while converting interface to node object")
		return
	}
	//check if it is the virtual node
	if _, ok := n.Labels["liqo.io/type"]; ok{
		return
	}
	nodeName = n.Name
	for _, addr := range n.Status.Addresses{
		if addr.Type == "InternalIP"{
			nodeIP = addr.Address
			break
		}
	}
	if nodeIP == tc.podIP{
		return
	}
	if nodeIP != ""{
		oldIP := tc.nodeIPs[nodeName]
		if oldIP != nodeIP{
			tc.nodeIPMutex.Lock()
			defer tc.nodeIPMutex.Unlock()
			tc.nodeIPs[nodeName] = nodeIP
			klog.Infof("adding IP %s for node %s", nodeIP, nodeName)
		}
	}
	return
}

func (tc *TunnelController) nodeHandlerUpdate(oldObj interface{}, newObj interface{}) {
	tc.nodeHandlerAdd(newObj)
}

func (tc *TunnelController) nodeHandlerDelete(obj interface{}){
	var nodeName  string
	n, ok := obj.(*corev1.Node)
	if !ok {
		klog.Errorf("an error occurred while converting interface to node object")
		return
	}
	nodeName = n.Name
	tc.nodeIPMutex.Lock()
	defer tc.nodeIPMutex.Unlock()
	delete(tc.nodeIPs, nodeName)
	klog.Infof("removing IP for node %s", nodeName)
}

