// Copyright 2019-2022 The Liqo Authors
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

package namespacemap

import (
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	vkv1alpha1 "github.com/liqotech/liqo/apis/virtualkubelet/v1alpha1"
	liqoclient "github.com/liqotech/liqo/pkg/client/clientset/versioned"
	liqoinformers "github.com/liqotech/liqo/pkg/client/informers/externalversions"
	vkv1alpha1listers "github.com/liqotech/liqo/pkg/client/listers/virtualkubelet/v1alpha1"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/manager"
)

// Handler implements the logic to start and stop the reflection of resources.
type Handler struct {
	lister          vkv1alpha1listers.NamespaceMapNamespaceLister
	informerFactory liqoinformers.SharedInformerFactory

	namespaceStartStopper manager.NamespaceStartStopper
}

// NewHandler creates a new NamespaceMapEventHandler.
func NewHandler(homeLiqoClient liqoclient.Interface, namespace string, resyncPeriod time.Duration) *Handler {
	localLiqoNamespaceMapTweakListOptions := func(opts *metav1.ListOptions) {
		opts.LabelSelector = labels.Set(map[string]string{liqoconst.RemoteClusterID: forge.RemoteClusterID}).String()
	}
	localLiqoInformerFactory := liqoinformers.NewSharedInformerFactoryWithOptions(homeLiqoClient, resyncPeriod,
		liqoinformers.WithNamespace(namespace),
		liqoinformers.WithTweakListOptions(localLiqoNamespaceMapTweakListOptions))

	return &Handler{
		informerFactory: localLiqoInformerFactory,
		lister:          localLiqoInformerFactory.Virtualkubelet().V1alpha1().NamespaceMaps().Lister().NamespaceMaps(namespace),
	}
}

// Start adds the handler to the informer, starts the informer, and waits for chache sync.
func (nh *Handler) Start(ctx context.Context, namespaceStartStopper manager.NamespaceStartStopper) {
	klog.Info("Starting the namespaceMap handler...")

	nh.namespaceStartStopper = namespaceStartStopper

	eh := cache.FilteringResourceEventHandler{
		FilterFunc: nh.checkNamespaceMapUniqueness,
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    nh.onAddNamespaceMap,
			UpdateFunc: nh.onUpdateNamespaceMap,
			DeleteFunc: nh.onDeleteNamespaceMap,
		},
	}
	nh.informerFactory.Virtualkubelet().V1alpha1().NamespaceMaps().Informer().AddEventHandler(eh)

	nh.informerFactory.Start(ctx.Done())
	nh.informerFactory.WaitForCacheSync(ctx.Done())

	klog.Info("namespaceMap handler started")
}

func (nh *Handler) onAddNamespaceMap(obj interface{}) {
	namespaceMap := obj.(*vkv1alpha1.NamespaceMap)

	for localNs, remoteNamespaceStatus := range namespaceMap.Status.CurrentMapping {
		nh.startNamespace(localNs, remoteNamespaceStatus)
	}
}

func (nh *Handler) onDeleteNamespaceMap(obj interface{}) {
	namespaceMap := obj.(*vkv1alpha1.NamespaceMap)

	for localNs, remoteNamespaceStatus := range namespaceMap.Status.CurrentMapping {
		nh.stopNamespace(localNs, remoteNamespaceStatus)
	}
}

func (nh *Handler) onUpdateNamespaceMap(oldObj, newObj interface{}) {
	oldNamespaceMap := oldObj.(*vkv1alpha1.NamespaceMap)
	newNamespaceMap := newObj.(*vkv1alpha1.NamespaceMap)

	// Stop namespaces that are in the old NamespaceMap and:
	// - Are not in the new NamespaceMap.
	// - Are not in the new NamespaceMap but they have just transitioned from MappingAccepted phase to another phase.
	for localNs, oldRemoteNamespaceStatus := range oldNamespaceMap.Status.CurrentMapping {
		newRemoteNamespaceStatus, newRemoteNamespaceStatusFound := newNamespaceMap.Status.CurrentMapping[localNs]
		if !newRemoteNamespaceStatusFound || newRemoteNamespaceStatus.Phase != vkv1alpha1.MappingAccepted {
			nh.stopNamespace(localNs, oldRemoteNamespaceStatus)
		}
	}

	// Start namespaces that are in the new NamespaceMap and:
	// - Are in the new NamespaceMap but not in the oldNamespaceMap.
	// - Are in the old NamespaceMap but they have just transitioned to MappingAccepted phase.
	for localNs, newRemoteNamespaceStatus := range newNamespaceMap.Status.CurrentMapping {
		oldRemoteNamespaceStatus, oldRemoteNamespaceStatusFound := oldNamespaceMap.Status.CurrentMapping[localNs]
		if !oldRemoteNamespaceStatusFound || oldRemoteNamespaceStatus.Phase != vkv1alpha1.MappingAccepted {
			nh.startNamespace(localNs, newRemoteNamespaceStatus)
		}
	}
}

func (nh *Handler) checkNamespaceMapUniqueness(_ interface{}) bool {
	nsList, err := nh.lister.List(labels.Everything())
	utilruntime.Must(err)

	if nNamespaceMaps := len(nsList); nNamespaceMaps > 1 {
		klog.Errorf("Listing NamespaceMap resources returned %d results: NamespaceMap expected to be unique", nNamespaceMaps)
		return false
	}

	return true
}

func (nh *Handler) startNamespace(localNs string, remoteNamespaceStatus vkv1alpha1.RemoteNamespaceStatus) {
	if remoteNamespaceStatus.Phase != vkv1alpha1.MappingAccepted {
		return
	}

	remoteNs := remoteNamespaceStatus.RemoteNamespace
	klog.V(3).Infof("Enabling reflection for remote namespace %s for local namespace %s", remoteNs, localNs)
	nh.namespaceStartStopper.StartNamespace(localNs, remoteNs)
}

func (nh *Handler) stopNamespace(localNs string, remoteNamespaceStatus vkv1alpha1.RemoteNamespaceStatus) {
	if remoteNamespaceStatus.Phase != vkv1alpha1.MappingAccepted {
		return
	}

	remoteNs := remoteNamespaceStatus.RemoteNamespace
	klog.V(3).Infof("Stopping reflection for remote namespace %s for local namespace %s", remoteNs, localNs)
	nh.namespaceStartStopper.StopNamespace(localNs, remoteNs)
}
