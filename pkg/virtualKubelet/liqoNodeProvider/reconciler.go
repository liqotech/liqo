package liqonodeprovider

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"

	"gomodules.xyz/jsonpatch/v2"
	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/util/slice"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	sharingv1alpha1 "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	advertisementOperator "github.com/liqotech/liqo/internal/advertisementoperator"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/utils"
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
		p.terminating = true
		for i, condition := range p.node.Status.Conditions {
			switch condition.Type {
			case v1.NodeReady:
				p.node.Status.Conditions[i].Status = v1.ConditionFalse
			case v1.NodeMemoryPressure:
				p.node.Status.Conditions[i].Status = v1.ConditionTrue
			default:
			}
		}
		p.node.Status.Allocatable = v1.ResourceList{}
		p.node.Status.Capacity = v1.ResourceList{}
		p.onNodeChangeCallback(p.node.DeepCopy())

		if err := p.handleResourceOfferDelete(&resourceOffer); err != nil {
			klog.Errorf("something went wrong during resourceOffer deletion - %v", err)
			return err
		}
		return nil
	}

	if err := p.ensureFinalizer(&resourceOffer, func() bool {
		return !controllerutil.ContainsFinalizer(&resourceOffer, consts.NodeFinalizer)
	}, controllerutil.AddFinalizer); err != nil {
		klog.Error(err)
		return err
	}

	if err := p.updateFromResourceOffer(&resourceOffer); err != nil {
		klog.Errorf("node update from resourceOffer %v failed for reason %v; retry...", resourceOffer.Name, err)
		return err
	}
	klog.Info("node correctly updated from resourceOffer")
	return nil
}

// ensureFinalizer ensures the finalizer status. The patch will be applied if the provided check function
// returns true, and it will build applying the provided changeFinalizer function.
func (p *LiqoNodeProvider) ensureFinalizer(resourceOffer *sharingv1alpha1.ResourceOffer,
	check func() bool, changeFinalizer func(client.Object, string)) error {
	if check() {
		original, err := json.Marshal(resourceOffer)
		if err != nil {
			klog.Error(err)
			return err
		}

		changeFinalizer(resourceOffer, consts.NodeFinalizer)

		target, err := json.Marshal(resourceOffer)
		if err != nil {
			klog.Error(err)
			return err
		}

		ops, err := jsonpatch.CreatePatch(original, target)
		if err != nil {
			klog.Error(err)
			return err
		}

		bytes, err := json.Marshal(ops)
		if err != nil {
			klog.Error(err)
			return err
		}

		_, err = p.dynClient.Resource(sharingv1alpha1.GroupVersion.WithResource("resourceoffers")).
			Namespace(resourceOffer.GetNamespace()).
			Patch(context.TODO(), resourceOffer.GetName(), types.JSONPatchType, bytes, metav1.PatchOptions{})
		if err != nil {
			klog.Error(err)
			return err
		}
	}
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
		for i, condition := range p.node.Status.Conditions {
			switch condition.Type {
			case v1.NodeReady:
				p.node.Status.Conditions[i].Status = v1.ConditionFalse
			case v1.NodeMemoryPressure:
				p.node.Status.Conditions[i].Status = v1.ConditionTrue
			default:
			}
		}
		p.node.Status.Allocatable = v1.ResourceList{}
		p.node.Status.Capacity = v1.ResourceList{}
		p.onNodeChangeCallback(p.node.DeepCopy())

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
		p.networkReady = false
		err := p.updateNode()
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
	p.updateMutex.Lock()
	defer p.updateMutex.Unlock()

	if err := p.patchLabels(resourceOffer.Spec.Labels); err != nil {
		klog.Error(err)
		return err
	}

	if p.node.Status.Capacity == nil {
		p.node.Status.Capacity = v1.ResourceList{}
	}
	if p.node.Status.Allocatable == nil {
		p.node.Status.Allocatable = v1.ResourceList{}
	}
	for k, v := range resourceOffer.Spec.ResourceQuota.Hard {
		p.node.Status.Capacity[k] = v
		p.node.Status.Allocatable[k] = v
	}

	p.node.Status.Images = []v1.ContainerImage{}
	p.node.Status.Images = append(p.node.Status.Images, resourceOffer.Spec.Images...)

	return p.updateNode()
}

