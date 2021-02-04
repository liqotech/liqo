package provider

import (
	"context"
	"errors"
	nettypes "github.com/liqotech/liqo/apis/net/v1alpha1"
	advtypes "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	advertisementOperator "github.com/liqotech/liqo/internal/advertisement-operator"
	"github.com/liqotech/liqo/internal/liqonet/tunnelEndpointCreator"
	"github.com/liqotech/liqo/internal/monitoring"
	"github.com/liqotech/liqo/internal/virtualKubelet/node"
	"github.com/liqotech/liqo/pkg/virtualKubelet"
	"github.com/liqotech/liqo/pkg/virtualKubelet/options"
	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog"
	"k8s.io/kubernetes/pkg/util/slice"
	"strings"
)

func (p *LiqoProvider) StartNodeUpdater(nodeRunner *node.NodeController) (chan struct{}, chan struct{}, error) {
	stop := make(chan struct{}, 1)
	advName := strings.Join([]string{virtualKubelet.AdvertisementPrefix, p.foreignClusterId}, "")
	advWatcher, err := p.advClient.Resource("advertisements").Watch(metav1.ListOptions{
		FieldSelector: strings.Join([]string{"metadata.name", advName}, "="),
		Watch:         true,
	})
	if err != nil {
		return nil, nil, err
	}

	tepWatcher, err := p.tunEndClient.Resource("tunnelendpoints").Watch(metav1.ListOptions{
		LabelSelector: strings.Join([]string{"clusterID", p.foreignClusterId}, "="),
		Watch:         true,
	})
	if err != nil {
		return nil, nil, err
	}

	p.nodeController = nodeRunner

	ready := make(chan struct{}, 1)

	monitoring.PeeringProcessExecutionStarted()
	monitoring.PeeringProcessEventRegister(monitoring.VirtualKubelet, monitoring.CreateVirtualNode, monitoring.Start)

	go func() {
		<-ready

		monitoring.PeeringProcessEventRegister(monitoring.VirtualKubelet, monitoring.WaitForAdvertisement, monitoring.Start)
		monitoring.PeeringProcessEventRegister(monitoring.VirtualKubelet, monitoring.WaitForTunnelEndpoint, monitoring.Start)

		for {
			select {
			case ev := <-advWatcher.ResultChan():
				err = p.ReconcileNodeFromAdv(ev)
				if err != nil {
					klog.Error(err)
					advWatcher.Stop()
					advWatcher, err = p.advClient.Resource("advertisements").Watch(metav1.ListOptions{
						FieldSelector: strings.Join([]string{"metadata.name", advName}, "="),
						Watch:         true,
					})
					if err != nil {
						klog.Error(err)
					}
				}
			case ev := <-tepWatcher.ResultChan():
				err = p.ReconcileNodeFromTep(ev)
				if err != nil {
					klog.Error(err)
					tepWatcher.Stop()
					tepWatcher, err = p.tunEndClient.Resource("tunnelendpoints").Watch(metav1.ListOptions{
						LabelSelector: strings.Join([]string{"clusterID", p.foreignClusterId}, "="),
						Watch:         true,
					})
					if err != nil {
						klog.Error(err)
					}
				}
			case <-stop:
				advWatcher.Stop()
				tepWatcher.Stop()
				return
			}
		}
	}()

	return ready, stop, nil
}

// The reconciliation function; every time this function is called,
// the node status is updated by means of r.updateFromAdv
func (p *LiqoProvider) ReconcileNodeFromAdv(event watch.Event) error {

	adv, ok := event.Object.(*advtypes.Advertisement)
	if !ok {
		return errors.New("error in casting advertisement: recreate watcher")
	}

	if event.Type == watch.Deleted || !adv.DeletionTimestamp.IsZero() {
		klog.Infof("advertisement %v is going to be deleted... set node status not ready", adv.Name)
		no, err := p.advClient.Client().CoreV1().Nodes().Get(context.TODO(), p.nodeName.Value().ToString(), metav1.GetOptions{})
		if err != nil {
			klog.Error(err)
		}
		for i, condition := range no.Status.Conditions {
			if condition.Type == v1.NodeReady {
				no.Status.Conditions[i].Status = v1.ConditionFalse
				err = p.nodeController.UpdateNodeFromOutside(false, no)
				break
			}
		}
		if err != nil {
			klog.Error(err)
		}

		if err := p.handleAdvDelete(adv); err != nil {
			klog.Errorf("something went wrong during advertisement deletion - %v", err)
		}
		return nil
	}

	for {
		if err := p.updateFromAdv(*adv); err == nil {
			klog.Info("node correctly updated from advertisement")
			monitoring.PeeringProcessEventRegister(monitoring.VirtualKubelet, monitoring.WaitForAdvertisement, monitoring.End)
			break
		} else {
			klog.Errorf("node update from advertisement %v failed for reason %v; retry...", adv.Name, err)
		}
	}
	return nil
}

