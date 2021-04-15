package tunnel_operator

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
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
	inf.Run(tc.stopPIPWChan)
}

func (tc *TunnelController) podIPHandlerAdd(obj interface{}) {
	var podName, nodeName string
	p, ok := obj.(*corev1.Pod)
	if !ok {
		klog.Errorf("an error occurred while converting interface to unstructured object")
		return
	}
	podName = p.Name
	nodeName = p.Spec.NodeName

	//check if the node.PodCIDR has been set
	if p.Status.PodIP == "" {
		klog.Infof("ip address for pod %s running on node %s not set", podName, nodeName)
		return
	}
	err := tc.overlay.AddSubnet(nodeName, p.Status.PodIP, tc.podCIDR)
	if err != nil {
		klog.Error(err)
		return
	}
	klog.Infof("routing information for pod %s with ip %s configured to use peer %s", p.Name, p.Status.PodIP, nodeName)
}

func (tc *TunnelController) podIPHandlerUpdate(oldObj interface{}, newObj interface{}) {
	tc.podIPHandlerAdd(newObj)
}

func (tc *TunnelController) podIPHandlerDelete(obj interface{}) {
	var podName, nodeName string
	p, ok := obj.(*corev1.Pod)
	if !ok {
		klog.Errorf("an error occurred while converting interface to unstructured object")
		return
	}
	podName = p.Name
	nodeName = p.Spec.NodeName

	//check if the node.PodCIDR has been set
	if p.Status.PodIP == "" {
		klog.Infof("ip address for pod %s running on node %s not set", podName, nodeName)
		return
	}
	err := tc.overlay.RemoveSubnet(nodeName, p.Status.PodIP)
	if err != nil {
		klog.Error(err)
		return
	}
	klog.Infof("routing information for pod %s with ip %s removed from peer %s", p.Name, p.Status.PodIP, nodeName)
}
