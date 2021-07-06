package resourcerequestoperator

import (
	"context"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	resourcehelper "k8s.io/kubectl/pkg/util/resource"

	configv1alpha1 "github.com/liqotech/liqo/apis/config/v1alpha1"
	"github.com/liqotech/liqo/internal/resource-request-operator/interfaces"
	crdclient "github.com/liqotech/liqo/pkg/crdClient"
	"github.com/liqotech/liqo/pkg/utils"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
)

// Broadcaster is an object which is used to get resources of the cluster.
type Broadcaster struct {
	allocatable               corev1.ResourceList
	resourcePodMap            map[string]corev1.ResourceList
	lastReadResources         map[string]corev1.ResourceList
	clusterConfig             configv1alpha1.ClusterConfig
	nodeMutex                 sync.RWMutex
	podMutex                  sync.RWMutex
	configMutex               sync.RWMutex
	nodeInformer              cache.SharedIndexInformer
	podInformer               cache.SharedIndexInformer
	updater                   interfaces.UpdaterInterface
	updateThresholdPercentage uint64
}

// SetupBroadcaster create the informer e run it to signal node changes updating Offers.
func (b *Broadcaster) SetupBroadcaster(clientset kubernetes.Interface, updater interfaces.UpdaterInterface,
	resyncPeriod time.Duration, offerUpdateThreshold uint64) error {
	b.allocatable = corev1.ResourceList{}
	b.updateThresholdPercentage = offerUpdateThreshold
	b.updater = updater
	b.resourcePodMap = map[string]corev1.ResourceList{}
	b.lastReadResources = map[string]corev1.ResourceList{}
	factory := informers.NewSharedInformerFactory(clientset, resyncPeriod)
	b.nodeInformer = factory.Core().V1().Nodes().Informer()
	b.nodeInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    b.onNodeAdd,
		UpdateFunc: b.onNodeUpdate,
		DeleteFunc: b.onNodeDelete,
	})

	b.podInformer = factory.Core().V1().Pods().Informer()
	b.podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    b.onPodAdd,
		DeleteFunc: b.onPodDelete,
	})

	return nil
}

// StartBroadcaster starts two shared Informers, one for nodes and one for pods launching two separated goroutines.
func (b *Broadcaster) StartBroadcaster(ctx context.Context, group *sync.WaitGroup) {
	go b.updater.Start(ctx, group)
	go b.startNodeInformer(ctx, group)
	go b.startPodInformer(ctx, group)
}

func (b *Broadcaster) startNodeInformer(ctx context.Context, group *sync.WaitGroup) {
	group.Add(1)
	defer group.Done()
	b.nodeInformer.Run(ctx.Done())
}

