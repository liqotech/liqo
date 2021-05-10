package tunnel_operator

import (
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/api/v1/pod"

	route_operator "github.com/liqotech/liqo/internal/liqonet/route-operator"
	"github.com/liqotech/liqo/pkg/liqonet/overlay"
)

var (
	keepalive = 10 * time.Second
)

func (tc *TunnelController) StartPodWatcher() {
	go tc.podWatcher()
}

func (tc *TunnelController) podWatcher() {
	factory := informers.NewSharedInformerFactoryWithOptions(tc.k8sClient, resyncPeriod, informers.WithNamespace(tc.namespace), informers.WithTweakListOptions(setPodSelectorLabel))
	inf := factory.Core().V1().Pods().Informer()
	inf.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    tc.podHandlerAdd,
		UpdateFunc: tc.podHandlerUpdate,
		DeleteFunc: tc.podHandlerDelete,
	})
	inf.Run(tc.stopPWChan)
}

func (tc *TunnelController) podHandlerAdd(obj interface{}) {
	var podName, nodeName string
	var allowedIPs []string
	p, ok := obj.(*corev1.Pod)
	if !ok {
		klog.Errorf("an error occurred while converting interface to unstructured object")
		return
	}
	podName = p.Name
	nodeName = p.Spec.NodeName
	//if pod is not ready just return
	if !pod.IsPodReady(p) {
		return
	}
	//check if the node.PodCIDR has been set
	nodePodCIDR, ok := p.GetAnnotations()[overlay.NodeCIDRKeyAnnotation]
	if !ok {
		klog.Infof("PodCIDR for node %s not present as an annotation on pod %s", nodeName, podName)
		return
	}
	//check if the the public key has been set
	pubKey, ok := p.GetAnnotations()[overlay.PubKeyAnnotation]
	if !ok {
		klog.Infof("wireguard public key for pod %s running on node %s not present", podName, nodeName)
		return
	}
	if p.Status.PodIP == "" {
		klog.Infof("ip address for pod %s running on node %s not set", podName, nodeName)
		return
	}
	overlayIP := strings.Join([]string{overlay.GetOverlayIP(p.Status.PodIP), "32"}, "/")
	podIP := strings.Join([]string{p.Status.PodIP, "32"}, "/")
	if nodePodCIDR != "" {
		allowedIPs = append([]string{}, overlayIP, podIP, nodePodCIDR)
	} else {
		klog.Infof("the node podCIDR for node %s is not set, make sure that all the pod traffic leaving that node is source natted to the node's IP", p.Spec.NodeName)
		allowedIPs = append([]string{}, overlayIP, podIP)
	}
	err := tc.wg.AddPeer(pubKey, p.Status.PodIP, overlay.WgListeningPort, allowedIPs, &keepalive)
	if err != nil {
		klog.Errorf("an error occurred while adding node %s to the overlay network: %v", nodeName, err)
		return
	}
	klog.Infof("node %s at %s:%s with public key '%s' added to wireguard interface", nodeName, podIP, overlay.WgListeningPort, pubKey)
}

func (tc *TunnelController) podHandlerUpdate(oldObj interface{}, newObj interface{}) {
	pOld, ok := oldObj.(*corev1.Pod)
	if !ok {
		klog.Errorf("an error occurred while converting interface to 'corev1.Pod' object")
		return
	}
	nodeName := pOld.Spec.NodeName
	podName := pOld.Name
	pNew, ok := newObj.(*corev1.Pod)
	if !ok {
		klog.Errorf("an error occurred while converting interface to 'corev1.Pod' object")
		return
	}
	//check if the the public key has been set
	pOldPubKey, ok := pOld.GetAnnotations()[overlay.PubKeyAnnotation]
	if !ok {
		tc.podHandlerAdd(newObj)
	}
	pNewPubKey, ok := pNew.GetAnnotations()[overlay.PubKeyAnnotation]
	//if the public key is removed we leave the configuration as it is.
	//do not remove the peer from the wireguard interface, we do that only when the pod is deleted
	if !ok {
		klog.Warningf("the public key for node %s in pod %s has been removed. if you want to change the key for this node consider to delete the secret containing its keys and then restart the pod", nodeName, podName)
		return
	}
	//in case of an updated key than we remove the old configuration for the current peer
	if pOldPubKey != pNewPubKey {
		if err := tc.wg.RemovePeer(pOldPubKey); err != nil {
			klog.Errorf("an error occurred while removing old configuration from wireguard interface or peer %s: %v", nodeName, err)
			return
		}
		klog.Infof("removing old configuration from wireguard interface for peer %s", nodeName)
		tc.podHandlerAdd(newObj)
	}
}

func (tc *TunnelController) podHandlerDelete(obj interface{}) {
	p, ok := obj.(*corev1.Pod)
	var nodeName string
	if !ok {
		klog.Errorf("an error occurred while converting interface to unstructured object")
		return
	}
	nodeName = p.Spec.NodeName
	//check if the the public key has been set
	pubKey, ok := p.GetAnnotations()[overlay.PubKeyAnnotation]
	if !ok {
		return
	}
	if err := tc.wg.RemovePeer(pubKey); err != nil {
		klog.Errorf("an error occurred while removing old configuration from wireguard interface or peer %s: %v", nodeName, err)
		return
	}
	klog.Infof("removing configuration from wireguard interface for peer %s", nodeName)
}

func setPodSelectorLabel(options *metav1.ListOptions) {
	labelSet := labels.Set{route_operator.PodRouteLabelKey: route_operator.PodRouteLabelValue}
	if options == nil {
		options = &metav1.ListOptions{}
		options.LabelSelector = labels.SelectorFromSet(labelSet).String()
		return
	}
	set, err := labels.ConvertSelectorToLabelsMap(options.LabelSelector)
	if err != nil {
		klog.Errorf("unable to get existing label selector: %v", err)
		return
	}
	options.LabelSelector = labels.Merge(set, labelSet).String()
}
