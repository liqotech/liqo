package tunnel_operator

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	"strings"
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
	//get key of the overlay peer where the pod is running
	peerKey, ok := tc.overlayPeers[nodeName]
	if !ok {
		klog.Infof("node %s has not been added to the overlay yet", nodeName)
		return
	}
	allowedIPs := strings.Join([]string{p.Status.PodIP, "32"}, "/")
	err := tc.wg.AddAllowedIPs(peerKey, allowedIPs)
	if err != nil {
		klog.Errorf("an error occurred while adding subnet %s to the allowedIPs for peer %s: %v", allowedIPs, nodeName, err)
		return
	}
	klog.Infof("subnet %s added to the allowedIPs for peer %s", allowedIPs, nodeName)
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
	//get key of the overlay peer where the pod is running
	peerKey, ok := tc.overlayPeers[nodeName]
	if !ok {
		klog.Infof("node %s has not been added to the overlay yet", nodeName)
		return
	}
	allowedIPs := strings.Join([]string{p.Status.PodIP, "32"}, "/")
	err := tc.wg.RemoveAllowedIPs(peerKey, allowedIPs)
	if err != nil {
		klog.Errorf("an error occurred while removing subnet %s from the allowedIPs for peer %s: %v", allowedIPs, nodeName, err)
		return
	}
	klog.Infof("subnet %s removed from the allowedIPs for peer %s", allowedIPs, nodeName)
}
