package kubernetes

import (
	"context"
	nettypes "github.com/liqoTech/liqo/api/net/v1alpha1"
	advtypes "github.com/liqoTech/liqo/api/sharing/v1alpha1"
	controllers "github.com/liqoTech/liqo/internal/liqonet"
	"github.com/liqoTech/liqo/internal/node"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/klog"
	"strings"
)

func (p *KubernetesProvider) StartNodeUpdater(nodeRunner *node.NodeController) (chan struct{}, chan struct{}, error) {
	stop := make(chan struct{}, 1)
	advName := strings.Join([]string{"advertisement", p.foreignClusterId}, "-")
	advWatcher, err := p.advClient.Resource("advertisements").Watch(metav1.ListOptions{
		FieldSelector: strings.Join([]string{"metadata.name", advName}, "="),
		Watch:         true,
	})
	if err != nil {
		return nil, nil, err
	}

	tepName := strings.Join([]string{controllers.TunEndpointNamePrefix, p.foreignClusterId}, "")
	tepWatcher, err := p.tunEndClient.Resource("tunnelendpoints").Watch(metav1.ListOptions{
		FieldSelector: strings.Join([]string{"metadata.name", tepName}, "="),
		Watch:         true,
	})
	if err != nil {
		return nil, nil, err
	}

	p.nodeController = nodeRunner

	ready := make(chan struct{}, 1)

	go func() {
		<-ready
		for {
			select {
			case ev := <-advWatcher.ResultChan():
				p.ReconcileNodeFromAdv(ev)
			case ev := <-tepWatcher.ResultChan():
				p.ReconcileNodeFromTep(ev)
			case <-stop:
				advWatcher.Stop()
				tepWatcher.Stop()
				return
			default:
				break
			}
		}
	}()

	return ready, stop, nil
}

// The reconciliation function; every time this function is called,
// the node status is updated by means of r.updateFromAdv
func (p *KubernetesProvider) ReconcileNodeFromAdv(event watch.Event) {

	adv, ok := event.Object.(*advtypes.Advertisement)
	if !ok {
		klog.Fatal("error in casting advertisement")
	}
	if event.Type == watch.Deleted {
		klog.Infof("advertisement %v deleted...the node is going to be deleted", adv.Name)
		return
	}

	if adv.Status.AdvertisementStatus == advtypes.AdvertisementDeleting {
		for retry := 0; retry < 3; retry++ {
			klog.Infof("advertisement %v is going to be deleted... set node status not ready", adv.Name)
			no, err := p.advClient.Client().CoreV1().Nodes().Get(context.TODO(), p.nodeName, metav1.GetOptions{})
			if err != nil {
				klog.Error(err)
				continue
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
				continue
			}
			klog.Infof("delete all offloaded resources and advertisement")
			if err := p.deleteAdv(adv); err != nil {
				klog.Infof("something went wrong during advertisement deletion - %v", err)
				continue
			}
			break
		}
	}

	for {
		if err := p.updateFromAdv(*adv); err == nil {
			klog.Info("node correctly updated from advertisement")
			break
		} else {
			klog.Errorf("node update from advertisement %v failed for reason %v; retry...", adv.Name, err)
		}
	}
}

func (p *KubernetesProvider) ReconcileNodeFromTep(event watch.Event) {
	tep, ok := event.Object.(*nettypes.TunnelEndpoint)
	if !ok {
		klog.Fatal("error in casting tunnelEndpoint")
	}
	if event.Type == watch.Deleted {
		klog.Infof("tunnelEndpoint %v deleted", tep.Name)
		p.RemoteRemappedPodCidr = ""
		no, err := p.homeClient.Client().CoreV1().Nodes().Get(context.TODO(), p.nodeName, metav1.GetOptions{})
		if err != nil {
			klog.Error(err)
			return
		}
		err = p.updateNode(no)
		if err != nil {
			klog.Error(err)
		}
		return
	}

	for {
		if err := p.updateFromTep(*tep); err == nil {
			klog.Info("correctly set pod CIDR from tunnel endpoint")
			break
		} else {
			klog.Errorf("node update from tunnelEndpoint %v failed for reason %v; retry...", tep.Name, err)
		}
	}

}

// updateFromAdv gets and  advertisement and updates the node status accordingly
func (p *KubernetesProvider) updateFromAdv(adv advtypes.Advertisement) error {
	var err error

	var no *v1.Node
	if no, err = p.homeClient.Client().CoreV1().Nodes().Get(context.TODO(), p.nodeName, metav1.GetOptions{}); err != nil {
		return err
	}

	no.SetAnnotations(map[string]string{
		"cluster-id": p.foreignClusterId,
	})

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

	no.Status.Images = []v1.ContainerImage{}
	no.Status.Images = append(no.Status.Images, adv.Spec.Images...)

	return p.updateNode(no)
}

func (p *KubernetesProvider) updateFromTep(tep nettypes.TunnelEndpoint) error {
	if tep.Status.RemoteRemappedPodCIDR != "" && tep.Status.RemoteRemappedPodCIDR != "None" {
		p.RemoteRemappedPodCidr = tep.Status.RemoteRemappedPodCIDR
	} else {
		p.RemoteRemappedPodCidr = tep.Spec.PodCIDR
	}

	if tep.Status.LocalRemappedPodCIDR != "" && tep.Status.LocalRemappedPodCIDR != "None" {
		p.LocalRemappedPodCidr = tep.Status.LocalRemappedPodCIDR
	}

	no, err := p.homeClient.Client().CoreV1().Nodes().Get(context.TODO(), p.nodeName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	return p.updateNode(no)
}

func (p *KubernetesProvider) updateNode(node *v1.Node) error {
	if p.RemoteRemappedPodCidr != "" && node.Status.Allocatable != nil {
		// both the podCIDR and the resources have been set: the node is ready
		for i, condition := range node.Status.Conditions {
			if condition.Type == v1.NodeReady {
				node.Status.Conditions[i].Status = v1.ConditionTrue
			}
			if condition.Type == v1.NodeNetworkUnavailable {
				node.Status.Conditions[i].Status = v1.ConditionFalse
			}
		}
	} else if p.RemoteRemappedPodCidr != "" && node.Status.Allocatable == nil {
		// the resources have not been set yet: set the node status to NotReady
		for i, condition := range node.Status.Conditions {
			if condition.Type == v1.NodeReady {
				node.Status.Conditions[i].Status = v1.ConditionFalse
			}
		}
	} else if p.RemoteRemappedPodCidr == "" && node.Status.Allocatable != nil {
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

func (p *KubernetesProvider) deleteAdv(adv *advtypes.Advertisement) error {
	// delete all reflected resources in reflected namespaces
	for ns := range p.reflectedNamespaces.ns {
		foreignNs, err := p.NatNamespace(ns, false)
		if err != nil {
			klog.Errorf("cannot nat namespace %v", ns)
			return err
		}
		if err := p.cleanupNamespace(foreignNs); err != nil {
			klog.Errorf("error in cleaning up namespace %v", foreignNs)
			return err
		}
	}

	// delete advertisement (which will delete virtual-kubelet)
	if err := p.advClient.Resource("advertisements").Delete(adv.Name, metav1.DeleteOptions{}); err != nil {
		klog.Errorf("Cannot delete advertisement %v", adv.Name)
		return err
	}
	return nil
}