func (p *LiqoProvider) ReconcileNodeFromTep(event watch.Event) error {
	tep, ok := event.Object.(*nettypes.TunnelEndpoint)
	if !ok {
		return errors.New("error in casting tunnel endpoint: recreate watcher")
	}
	if event.Type == watch.Deleted {
		klog.Infof("tunnelEndpoint %v deleted", tep.Name)
		p.RemoteRemappedPodCidr.SetValue(tunnelEndpointCreator.DefaultPodCIDRValue)
		no, err := p.nntClient.Client().CoreV1().Nodes().Get(context.TODO(), p.nodeName.Value().ToString(), metav1.GetOptions{})
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

	for {
		if err := p.updateFromTep(*tep); err == nil {
			klog.Info("correctly set pod CIDR from tunnel endpoint")
			break
		} else {
			klog.Errorf("node update from tunnelEndpoint %v failed for reason %v; retry...", tep.Name, err)
		}
	}
	return nil
}

// updateFromAdv gets and  advertisement and updates the node status accordingly
func (p *LiqoProvider) updateFromAdv(adv advtypes.Advertisement) error {
	var err error

	var no *v1.Node
	if no, err = p.nntClient.Client().CoreV1().Nodes().Get(context.TODO(), p.nodeName.Value().ToString(), metav1.GetOptions{}); err != nil {
		return err
	}

	no.SetAnnotations(map[string]string{
		"cluster-id": p.foreignClusterId,
	})
	no.SetLabels(mergeMaps(no.GetLabels(), adv.Spec.Labels))
	no, err = p.nntClient.Client().CoreV1().Nodes().Update(context.TODO(), no, metav1.UpdateOptions{})
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
				Status: v1.ConditionFalse,
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

func mergeMaps(m1 map[string]string, m2 map[string]string) map[string]string {
	for k, v := range m2 {
		m1[k] = v
	}
	return m1
}

func (p *LiqoProvider) updateFromTep(tep nettypes.TunnelEndpoint) error {
	var tepSet bool

	// if tep.Status.CIDRs are not set yet, return
	if tep.Status.RemoteRemappedPodCIDR == "" || tep.Status.LocalRemappedPodCIDR == "" {
		return nil
	}
	if !p.RemoteRemappedPodCidr.IsSet() && !p.LocalRemappedPodCidr.IsSet() {
		tepSet = true
	}

	// else set podCIDRS from TunnelEndpoint.Status
	// Enforcement of their validity is performed in forge.changePodId
	p.RemoteRemappedPodCidr.SetValue(options.OptionValue(tep.Status.RemoteRemappedPodCIDR))
	p.LocalRemappedPodCidr.SetValue(options.OptionValue(tep.Status.LocalRemappedPodCIDR))
	if tepSet {
		monitoring.PeeringProcessEventRegister(monitoring.VirtualKubelet, monitoring.WaitForTunnelEndpoint, monitoring.End)
		close(p.tepReady)
	}

	no, err := p.nntClient.Client().CoreV1().Nodes().Get(context.TODO(), p.nodeName.Value().ToString(), metav1.GetOptions{})
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
				Status: v1.ConditionFalse,
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

	return p.updateNode(no)
}

func (p *LiqoProvider) updateNode(node *v1.Node) error {
	if p.RemoteRemappedPodCidr.Value() != "" && node.Status.Allocatable != nil {
		// both the podCIDR and the resources have been set: the node is ready
		for i, condition := range node.Status.Conditions {
			if condition.Type == v1.NodeReady {
				node.Status.Conditions[i].Status = v1.ConditionTrue
			}
			if condition.Type == v1.NodeNetworkUnavailable {
				node.Status.Conditions[i].Status = v1.ConditionFalse
			}
		}
	} else if p.RemoteRemappedPodCidr.Value() != "" && node.Status.Allocatable == nil {
		// the resources have not been set yet: set the node status to NotReady
		for i, condition := range node.Status.Conditions {
			if condition.Type == v1.NodeReady {
				node.Status.Conditions[i].Status = v1.ConditionFalse
			}
		}
	} else if p.RemoteRemappedPodCidr.Value() == "" && node.Status.Allocatable != nil {
		// the podCIDR has not been set yet: set the node status to NetworkUnavailable
		for i, condition := range node.Status.Conditions {
			if condition.Type == v1.NodeReady {
				node.Status.Conditions[i].Status = v1.ConditionFalse
			}
			if condition.Type == v1.NodeNetworkUnavailable {
				node.Status.Conditions[i].Status = v1.ConditionTrue
			}
		}
	} else {
		// both the podCIDR and resources have not been set
		for i, condition := range node.Status.Conditions {
			if condition.Type == v1.NodeReady {
				node.Status.Conditions[i].Status = v1.ConditionFalse
			}
		}
	}
	return p.nodeController.UpdateNodeFromOutside(false, node)
}

func (p *LiqoProvider) handleAdvDelete(adv *advtypes.Advertisement) error {
	if err := p.apiController.StopController(); err != nil {
		return err
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

	if err := retry.OnError(retry.DefaultBackoff, retriable, func() error {
		_, err := p.advClient.Resource("advertisements").Update(adv.Name, adv, metav1.UpdateOptions{})
		return err
	}); err != nil {
		return err
	}

	return nil
}
