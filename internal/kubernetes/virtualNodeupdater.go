package kubernetes

import (
	advv1 "github.com/liqoTech/liqo/api/advertisement-operator/v1"
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

	adv, ok := event.Object.(*advv1.Advertisement)
	if !ok {
		klog.Fatal("error in casting advertisement")
	}
	if event.Type == watch.Deleted {
		klog.Infof("advertisement %v deleted...the node is going to be deleted", adv.Name)
		return
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
func (p *KubernetesProvider) initVirtualKubelet(adv advv1.Advertisement) error {
	klog.Info("vk initializing")

	if adv.Status.RemoteRemappedPodCIDR != "None" {
		p.RemappedPodCidr = adv.Status.RemoteRemappedPodCIDR
	} else {
		p.RemappedPodCidr = adv.Spec.Network.PodCIDR
	}

	return nil
}

// updateFromAdv gets and  advertisement and updates the node status accordingly
func (p *KubernetesProvider) updateFromAdv(adv advv1.Advertisement) error {
	var err error

	if !p.initialized {
		if err = p.initVirtualKubelet(adv); err != nil {
			return err
		}
	}

	var no *v1.Node
	if no, err = p.homeClient.Client().CoreV1().Nodes().Get(p.nodeName, metav1.GetOptions{}); err != nil {
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
