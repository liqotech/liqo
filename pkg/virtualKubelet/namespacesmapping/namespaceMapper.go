// Copyright 2019-2021 The Liqo Authors
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

package namespacesmapping

import (
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	vkalpha1 "github.com/liqotech/liqo/apis/virtualKubelet/v1alpha1"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
)

const namespaceMapEntryNotAvailable = ""

// NamespaceMapper embeds data and clients for namespace mapping.
type NamespaceMapper struct {
	client                 dynamic.Interface
	lister                 cache.GenericLister
	Controller             chan struct{}
	informer               cache.SharedIndexInformer
	foreignClusterID       string
	homeClusterID          string
	namespaceReadyMapCache namespaceReadyMapCache
	namespace              string

	startOutgoingReflection chan string
	startIncomingReflection chan string
	stopOutgoingReflection  chan string
	stopIncomingReflection  chan string
	startMapper             chan struct{}
	stopMapper              chan struct{}
	restartReady            chan struct{}
}

func (m *NamespaceMapper) init(ctx context.Context, config *rest.Config) error {
	var err error

	m.client, err = dynamic.NewForConfig(config)
	if err != nil {
		return err
	}

	m.namespaceReadyMapCache.mappings = map[string]string{}

	gvr := vkalpha1.NamespaceMapGroupVersionResource

	ehf := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			m.startMapper <- struct{}{}
			m.manageReflections(nil, obj)
		},
		UpdateFunc: m.manageReflections,
		DeleteFunc: func(obj interface{}) {
			m.manageReflections(obj, nil)
			m.stopMapper <- struct{}{}
			<-m.restartReady
		},
	}

	informerFactory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(m.client, 10*time.Minute, m.namespace, func(options *metav1.ListOptions) {
		options.LabelSelector = labels.Set(map[string]string{liqoconst.RemoteClusterID: m.foreignClusterID}).String()
	})
	m.informer = informerFactory.ForResource(gvr).Informer()
	m.lister = informerFactory.ForResource(gvr).Lister()
	m.informer.AddEventHandler(ehf)
	go m.informer.Run(ctx.Done())

	return nil
}

// checkMapUniqueness checks that the NamespaceMap present on the cluster is unique.
func (m *NamespaceMapper) checkMapUniqueness() bool {
	var labelSelector = labels.Set(map[string]string{liqoconst.RemoteClusterID: m.foreignClusterID}).AsSelector()
	ret, err := m.lister.ByNamespace(m.namespace).List(labelSelector)
	if err != nil {
		return false
	}
	if len(ret) > 1 {
		return false
	}
	return true
}

// HomeToForeignNamespace returns the foreign namespace name associated to a local namespaceName reading from the namespaceReadyMapCache.
// It returns an error if the namespace is not found.
func (m *NamespaceMapper) HomeToForeignNamespace(namespaceName string) (string, error) {
	currentMapping := m.namespaceReadyMapCache.read(namespaceName)
	if currentMapping == namespaceMapEntryNotAvailable {
		return namespaceMapEntryNotAvailable, &namespaceNotAvailable{
			namespaceName: namespaceName,
		}
	}
	return currentMapping, nil
}

// ForeignToLocalNamespace returns the local namespace name associated to a remote namespaceName reading from the namespaceReadyMapCache.
// It returns an error if the namespace is not found.
func (m *NamespaceMapper) ForeignToLocalNamespace(namespaceName string) (string, error) {
	currentMapping := m.namespaceReadyMapCache.inverseRead(namespaceName)
	if currentMapping == namespaceMapEntryNotAvailable {
		return namespaceMapEntryNotAvailable, &namespaceNotAvailable{
			namespaceName: namespaceName,
		}
	}

	return currentMapping, nil
}

// MappedNamespaces returns the whole namespaceReadyMapCache map.
func (m *NamespaceMapper) MappedNamespaces() map[string]string {
	return m.namespaceReadyMapCache.readAll()
}

