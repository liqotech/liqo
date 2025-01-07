// Copyright 2019-2025 The Liqo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package reflection

import (
	"context"
	"strconv"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	"github.com/liqotech/liqo/internal/crdReplicator/resources"
	"github.com/liqotech/liqo/pkg/consts"
)

// Manager represents an object creating reflectors towards remote clusters.
type Manager struct {
	client dynamic.Interface
	resync time.Duration

	listers       map[schema.GroupVersionResource]cache.GenericLister
	handlers      map[schema.GroupVersionResource]map[string]func(key item)
	handlersMutex sync.RWMutex

	clusterID liqov1beta1.ClusterID
	workers   uint
}

// NewManager returns a new manager to start the reflection towards remote clusters.
func NewManager(client dynamic.Interface, clusterID liqov1beta1.ClusterID, workersPerCluster uint, resync time.Duration) *Manager {
	return &Manager{
		client: client,
		resync: resync,

		listers:  make(map[schema.GroupVersionResource]cache.GenericLister),
		handlers: make(map[schema.GroupVersionResource]map[string]func(key item)),

		clusterID: clusterID,
		workers:   workersPerCluster,
	}
}

// Start starts the manager registering the given resources.
func (m *Manager) Start(ctx context.Context, registeredResources []resources.Resource) {
	tweakListOptions := func(opts *metav1.ListOptions) { opts.LabelSelector = m.localLabelSelector().String() }
	factory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(m.client, m.resync, metav1.NamespaceAll, tweakListOptions)

	// Configure the informer for all resources.
	for _, resource := range registeredResources {
		gvr := resource.GroupVersionResource
		klog.Infof("Configuring local informer for %v", gvr)
		informer := factory.ForResource(gvr)
		informer.Informer().AddEventHandler(m.eventHandlers(gvr))
		m.listers[gvr] = informer.Lister()
	}

	klog.Infof("Starting the local informer factory")
	factory.Start(ctx.Done())
	factory.WaitForCacheSync(ctx.Done())
	klog.Infof("Local informer factory synced correctly")
}

// NewForRemote returns a new reflector for a given remote cluster.
func (m *Manager) NewForRemote(client dynamic.Interface, clusterID liqov1beta1.ClusterID, localNamespace, remoteNamespace string,
	secretHash string) *Reflector {
	return &Reflector{
		manager: m,

		localNamespace: localNamespace,
		localClusterID: m.clusterID,

		remoteClient:    client,
		remoteNamespace: remoteNamespace,
		remoteClusterID: clusterID,

		secretHash: secretHash,

		resources: make(map[schema.GroupVersionResource]*reflectedResource),
		workqueue: workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
	}
}

// registerHandler registers the handler for a given GroupVersionResource and namespace.
func (m *Manager) registerHandler(gvr schema.GroupVersionResource, namespace string, handler func(key item)) {
	// Add the handler to the list of known ones.
	m.handlersMutex.Lock()
	m.handlers[gvr][namespace] = handler
	m.handlersMutex.Unlock()

	// Iterate over all elements already existing, and trigger the handler
	objects, err := m.listers[gvr].ByNamespace(namespace).List(labels.Everything())
	utilruntime.Must(err)

	for i := range objects {
		metadata, err := meta.Accessor(objects[i])
		utilruntime.Must(err)
		handler(item{gvr: gvr, name: metadata.GetName()})
	}
}

// unregisterHandler unregisters the handler for a given GroupVersionResource and namespace.
func (m *Manager) unregisterHandler(gvr schema.GroupVersionResource, namespace string) {
	m.handlersMutex.Lock()
	delete(m.handlers[gvr], namespace)
	m.handlersMutex.Unlock()
}

// eventHandlers returns the event handlers which add the elements of a given GroupVersionResource to the working queue.
func (m *Manager) eventHandlers(gvr schema.GroupVersionResource) cache.ResourceEventHandlerFuncs {
	m.handlers[gvr] = make(map[string]func(key item))

	eh := func(obj interface{}) {
		unstruct := obj.(*unstructured.Unstructured)
		m.handlersMutex.RLock()
		defer m.handlersMutex.RUnlock()
		if handle, found := m.handlers[gvr][unstruct.GetNamespace()]; found {
			handle(item{gvr: gvr, name: unstruct.GetName()})
		}
	}

	return cache.ResourceEventHandlerFuncs{
		AddFunc:    eh,
		UpdateFunc: func(_, obj interface{}) { eh(obj) },
		DeleteFunc: eh,
	}
}

// localLabelSelector returns a function which configures the label selector targeting the resources
// in the local cluster to be replicated.
func (m *Manager) localLabelSelector() labels.Selector {
	req1, err := labels.NewRequirement(consts.ReplicationRequestedLabel, selection.Equals, []string{strconv.FormatBool(true)})
	utilruntime.Must(err)
	req2, err := labels.NewRequirement(consts.ReplicationDestinationLabel, selection.Exists, []string{})
	utilruntime.Must(err)
	return labels.NewSelector().Add(*req1, *req2)
}

// LocalResourcesLabelSelector is an helper function which returns a label selector to list all the local resources to be replicated.
func LocalResourcesLabelSelector() metav1.LabelSelector {
	return metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{
			{
				Key:      consts.ReplicationRequestedLabel,
				Operator: metav1.LabelSelectorOpIn,
				Values:   []string{strconv.FormatBool(true)},
			},
			{
				Key:      consts.ReplicationDestinationLabel,
				Operator: metav1.LabelSelectorOpExists,
			},
		},
	}
}
