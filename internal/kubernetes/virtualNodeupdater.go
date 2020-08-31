package kubernetes

import (
	"context"
	advtypes "github.com/liqoTech/liqo/api/sharing/v1alpha1"
	advop "github.com/liqoTech/liqo/internal/advertisement-operator"
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
	c, err := p.nodeUpdateClient.Resource("advertisements").Watch(metav1.ListOptions{
		FieldSelector: strings.Join([]string{"metadata.name", advName}, "="),
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
			case ev := <-c.ResultChan():
				p.ReconcileNodeFromAdv(ev)
			case <-stop:
				c.Stop()
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

	if adv.Status.AdvertisementStatus == advop.AdvertisementDeleting {
		for retry := 0; retry < 3; retry++ {
			klog.Infof("advertisement %v is going to be deleted... set node status not ready", adv.Name)
			no, err := p.nodeUpdateClient.Client().CoreV1().Nodes().Get(context.TODO(), p.nodeName, metav1.GetOptions{})
			if err != nil {
				klog.Error(err)
				continue
			}
			for _, condition := range no.Status.Conditions {
				if condition.Type == v1.NodeReady {
					no.Status.Conditions[0].Status = v1.ConditionFalse
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
		}
		klog.Errorf("node update from advertisement %v failed, trying again...", adv.Name)
	}
}

// Initialization of the Virtual kubelet, that implies:
func (p *KubernetesProvider) initVirtualKubelet(adv advtypes.Advertisement) error {
	klog.Info("vk initializing")

	if adv.Status.RemoteRemappedPodCIDR != "None" {
		p.RemappedPodCidr = adv.Status.RemoteRemappedPodCIDR
	} else {
		p.RemappedPodCidr = adv.Spec.Network.PodCIDR
	}

	return nil
}

// updateFromAdv gets and  advertisement and updates the node status accordingly
func (p *KubernetesProvider) updateFromAdv(adv advtypes.Advertisement) error {
	var err error

	if !p.initialized {
		if err = p.initVirtualKubelet(adv); err != nil {
			return err
		}
	}

	var no *v1.Node
	if no, err = p.homeClient.Client().CoreV1().Nodes().Get(context.TODO(), p.nodeName, metav1.GetOptions{}); err != nil {
		return err
	}

	if !p.initialized {
		p.initialized = true
		no.SetAnnotations(map[string]string{
			"cluster-id": p.foreignClusterId,
		})
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

	no.Status.Images = []v1.ContainerImage{}
	no.Status.Images = append(no.Status.Images, adv.Spec.Images...)

	return p.nodeController.UpdateNodeFromOutside(false, no)
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
	if err := p.nodeUpdateClient.Resource("advertisements").Delete(adv.Name, metav1.DeleteOptions{}); err != nil {
		klog.Errorf("Cannot delete advertisement %v", adv.Name)
		return err
	}
	return nil
}