// manageReflections handles updates in namespaceMap. It adds/removes new mappings to the namespaceReadyMapCache and
// starts/stop outgoing/incoming reflection routines by comparing oldNamespaceMapObject and newNamespaceMapObject.
func (m *NamespaceMapper) manageReflections(oldNamespaceMapObject, newNamespaceMapObject interface{}) {
	if !m.checkMapUniqueness() {
		return
	}
	var oldNamespaceMap, newNamespaceMap vkalpha1.NamespaceMap
	if newNamespaceMapObject != nil {
		err := runtime.DefaultUnstructuredConverter.
			FromUnstructured(newNamespaceMapObject.(*unstructured.Unstructured).Object, &newNamespaceMap)
		if err != nil {
			return
		}
	}

	// If it is not a create event, we have to check if old namespaces should be removed
	if oldNamespaceMapObject != nil {
		err := runtime.DefaultUnstructuredConverter.
			FromUnstructured(oldNamespaceMapObject.(*unstructured.Unstructured).Object, &oldNamespaceMap)
		if err != nil {
			return
		}
		m.handleMapperDeletions(&oldNamespaceMap, &newNamespaceMap, newNamespaceMapObject == nil)
	}

	// If it is a delete event, no need to check for new additions
	if newNamespaceMapObject != nil {
		m.handleMapperAdditions(&oldNamespaceMap, &newNamespaceMap, oldNamespaceMapObject == nil)
	}
}

// WaitNamespaceNattingTableSync waits until internal caches are synchronized.
func (m *NamespaceMapper) WaitNamespaceNattingTableSync(ctx context.Context) {
	cache.WaitForCacheSync(ctx.Done(), m.informer.HasSynced)
}

func (m *NamespaceMapper) handleMapperAdditions(oldNamespaceMap, newNamespaceMap *vkalpha1.NamespaceMap, creationEvent bool) {
	for localNs, newMapping := range newNamespaceMap.Status.CurrentMapping {
		// If the newMapping resource is not Accepted, the namespace reflection should not be enabled. So, no need to process this item.
		if newMapping.Phase != vkalpha1.MappingAccepted {
			continue
		}
		// When it is a creation event and is Accepted, we always have to add the namespace.
		if creationEvent {
			klog.V(3).Infof("Enabling reflection for remote namespace %s for local namespace %s", newMapping.RemoteNamespace, localNs)
			m.namespaceReadyMapCache.write(localNs, newMapping.RemoteNamespace)
			m.startOutgoingReflection <- localNs
			m.startIncomingReflection <- localNs
			// When there is an update event, we add to the cache if the element was not present before or if we had a change to MappingAccepted.
		} else if oldRemoteNs, ok := oldNamespaceMap.Status.CurrentMapping[localNs]; !ok || oldRemoteNs.Phase != vkalpha1.MappingAccepted {
			klog.V(3).Infof("Enabling reflection for remote namespace %s for local namespace %s", newMapping.RemoteNamespace, localNs)
			m.namespaceReadyMapCache.write(localNs, newMapping.RemoteNamespace)
			m.startOutgoingReflection <- localNs
			m.startIncomingReflection <- localNs
		}
	}
}

func (m *NamespaceMapper) handleMapperDeletions(oldNamespaceMap, newNamespaceMap *vkalpha1.NamespaceMap, deletionEvent bool) {
	for localNs, oldMapping := range oldNamespaceMap.Status.CurrentMapping {
		// If the old resource was not Accepted, the namespace reflection was not enabled. So, no need to process this item
		if oldMapping.Phase != vkalpha1.MappingAccepted {
			continue
		}
		if deletionEvent {
			klog.V(3).Infof("Stopping reflection for remote namespace %s for local namespace %s", oldMapping.RemoteNamespace, localNs)
			m.stopOutgoingReflection <- localNs
			m.stopIncomingReflection <- localNs
		} else if newRemoteNs, ok := newNamespaceMap.Status.CurrentMapping[localNs]; !ok || newRemoteNs.Phase != vkalpha1.MappingAccepted {
			klog.V(3).Infof("Stopping reflection for remote namespace %s for local namespace %s", oldMapping.RemoteNamespace, localNs)
			m.stopOutgoingReflection <- localNs
			m.stopIncomingReflection <- localNs
		}
	}
}
