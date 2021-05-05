package resourcerequestoperator

import (
	"context"
	"fmt"
	"sync"
	"time"

	crdclient "github.com/liqotech/liqo/pkg/crdClient"

	configv1alpha1 "github.com/liqotech/liqo/apis/config/v1alpha1"
	"github.com/liqotech/liqo/pkg/utils"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"
)

// Broadcaster is an object which is used to get resources of the cluster.
type Broadcaster struct {
	allocatable   corev1.ResourceList
	clusterConfig configv1alpha1.ClusterConfig
	offerMutex    sync.RWMutex
	configMutex   sync.RWMutex
	informer      cache.SharedInformer
}

// SetupBroadcaster create the informer e run it to signal node changes updating Offers.
func (b *Broadcaster) SetupBroadcaster(clientset *kubernetes.Clientset, resyncPeriod time.Duration) error {
	b.allocatable = corev1.ResourceList{}
	factory := informers.NewSharedInformerFactory(clientset, resyncPeriod)
	b.informer = factory.Core().V1().Nodes().Informer()
	b.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    b.onAdd,
		UpdateFunc: b.onUpdate,
		DeleteFunc: b.onDelete,
	})

	return nil
}

// StartBroadcaster starts a new sharedInformer to watch nodes resources.
func (b *Broadcaster) StartBroadcaster(ctx context.Context, group *sync.WaitGroup) {
	defer group.Done()
	b.informer.Run(ctx.Done())
}

// WatchConfiguration starts a new watcher to get clusterConfig.
func (b *Broadcaster) WatchConfiguration(localKubeconfig string, crdClient *crdclient.CRDClient, wg *sync.WaitGroup) {
	defer wg.Done()
	utils.WatchConfiguration(b.setConfig, crdClient, localKubeconfig)
}

func (b *Broadcaster) setConfig(configuration *configv1alpha1.ClusterConfig) {
	b.configMutex.Lock()
	defer b.configMutex.Unlock()
	b.clusterConfig = *configuration
}

func (b *Broadcaster) getConfig() *configv1alpha1.ClusterConfig {
	b.configMutex.RLock()
	defer b.configMutex.RUnlock()
	configCopy := b.clusterConfig.DeepCopy()
	return configCopy
}

// react to a Node Creation/First informer run.
func (b *Broadcaster) onAdd(obj interface{}) {
	node := obj.(*corev1.Node)
	if node.Status.Phase == corev1.NodeRunning {
		toAdd := &node.Status.Allocatable
		currentResources := b.allocatable.DeepCopy()
		addResources(currentResources, *toAdd)

		if err := b.writeResources(currentResources); err != nil {
			klog.Errorf("OnAdd error: unable to write allocatable of Node %s: %s", node.Name, err)
		}
	}
}

// react to a Node Update.
func (b *Broadcaster) onUpdate(oldObj, newObj interface{}) {
	oldNode := oldObj.(*corev1.Node)
	newNode := newObj.(*corev1.Node)
	oldNodeResources := oldNode.Status.Allocatable
	newNodeResources := newNode.Status.Allocatable
	currentResources := b.allocatable.DeepCopy()
	if newNode.Status.Phase == corev1.NodeRunning {
		// node was already Running, update with possible new resources.
		if oldNode.Status.Phase == corev1.NodeRunning {
			updateResources(currentResources, oldNodeResources, newNodeResources)
			// node is starting, add all its resources.
		} else if oldNode.Status.Phase == corev1.NodePending || oldNode.Status.Phase == corev1.NodeTerminated {
			addResources(currentResources, newNodeResources)
		}
		// node is terminating or stopping, delete all its resources.
	} else if oldNode.Status.Phase == corev1.NodeRunning &&
		(newNode.Status.Phase == corev1.NodeTerminated || newNode.Status.Phase == corev1.NodePending) {
		subResources(currentResources, oldNodeResources)
	}
	if err := b.writeResources(currentResources); err != nil {
		klog.Errorf("OnUpdate error: unable to write allocatable of Node %s: %s", newNode.Name, err)
	}
}

// react to a Node Delete.
func (b *Broadcaster) onDelete(obj interface{}) {
	node := obj.(*corev1.Node)
	toDelete := &node.Status.Allocatable
	currentResources := b.allocatable.DeepCopy()
	if node.Status.Phase == corev1.NodeRunning {
		subResources(currentResources, *toDelete)
		if err := b.writeResources(currentResources); err != nil {
			klog.Errorf("OnAdd error: unable to write allocatable of Node %s: %s", node.Name, err)
		}
	}
}

// write cluster resources in thread safe mode.
func (b *Broadcaster) writeResources(newResources corev1.ResourceList) error {
	b.offerMutex.Lock()
	defer b.offerMutex.Unlock()
	if newResources != nil {
		b.allocatable = newResources.DeepCopy()
		return nil
	}

	return fmt.Errorf("some error occurred during cluster resources read. Attempting writing nil resources")
}

// ReadResources return in thread safe mode a scaled value of the resources.
func (b *Broadcaster) ReadResources() (corev1.ResourceList, error) {
	b.offerMutex.RLock()
	defer b.offerMutex.RUnlock()
	if b.allocatable == nil {
		return nil, fmt.Errorf("error getting cluster resources")
	}
	toRead := b.allocatable.DeepCopy()
	for resourceName, quantity := range toRead {
		scaled := quantity
		b.scaleResources(&scaled)
		toRead[resourceName] = scaled
	}
	return toRead, nil
}

func (b *Broadcaster) scaleResources(quantity *resource.Quantity) {
	percentage := int64(b.getConfig().Spec.AdvertisementConfig.OutgoingConfig.ResourceSharingPercentage)
	if percentage == 0 {
		return
	}

	quantity.Set(quantity.Value() * percentage / 100)
}

// addResources is an utility function to add resources.
func addResources(currentResources, toAdd corev1.ResourceList) {
	for resourceName, quantity := range toAdd {
		if value, exists := currentResources[resourceName]; exists {
			value.Add(quantity)
			currentResources[resourceName] = value
		} else {
			currentResources[resourceName] = quantity
		}
	}
}

// subResources is an utility function to subtract resources.
func subResources(currentResources, toSub corev1.ResourceList) {
	for resourceName, quantity := range toSub {
		if value, exists := currentResources[resourceName]; exists {
			value.Sub(quantity)
			currentResources[resourceName] = value
		}
	}
}

// updateResources is an utility function to update resources.
func updateResources(currentResources, oldResources, newResources corev1.ResourceList) {
	for resourceName, quantity := range newResources {
		if oldQuantity, exists := oldResources[resourceName]; exists {
			value := currentResources[resourceName]
			quantityToUpdate := resource.NewQuantity(quantity.Value()-oldQuantity.Value(),
				quantity.Format)
			value.Add(*quantityToUpdate)
			currentResources[resourceName] = value
		} else {
			currentResources[resourceName] = quantity
		}
	}
}
