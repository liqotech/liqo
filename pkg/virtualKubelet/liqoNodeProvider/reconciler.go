package liqonodeprovider

import (
	"context"
	"errors"

	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/util/slice"

	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	sharingv1alpha1 "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	advertisementOperator "github.com/liqotech/liqo/internal/advertisementoperator"
)

// The reconciliation function; every time this function is called,
// the node status is updated by means of r.updateFromResourceOffer.
func (p *LiqoNodeProvider) reconcileNodeFromResourceOffer(event watch.Event) error {
	var resourceOffer sharingv1alpha1.ResourceOffer
	unstruct, ok := event.Object.(*unstructured.Unstructured)
	if !ok {
		return errors.New("error in casting ResourceOffer")
	}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstruct.Object, &resourceOffer); err != nil {
		klog.Error(err)
		return err
	}

	if event.Type == watch.Deleted || !resourceOffer.DeletionTimestamp.IsZero() {
		p.updateMutex.Lock()
		defer p.updateMutex.Unlock()
		klog.Infof("resourceOffer %v is going to be deleted... set node status not ready", resourceOffer.Name)
		no, err := p.client.CoreV1().Nodes().Get(context.TODO(), p.nodeName, metav1.GetOptions{})
		if err != nil {
			klog.Error(err)
			return err
		}
		for i, condition := range no.Status.Conditions {
			if condition.Type == v1.NodeReady {
				no.Status.Conditions[i].Status = v1.ConditionFalse
				p.onNodeChangeCallback(no)
				break
			}
		}

		if err := p.handleResourceOfferDelete(&resourceOffer); err != nil {
			klog.Errorf("something went wrong during resourceOffer deletion - %v", err)
			return err
		}
		return nil
	}

	if err := p.updateFromResourceOffer(&resourceOffer); err != nil {
		klog.Errorf("node update from resourceOffer %v failed for reason %v; retry...", resourceOffer.Name, err)
		return err
	}
	klog.Info("node correctly updated from resourceOffer")
	return nil
}

// The reconciliation function; every time this function is called,
// the node status is updated by means of r.updateFromAdv.
func (p *LiqoNodeProvider) reconcileNodeFromAdv(event watch.Event) error {
	var adv sharingv1alpha1.Advertisement
	unstruct, ok := event.Object.(*unstructured.Unstructured)
	if !ok {
		return errors.New("error in casting advertisement: recreate watcher")
	}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstruct.Object, &adv); err != nil {
		klog.Error(err)
		return err
	}

	if event.Type == watch.Deleted || !adv.DeletionTimestamp.IsZero() {
		p.updateMutex.Lock()
		defer p.updateMutex.Unlock()
		klog.Infof("advertisement %v is going to be deleted... set node status not ready", adv.Name)
		no, err := p.client.CoreV1().Nodes().Get(context.TODO(), p.nodeName, metav1.GetOptions{})
		if err != nil {
			klog.Error(err)
			return err
		}
		for i, condition := range no.Status.Conditions {
			if condition.Type == v1.NodeReady {
				no.Status.Conditions[i].Status = v1.ConditionFalse
				p.onNodeChangeCallback(no)
				break
			}
		}

		if err := p.handleAdvDelete(&adv); err != nil {
			klog.Errorf("something went wrong during advertisement deletion - %v", err)
			return err
		}
		return nil
	}

	if err := p.updateFromAdv(&adv); err != nil {
		klog.Errorf("node update from advertisement %v failed for reason %v; retry...", adv.Name, err)
		return err
	}
	klog.Info("node correctly updated from advertisement")
	return nil
}

func (p *LiqoNodeProvider) reconcileNodeFromTep(event watch.Event) error {
	var tep netv1alpha1.TunnelEndpoint
	unstruct, ok := event.Object.(*unstructured.Unstructured)
	if !ok {
		return errors.New("error in casting tunnel endpoint: recreate watcher")
	}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstruct.Object, &tep); err != nil {
		klog.Error(err)
		return err
	}
	if event.Type == watch.Deleted {
		p.updateMutex.Lock()
		defer p.updateMutex.Unlock()
		klog.Infof("tunnelEndpoint %v deleted", tep.Name)
		no, err := p.client.CoreV1().Nodes().Get(context.TODO(), p.nodeName, metav1.GetOptions{})
		if err != nil {
			klog.Error(err)
			return err
		}
		err = p.updateNode(no)
		if err != nil {
			klog.Error(err)
		}
		return err
	}

	if err := p.updateFromTep(&tep); err != nil {
		klog.Errorf("node update from tunnelEndpoint %v failed for reason %v; retry...", tep.Name, err)
		return err
	}
	klog.Info("correctly set pod CIDR from tunnel endpoint")
	return nil
}

