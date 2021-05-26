package tunneloperator

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"

	"github.com/liqotech/liqo/pkg/liqonet/overlay"
)

var (
	serviceLabelKey   = "net.liqo.io/gateway"
	serviceLabelValue = "true"
)

// StartServiceWatcher starts the service informer.
func (tc *TunnelController) StartServiceWatcher() {
	go tc.serviceWatcher()
}

func (tc *TunnelController) serviceWatcher() {
	factory := informers.NewSharedInformerFactoryWithOptions(tc.k8sClient, resyncPeriod, informers.WithNamespace(tc.namespace),
		informers.WithTweakListOptions(setServiceSelectorLabel))
	inf := factory.Core().V1().Services().Informer()
	inf.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    tc.serviceHandlerAdd,
		UpdateFunc: tc.serviceHandlerUpdate,
	})
	inf.Run(tc.stopSWChan)
}

func (tc *TunnelController) serviceHandlerAdd(obj interface{}) {
	c := tc.k8sClient
	ns := tc.namespace
	s, ok := obj.(*corev1.Service)
	if !ok {
		klog.Errorf("an error occurred while converting interface to 'corev1.Service'")
		return
	}
	if s.Spec.Type != corev1.ServiceTypeNodePort && s.Spec.Type != corev1.ServiceTypeLoadBalancer {
		klog.Errorf("the service %s in namespace %s is of type %s, only types of %s and %s are accepted", s.GetName(),
			s.GetNamespace(), s.Spec.Type, corev1.ServiceTypeLoadBalancer, corev1.ServiceTypeNodePort)
		return
	}
	currentPubKey := tc.wg.GetPubKey()
	pubKey := s.GetAnnotations()[overlay.PubKeyAnnotation]
	if pubKey != currentPubKey {
		pubKey = currentPubKey
	} else {
		return
	}
	retryError := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		svc, err := c.CoreV1().Services(ns).Get(context.Background(), s.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		annotations := svc.GetAnnotations()
		if annotations == nil {
			annotations = make(map[string]string)
		}
		annotations[overlay.PubKeyAnnotation] = pubKey
		svc.SetAnnotations(annotations)
		_, err = c.CoreV1().Services(ns).Update(context.Background(), svc, metav1.UpdateOptions{})
		return err
	})
	if retryError != nil {
		klog.Errorf("an error occurred while updating pod %s: %s", s.Name, retryError)
		return
	}
}

func (tc *TunnelController) serviceHandlerUpdate(oldObj, newObj interface{}) {
	tc.serviceHandlerAdd(newObj)
}

func setServiceSelectorLabel(options *metav1.ListOptions) {
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