// updateFromAdv gets and updates the node status accordingly.
// nolint:dupl // (aleoli): Suppressing for now, it will part of a major refactoring before v0.3
func (p *LiqoNodeProvider) updateFromAdv(adv *sharingv1alpha1.Advertisement) error {
	p.updateMutex.Lock()
	defer p.updateMutex.Unlock()

	if err := p.patchLabels(adv.Spec.Labels); err != nil {
		klog.Error(err)
		return err
	}

	if p.node.Status.Capacity == nil {
		p.node.Status.Capacity = v1.ResourceList{}
	}
	if p.node.Status.Allocatable == nil {
		p.node.Status.Allocatable = v1.ResourceList{}
	}
	for k, v := range adv.Spec.ResourceQuota.Hard {
		p.node.Status.Capacity[k] = v
		p.node.Status.Allocatable[k] = v
	}

	p.node.Status.Images = []v1.ContainerImage{}
	p.node.Status.Images = append(p.node.Status.Images, adv.Spec.Images...)

	return p.updateNode()
}

func (p *LiqoNodeProvider) updateFromTep(tep *netv1alpha1.TunnelEndpoint) error {
	p.updateMutex.Lock()
	defer p.updateMutex.Unlock()

	// if tep is not connected yet, return
	if tep.Status.Connection.Status != netv1alpha1.Connected {
		p.networkReady = false
		return p.updateNode()
	}
	p.networkReady = true
	if isChanOpen(p.networkReadyChan) {
		close(p.networkReadyChan)
	}

	return p.updateNode()
}

func (p *LiqoNodeProvider) updateNode() error {
	if p.node.Status.Conditions == nil {
		p.node.Status.Conditions = []v1.NodeCondition{
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

	networkReady := p.networkReady
	resourceReady := areResourcesReady(p.node.Status.Allocatable)
	ready := networkReady && resourceReady

	for i, condition := range p.node.Status.Conditions {
		if condition.Type == v1.NodeReady {
			if ready {
				p.node.Status.Conditions[i].Status = v1.ConditionTrue
			} else {
				p.node.Status.Conditions[i].Status = v1.ConditionFalse
			}
		}
		if condition.Type == v1.NodeNetworkUnavailable {
			if networkReady {
				p.node.Status.Conditions[i].Status = v1.ConditionFalse
			} else {
				p.node.Status.Conditions[i].Status = v1.ConditionTrue
			}
		}
		if condition.Type == v1.NodeMemoryPressure {
			if resourceReady {
				p.node.Status.Conditions[i].Status = v1.ConditionFalse
			} else {
				p.node.Status.Conditions[i].Status = v1.ConditionTrue
			}
		}
	}

	p.onNodeChangeCallback(p.node.DeepCopy())
	return nil
}

func (p *LiqoNodeProvider) handleResourceOfferDelete(resourceOffer *sharingv1alpha1.ResourceOffer) error {
	ctx := context.TODO()

	if err := p.cordonNode(ctx); err != nil {
		klog.Errorf("error cordoning node: %v", err)
		return err
	}

	if err := p.drainNode(ctx); err != nil {
		klog.Errorf("error draining node: %v", err)
		return err
	}

	if isChanOpen(p.podProviderStopper) {
		close(p.podProviderStopper)
	}

	// delete the node
	if err := p.client.CoreV1().Nodes().Delete(ctx, p.node.GetName(), metav1.DeleteOptions{}); err != nil && !kerrors.IsNotFound(err) {
		klog.Errorf("error deleting node: %v", err)
		return err
	}

	// remove the finalizer
	if err := p.ensureFinalizer(resourceOffer, func() bool {
		return controllerutil.ContainsFinalizer(resourceOffer, consts.NodeFinalizer)
	}, controllerutil.RemoveFinalizer); err != nil {
		klog.Errorf("error removing finalizer from resource offer %v/%v: %v", resourceOffer.GetNamespace(), resourceOffer.GetName(), err)
		return err
	}

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

func (p *LiqoNodeProvider) patchLabels(labels map[string]string) error {
	if reflect.DeepEqual(labels, p.lastAppliedLabels) {
		return nil
	}
	if labels == nil {
		labels = map[string]string{}
	}

	if err := p.patchNode(func(node *v1.Node) error {
		nodeLabels := node.GetLabels()
		nodeLabels = utils.SubMaps(nodeLabels, p.lastAppliedLabels)
		nodeLabels = utils.MergeMaps(nodeLabels, labels)
		node.Labels = nodeLabels
		return nil
	}); err != nil {
		klog.Error(err)
		return err
	}

	p.lastAppliedLabels = labels
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

// areResourcesReady returns true if both cpu and memory are more than zero.
func areResourcesReady(allocatable v1.ResourceList) bool {
	if allocatable == nil {
		return false
	}
	cpu := allocatable.Cpu()
	if cpu == nil || cpu.CmpInt64(0) <= 0 {
		return false
	}
	memory := allocatable.Memory()
	if memory == nil || memory.CmpInt64(0) <= 0 {
		return false
	}
	return true
}