// updateFromResourceOffer gets and updates the node status accordingly.
// nolint:dupl // (aleoli): Suppressing for now, it will part of a major refactoring before v0.3
func (p *LiqoNodeProvider) updateFromResourceOffer(resourceOffer *sharingv1alpha1.ResourceOffer) error {
	var err error

	p.updateMutex.Lock()
	defer p.updateMutex.Unlock()

	var no *v1.Node
	if no, err = p.client.CoreV1().Nodes().Get(context.TODO(), p.nodeName, metav1.GetOptions{}); err != nil {
		return err
	}

	no.SetAnnotations(map[string]string{
		"cluster-id": p.foreignClusterID,
	})
	no.SetLabels(mergeMaps(no.GetLabels(), resourceOffer.Spec.Labels))
	no, err = p.client.CoreV1().Nodes().Update(context.TODO(), no, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	if no.Status.Capacity == nil {
		no.Status.Capacity = v1.ResourceList{}
	}
	if no.Status.Allocatable == nil {
		no.Status.Allocatable = v1.ResourceList{}
	}
	for k, v := range resourceOffer.Spec.ResourceQuota.Hard {
		no.Status.Capacity[k] = v
		no.Status.Allocatable[k] = v
	}
	if no.Status.Conditions == nil {
		no.Status.Conditions = []v1.NodeCondition{
			{
				Type:   v1.NodeReady,
				Status: v1.ConditionFalse,
			},
			{
				Type:   v1.NodeMemoryPressure,
				Status: v1.ConditionTrue,
			},
			{
				Type:   v1.NodeDiskPressure,
				Status: v1.ConditionFalse,
			},
			{
				Type:   v1.NodePIDPressure,
				Status: v1.ConditionFalse,
			},
			{
				Type:   v1.NodeNetworkUnavailable,
				Status: v1.ConditionTrue,
			},
		}
	}

	no.Status.Images = []v1.ContainerImage{}
	no.Status.Images = append(no.Status.Images, resourceOffer.Spec.Images...)

	return p.updateNode(no)
}

// updateFromAdv gets and updates the node status accordingly.
// nolint:dupl // (aleoli): Suppressing for now, it will part of a major refactoring before v0.3
func (p *LiqoNodeProvider) updateFromAdv(adv *sharingv1alpha1.Advertisement) error {
	var err error

	p.updateMutex.Lock()
	defer p.updateMutex.Unlock()

	var no *v1.Node
	if no, err = p.client.CoreV1().Nodes().Get(context.TODO(), p.nodeName, metav1.GetOptions{}); err != nil {
		return err
	}

	no.SetAnnotations(map[string]string{
		"cluster-id": p.foreignClusterID,
	})
	no.SetLabels(mergeMaps(no.GetLabels(), adv.Spec.Labels))
	no, err = p.client.CoreV1().Nodes().Update(context.TODO(), no, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	if no.Status.Capacity == nil {
		no.Status.Capacity = v1.ResourceList{}
	}
	if no.Status.Allocatable == nil {
		no.Status.Allocatable = v1.ResourceList{}
	}
	for k, v := range adv.Spec.ResourceQuota.Hard {
		no.Status.Capacity[k] = v
		no.Status.Allocatable[k] = v
	}
	if no.Status.Conditions == nil {
		no.Status.Conditions = []v1.NodeCondition{
			{
				Type:   v1.NodeReady,
				Status: v1.ConditionFalse,
			},
			{
				Type:   v1.NodeMemoryPressure,
				Status: v1.ConditionTrue,
			},
			{
				Type:   v1.NodeDiskPressure,
				Status: v1.ConditionFalse,
			},
			{
				Type:   v1.NodePIDPressure,
				Status: v1.ConditionFalse,
			},
			{
				Type:   v1.NodeNetworkUnavailable,
				Status: v1.ConditionTrue,
			},
		}
	}

	no.Status.Images = []v1.ContainerImage{}
	no.Status.Images = append(no.Status.Images, adv.Spec.Images...)

	return p.updateNode(no)
}

func mergeMaps(m1, m2 map[string]string) map[string]string {
	for k, v := range m2 {
		m1[k] = v
	}
	return m1
}

func (p *LiqoNodeProvider) updateFromTep(tep *netv1alpha1.TunnelEndpoint) error {
	p.updateMutex.Lock()
	defer p.updateMutex.Unlock()

	no, err := p.client.CoreV1().Nodes().Get(context.TODO(), p.nodeName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if no.Status.Conditions == nil {
		no.Status.Conditions = []v1.NodeCondition{
			{
				Type:   v1.NodeReady,
				Status: v1.ConditionFalse,
			},
			{
				Type:   v1.NodeMemoryPressure,
				Status: v1.ConditionTrue,
			},
			{
				Type:   v1.NodeDiskPressure,
				Status: v1.ConditionFalse,
			},
			{
				Type:   v1.NodePIDPressure,
				Status: v1.ConditionFalse,
			},
			{
				Type:   v1.NodeNetworkUnavailable,
				Status: v1.ConditionTrue,
			},
		}
	}

	// if tep is not connected yet, return
	if tep.Status.Connection.Status != netv1alpha1.Connected {
		p.networkReady = false
		return p.updateNode(no)
	}
	p.networkReady = true
	if isChanOpen(p.networkReadyChan) {
		close(p.networkReadyChan)
	}

	return p.updateNode(no)
}

func (p *LiqoNodeProvider) updateNode(node *v1.Node) error {
	networkReady := p.networkReady
	resourceReady := node.Status.Allocatable != nil
	ready := networkReady && resourceReady

	for i, condition := range node.Status.Conditions {
		if condition.Type == v1.NodeReady {
			if ready {
				node.Status.Conditions[i].Status = v1.ConditionTrue
			} else {
				node.Status.Conditions[i].Status = v1.ConditionFalse
			}
		}
		if condition.Type == v1.NodeNetworkUnavailable {
			if networkReady {
				node.Status.Conditions[i].Status = v1.ConditionFalse
			} else {
				node.Status.Conditions[i].Status = v1.ConditionTrue
			}
		}
		if condition.Type == v1.NodeMemoryPressure {
			if resourceReady {
				node.Status.Conditions[i].Status = v1.ConditionFalse
			} else {
				node.Status.Conditions[i].Status = v1.ConditionTrue
			}
		}
	}

	p.onNodeChangeCallback(node)
	return nil
}

func (p *LiqoNodeProvider) handleResourceOfferDelete(resourceOffer *sharingv1alpha1.ResourceOffer) error {
	if isChanOpen(p.podProviderStopper) {
		close(p.podProviderStopper)
	}

	// TODO
	klog.Info("TODO: handleResourceOfferDelete")
	return nil
}

func (p *LiqoNodeProvider) handleAdvDelete(adv *sharingv1alpha1.Advertisement) error {
	if isChanOpen(p.podProviderStopper) {
		close(p.podProviderStopper)
	}

	// remove finalizer
	if slice.ContainsString(adv.Finalizers, advertisementOperator.FinalizerString, nil) {
		adv.Finalizers = slice.RemoveString(adv.Finalizers, advertisementOperator.FinalizerString, nil)
	}

	// update advertisement -> remove finalizer (which will delete virtual-kubelet)
	retriable := func(err error) bool {
		switch kerrors.ReasonForError(err) {
		case metav1.StatusReasonNotFound:
			return false
		default:
			klog.Warningf("retrying while deleting advertisement because of- ERR; %v", err)
			return true
		}
	}

	unstruct, err := runtime.DefaultUnstructuredConverter.ToUnstructured(adv)
	if err != nil {
		klog.Error(err)
		return err
	}

	if err := retry.OnError(retry.DefaultBackoff, retriable, func() error {
		_, err := p.dynClient.Resource(sharingv1alpha1.GroupVersion.WithResource("advertisements")).
			Update(context.TODO(), &unstructured.Unstructured{Object: unstruct}, metav1.UpdateOptions{})
		return err
	}); err != nil {
		return err
	}

	return nil
}

func isChanOpen(ch chan struct{}) bool {
	open := true
	select {
	case _, open = <-ch:
	default:
	}
	return open
}
