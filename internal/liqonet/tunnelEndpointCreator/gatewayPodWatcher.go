package tunnelEndpointCreator

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic/dynamicinformer"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"

	"github.com/liqotech/liqo/pkg/liqonet/utils"
)

var (
	podResource     = "pods"
	GwPodLabelKey   = "net.liqo.io/gatewayPod"
	GwPodLabelValue = "true"
)

func (tec *TunnelEndpointCreator) StartGWPodWatcher() {
	dynFactory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(tec.DynClient, ResyncPeriod, tec.Namespace, setGWPodSelectorLabel)
	go tec.Watcher(dynFactory, corev1.SchemeGroupVersion.WithResource(podResource), cache.ResourceEventHandlerFuncs{
		AddFunc:    tec.gwPodHandlerAdd,
		UpdateFunc: tec.gwPodHandlerUpdate,
	}, tec.secretClusterStopChan)
}

func (tec *TunnelEndpointCreator) gwPodHandlerAdd(obj interface{}) {
	var nodeName, nodeIP string
	objUnstruct, ok := obj.(*unstructured.Unstructured)
	if !ok {
		klog.Errorf("an error occurred while converting interface to unstructured object")
		return
	}
	p := &corev1.Pod{}
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(objUnstruct.Object, p)
	if err != nil {
		klog.Errorf("an error occurred while converting resource %s of type %s to typed object: %s", objUnstruct.GetName(), objUnstruct.GetKind(), err)
		return
	}

	service, err := getService(tec.ClientSet, serviceLabelKey, serviceLabelValue, tec.Namespace)
	if err != nil {
		klog.Error(err)
		return
	}
	if service.Spec.Type != corev1.ServiceTypeNodePort {
		return
	}

	//check if the pod has been assigned to node by the scheduler
	if p.Spec.NodeName == "" {
		klog.Infof("gateway pod has not been assigned to a node yet")
		return
	}
	nodeName = p.Spec.NodeName
	node, err := tec.ClientSet.CoreV1().Nodes().Get(context.Background(), nodeName, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("unable to get node where the gateway pod is running: %v", err)
		return
	}
	nodeIP, err = utils.GetInternalIPOfNode(node)
	if err != nil {
		klog.Error(err)
		return
	}

	//check if the node's IP where the gatewayPod is running has been set
	exitingNodeIP := service.GetAnnotations()[serviceAnnotationKey]
	if exitingNodeIP == nodeIP {
		return
	}
	retryError := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		s, err := tec.ClientSet.CoreV1().Services(tec.Namespace).Get(context.Background(), service.GetName(), metav1.GetOptions{})
		if err != nil {
			klog.Errorf("an error occurred while retrieving resource of type %s named %s: %v", service.GroupVersionKind().String(), service.GetName(), err)
			return err
		}
		annotations := s.GetAnnotations()
		if annotations == nil {
			annotations = make(map[string]string)
		}
		annotations[serviceAnnotationKey] = nodeIP
		s.SetAnnotations(annotations)
		_, err = tec.ClientSet.CoreV1().Services(tec.Namespace).Update(context.Background(), s, metav1.UpdateOptions{})
		return err
	})
	if retryError != nil {
		klog.Errorf("an error occurred while updating spec of service resource %s: %s", service.GetName(), retryError)
	}
}

func (tec *TunnelEndpointCreator) gwPodHandlerUpdate(oldObj interface{}, newObj interface{}) {
	tec.gwPodHandlerAdd(newObj)
}

func setGWPodSelectorLabel(options *metav1.ListOptions) {
	if options == nil {
		options = &metav1.ListOptions{}
		newLabelSelector := []string{options.LabelSelector, GwPodLabelKey, "=", GwPodLabelValue}
		options.LabelSelector = strings.Join(newLabelSelector, "")
	}
	if options.LabelSelector == "" {
		newLabelSelector := []string{GwPodLabelKey, "=", GwPodLabelValue}
		options.LabelSelector = strings.Join(newLabelSelector, "")
	}
}

func getService(client *k8s.Clientset, labelSelectorKey, labelSelectorValue, namespace string) (*corev1.Service, error) {
	serviceList, err := client.CoreV1().Services(namespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: strings.Join([]string{labelSelectorKey, "=", labelSelectorValue}, ""),
	})
	if err != nil {
		return nil, err
	}
	if len(serviceList.Items) == 0 {
		return nil, fmt.Errorf("no service with label %s and value %s found", labelSelectorKey, labelSelectorValue)
	}
	return &serviceList.Items[0], nil
}
