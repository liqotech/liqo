package route_operator

import (
	"github.com/liqotech/liqo/pkg/liqonet/overlay"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

var (
	serviceResource   = "services"
	serviceLabelKey   = "net.liqo.io/gateway"
	serviceLabelValue = "true"
)

func (r *RouteController) StartServiceWatcher() {
	dynFactory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(r.DynClient, resyncPeriod, r.namespace, setServiceFilteringLabel)
	go r.Watcher(dynFactory, corev1.SchemeGroupVersion.WithResource(serviceResource), cache.ResourceEventHandlerFuncs{
		AddFunc:    r.serviceHandlerAdd,
		UpdateFunc: r.serviceHandlerUpdate,
	}, make(chan struct{}))
}

func (r *RouteController) serviceHandlerAdd(obj interface{}) {
	var endpointIP, endpointPort string
	objUnstruct, ok := obj.(*unstructured.Unstructured)
	if !ok {
		klog.Errorf("an error occurred while converting interface to unstructured object")
		return
	}
	s := &corev1.Service{}
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(objUnstruct.Object, s)
	if err != nil {
		klog.Errorf("an error occurred while converting resource %s of type %s to typed object: %s", objUnstruct.GetName(), objUnstruct.GetKind(), err)
		return
	}
	if s.Spec.Type != corev1.ServiceTypeNodePort && s.Spec.Type != corev1.ServiceTypeLoadBalancer {
		klog.Errorf("the service %s in namespace %s is of type %s, only types of %s and %s are accepted", s.GetName(), s.GetNamespace(), s.Spec.Type, corev1.ServiceTypeLoadBalancer, corev1.ServiceTypeNodePort)
		return
	}
	endpointIP = s.Spec.ClusterIP
	for _, port := range s.Spec.Ports {
		if port.Name == "wireguard-overlay" {
			endpointPort = port.TargetPort.String()
		}
	}
	//check if the pubkey has been set
	pubKey, ok := s.GetAnnotations()[overlay.PubKeyAnnotation]
	if !ok {
		klog.Infof("overlay public key has not been set yep in service %s", s.Name)
		return
	}
	peer := overlay.OverlayPeer{
		Name:          s.Name,
		PubKey:        pubKey,
		IpAddr:        endpointIP,
		ListeningPort: endpointPort,
		AllowedIPs:    []string{"0.0.0.0/0"},
	}
	//check if the peer has been configured
	err = r.overlay.AddPeer(peer)
	if err != nil {
		klog.Error(err)
		return
	}
}

func (r *RouteController) serviceHandlerUpdate(oldObj interface{}, newObj interface{}) {
	r.serviceHandlerAdd(newObj)
}

func setServiceFilteringLabel(options *metav1.ListOptions) {
	labelSet := labels.Set{serviceLabelKey: serviceLabelValue}
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