func (b *Broadcaster) startPodInformer(ctx context.Context, group *sync.WaitGroup) {
	group.Add(1)
	defer group.Done()
	b.podInformer.Run(ctx.Done())
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

// GetConfig returns an instance of a ClusterConfig resource.
func (b *Broadcaster) GetConfig() *configv1alpha1.ClusterConfig {
	b.configMutex.RLock()
	defer b.configMutex.RUnlock()
	configCopy := b.clusterConfig.DeepCopy()
	return configCopy
}

// react to a Node Creation/First informer run.
func (b *Broadcaster) onNodeAdd(obj interface{}) {
	node := obj.(*corev1.Node)
	if utils.IsNodeReady(node) {
		klog.V(4).Infof("Adding Node %s\n", node.Name)
		toAdd := &node.Status.Allocatable
		currentResources := b.readClusterResources()
		addResources(currentResources, *toAdd)
		b.writeClusterResources(currentResources)
	}
}

// react to a Node Update.
func (b *Broadcaster) onNodeUpdate(oldObj, newObj interface{}) {
	oldNode := oldObj.(*corev1.Node)
	newNode := newObj.(*corev1.Node)
	oldNodeResources := oldNode.Status.Allocatable
	newNodeResources := newNode.Status.Allocatable
	currentResources := b.readClusterResources()
	klog.V(4).Infof("Updating Node %s in %v\n", oldNode.Name, newNode)
	if utils.IsNodeReady(newNode) {
		// node was already Ready, update with possible new resources.
		if utils.IsNodeReady(oldNode) {
			updateResources(currentResources, oldNodeResources, newNodeResources)
			// node is starting, add all its resources.
		} else {
			addResources(currentResources, newNodeResources)
		}
		// node is terminating or stopping, delete all its resources.
	} else if utils.IsNodeReady(oldNode) && !utils.IsNodeReady(newNode) {
		subResources(currentResources, oldNodeResources)
	}
	b.writeClusterResources(currentResources)
}

// react to a Node Delete.
func (b *Broadcaster) onNodeDelete(obj interface{}) {
	node := obj.(*corev1.Node)
	toDelete := &node.Status.Allocatable
	currentResources := b.readClusterResources()
	if utils.IsNodeReady(node) {
		klog.V(4).Infof("Deleting Node %s\n", node.Name)
		subResources(currentResources, *toDelete)
		b.writeClusterResources(currentResources)
	}
}

func (b *Broadcaster) onPodAdd(obj interface{}) {
	pod := obj.(*corev1.Pod)
	klog.V(4).Infof("OnPodAdd: Add for pod %s:%s\n", pod.Namespace, pod.Name)
	podResources := extractPodResources(pod)
	currentResources := b.readClusterResources()
	// subtract the pod resource from cluster resources. This action is done for all pods to extract actual available resources.
	subResources(currentResources, podResources)
	b.writeClusterResources(currentResources)
	if clusterID := pod.Labels[forge.LiqoOriginClusterID]; clusterID != "" {
		klog.V(4).Infof("OnPodAdd: Pod %s:%s passed ClusterID check. ClusterID = %s\n", pod.Namespace, pod.Name, clusterID)
		currentPodsResources := b.readPodResources(clusterID)
		// add the resource of this pod in the map clusterID => resources to be used in ReadResources() function.
		// this action is done to correct the computation not considering pod offloaded by the cluster with this ClusterID
		addResources(currentPodsResources, podResources)
		b.writePodsResources(clusterID, currentPodsResources)
	}
}

func (b *Broadcaster) onPodDelete(obj interface{}) {
	pod := obj.(*corev1.Pod)
	klog.V(4).Infof("OnPodDelete: Delete for pod %s:%s\n", pod.Namespace, pod.Name)
	podResources := extractPodResources(pod)
	currentResources := b.readClusterResources()
	// this action is the complementary of the onPodAdd() one. For more details see at that function.
	addResources(currentResources, podResources)
	b.writeClusterResources(currentResources)
	if clusterID := pod.Labels[forge.LiqoOriginClusterID]; clusterID != "" {
		klog.V(4).Infof("OnPodDelete: Pod %s:%s passed ClusterID check. ClusterID = %s\n", pod.Namespace, pod.Name, clusterID)
		currentPodsResources := b.readPodResources(clusterID)
		subResources(currentPodsResources, podResources)
		b.writePodsResources(clusterID, currentPodsResources)
	}
}

// write nodes resources in thread safe mode.
func (b *Broadcaster) writeClusterResources(newResources corev1.ResourceList) {
	b.nodeMutex.Lock()
	b.podMutex.RLock()
	defer b.nodeMutex.Unlock()
	defer b.podMutex.RUnlock()
	b.allocatable = newResources.DeepCopy()
	for clusterID := range b.resourcePodMap {
		if b.checkThreshold(clusterID) {
			b.enqueueForCreationOrUpdate(clusterID)
		}
	}
}

// write pods resources in thread safe mode.
func (b *Broadcaster) writePodsResources(clusterID string, newResources corev1.ResourceList) {
	b.podMutex.Lock()
	defer b.podMutex.Unlock()
	b.resourcePodMap[clusterID] = newResources.DeepCopy()
	if b.checkThreshold(clusterID) {
		b.enqueueForCreationOrUpdate(clusterID)
	}
}

// ReadResources return in thread safe mode a scaled value of the resources.
func (b *Broadcaster) ReadResources(clusterID string) corev1.ResourceList {
	toRead := b.readClusterResources()
	podsResources := b.readPodResources(clusterID)
	addResources(toRead, podsResources)
	b.lastReadResources[clusterID] = toRead
	for resourceName, quantity := range toRead {
		scaled := quantity
		b.scaleResources(resourceName, &scaled)
		toRead[resourceName] = scaled
	}
	return toRead
}

func (b *Broadcaster) enqueueForCreationOrUpdate(clusterID string) {
	b.updater.Push(clusterID)
}

// RemoveClusterID removes a clusterID from all broadcaster internal structures
// it is useful when a particular foreign cluster has no more peering and its ResourceRequest has been deleted.
func (b *Broadcaster) RemoveClusterID(clusterID string) {
	b.podMutex.Lock()
	defer b.podMutex.Unlock()
	delete(b.resourcePodMap, clusterID)
	delete(b.lastReadResources, clusterID)
}

func (b *Broadcaster) readClusterResources() corev1.ResourceList {
	b.nodeMutex.RLock()
	defer b.nodeMutex.RUnlock()
	return b.allocatable.DeepCopy()
}

func (b *Broadcaster) readPodResources(clusterID string) corev1.ResourceList {
	b.podMutex.RLock()
	defer b.podMutex.RUnlock()
	if toRead, exists := b.resourcePodMap[clusterID]; exists {
		return toRead.DeepCopy()
	}
	return corev1.ResourceList{}
}

func (b *Broadcaster) checkThreshold(clusterID string) bool {
	for resourceName, resources := range b.resourcePodMap[clusterID] {
		clusterResources := b.allocatable[resourceName]
		lastRead := b.lastReadResources[clusterID][resourceName]
		clusterResourcesValue := clusterResources.Value()
		resourcePodValue := resources.Value()
		lastReadValue := lastRead.Value()
		diff := (clusterResourcesValue + resourcePodValue) - lastReadValue
		if diff > lastReadValue*int64(b.updateThresholdPercentage)/100 {
			return true
		}
	}

	return false
}

func (b *Broadcaster) scaleResources(resourceName corev1.ResourceName, quantity *resource.Quantity) {
	percentage := int64(b.GetConfig().Spec.AdvertisementConfig.OutgoingConfig.ResourceSharingPercentage)

	switch resourceName {
	case corev1.ResourceCPU:
		// use millis
		quantity.SetScaled(quantity.MilliValue()*percentage/100, resource.Milli)
	case corev1.ResourceMemory:
		// use mega
		quantity.SetScaled(quantity.ScaledValue(resource.Mega)*percentage/100, resource.Mega)
	default:
		quantity.Set(quantity.Value() * percentage / 100)
	}
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

func extractPodResources(pod *corev1.Pod) corev1.ResourceList {
	resourcesToExtract, _ := resourcehelper.PodRequestsAndLimits(pod)
	return resourcesToExtract
}
