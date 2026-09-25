// Copyright 2019-2026 The Liqo Authors
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
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	liqoclient "github.com/liqotech/liqo/pkg/client/clientset/versioned"
	liqoinformers "github.com/liqotech/liqo/pkg/client/informers/externalversions"
	offloadingv1beta1listers "github.com/liqotech/liqo/pkg/client/listers/offloading/v1beta1"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/manager"
)

// Handler implements the logic to start and stop the reflection of resources.
type Handler struct {
	lister          offloadingv1beta1listers.NamespaceMapNamespaceLister
	informerFactory liqoinformers.SharedInformerFactory

	namespaceStartStopper manager.NamespaceStartStopper
}

// NewHandler creates a new NamespaceMapEventHandler.
func NewHandler(localLiqoClient liqoclient.Interface, namespace string, resyncPeriod time.Duration) *Handler {
	localLiqoNamespaceMapTweakListOptions := func(opts *metav1.ListOptions) {
		opts.LabelSelector = labels.Set(map[string]string{liqoconst.RemoteClusterID: string(forge.RemoteCluster)}).String()
	}
	localLiqoInformerFactory := liqoinformers.NewSharedInformerFactoryWithOptions(localLiqoClient, resyncPeriod,
		liqoinformers.WithNamespace(namespace),
		liqoinformers.WithTweakListOptions(localLiqoNamespaceMapTweakListOptions))

	return &Handler{
		informerFactory: localLiqoInformerFactory,
		lister:          localLiqoInformerFactory.Offloading().V1beta1().NamespaceMaps().Lister().NamespaceMaps(namespace),
	}
}

// registrationSyncTimeout is the maximum time to wait for the handler registration to be synced,
// i.e., for the initial events to be delivered, after the informer caches are synced.
const registrationSyncTimeout = 30 * time.Second

// Start adds the handler to the informer, starts the informer, and waits for chache sync.
// It returns an error in case the registered handler failed to receive the initial events within
// the timeout, as continuing would leave the namespace reflection in an undefined state.
func (nh *Handler) Start(ctx context.Context, namespaceStartStopper manager.NamespaceStartStopper) error {
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
	registration, err := nh.informerFactory.Offloading().V1beta1().NamespaceMaps().Informer().AddEventHandler(eh)
	utilruntime.Must(err)

	nh.informerFactory.Start(ctx.Done())
	nh.informerFactory.WaitForCacheSync(ctx.Done())

	// The factory-level cache sync only guarantees that the informer store is populated, but not that the
	// event handler has been invoked for the pre-existing resources. Wait also for the registration to be
	// synced, i.e. for the initial events to be delivered, so that StartNamespace is guaranteed to have been
	// invoked for all accepted mappings before the reflection manager is marked as ready. Otherwise, pods of
	// managed namespaces could be spuriously processed by the fallback reflectors and wrongly rejected.
	regCtx, cancel := context.WithTimeout(ctx, registrationSyncTimeout)
	defer cancel()
	if !cache.WaitForCacheSync(regCtx.Done(), registration.HasSynced) {
		// Do not treat an orderly shutdown as an error.
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("timed out waiting for the namespaceMap handler registration to sync")
	}

	klog.Info("namespaceMap handler started")
	return nil
}

func (nh *Handler) onAddNamespaceMap(obj interface{}) {
	namespaceMap := obj.(*offloadingv1beta1.NamespaceMap)

	for localNs, remoteNamespaceStatus := range namespaceMap.Status.CurrentMapping {
		nh.startNamespace(localNs, remoteNamespaceStatus)
	}
}

func (nh *Handler) onDeleteNamespaceMap(obj interface{}) {
	namespaceMap := obj.(*offloadingv1beta1.NamespaceMap)

	for localNs, remoteNamespaceStatus := range namespaceMap.Status.CurrentMapping {
		nh.stopNamespace(localNs, remoteNamespaceStatus)
	}
}

func (nh *Handler) onUpdateNamespaceMap(oldObj, newObj interface{}) {
	oldNamespaceMap := oldObj.(*offloadingv1beta1.NamespaceMap)
	newNamespaceMap := newObj.(*offloadingv1beta1.NamespaceMap)

	// Stop namespaces that are in the old NamespaceMap and:
	// - Are not in the new NamespaceMap.
	// - Are not in the new NamespaceMap but they have just transitioned from MappingAccepted phase to another phase.
	for localNs, oldRemoteNamespaceStatus := range oldNamespaceMap.Status.CurrentMapping {
		newRemoteNamespaceStatus, newRemoteNamespaceStatusFound := newNamespaceMap.Status.CurrentMapping[localNs]
		if !newRemoteNamespaceStatusFound || newRemoteNamespaceStatus.Phase != offloadingv1beta1.MappingAccepted {
			nh.stopNamespace(localNs, oldRemoteNamespaceStatus)
		}
	}

	// Start namespaces that are in the new NamespaceMap and:
	// - Are in the new NamespaceMap but not in the oldNamespaceMap.
	// - Are in the old NamespaceMap but they have just transitioned to MappingAccepted phase.
	for localNs, newRemoteNamespaceStatus := range newNamespaceMap.Status.CurrentMapping {
		oldRemoteNamespaceStatus, oldRemoteNamespaceStatusFound := oldNamespaceMap.Status.CurrentMapping[localNs]
		if !oldRemoteNamespaceStatusFound || oldRemoteNamespaceStatus.Phase != offloadingv1beta1.MappingAccepted {
			nh.startNamespace(localNs, newRemoteNamespaceStatus)
		}
	}
}

func (nh *Handler) checkNamespaceMapUniqueness(_ interface{}) bool {
	nsList, err := nh.lister.List(labels.SelectorFromSet(labels.Set{
		liqoconst.RemoteClusterID:             string(forge.RemoteCluster),
		liqoconst.ReplicationDestinationLabel: string(forge.RemoteCluster),
	}))
	utilruntime.Must(err)

	if nNamespaceMaps := len(nsList); nNamespaceMaps > 1 {
		klog.Errorf("Listing NamespaceMap resources returned %d results: NamespaceMap expected to be unique", nNamespaceMaps)
		return false
	}

	return true
}

// IsNamespaceMapped returns whether the given local namespace is currently mapped to a remote namespace
// in accepted phase. Fallback reflectors use it to avoid erroneously rejecting pods belonging to
// namespaces whose reflection is only transiently stopped (e.g., during the startup of the reflection
// manager, or a momentary flapping of the NamespaceMap).
// An error (e.g., listing failure, or no NamespaceMap known yet) marks the state as uncertain: callers are
// expected to retry instead of taking irreversible actions on potentially incomplete information. Multiple
// NamespaceMaps (possible during delete+recreate cycles) are checked, rather than requiring uniqueness.
// The absence of the NamespaceMap is transient by construction during the virtual kubelet lifetime: the
// virtualnode controller recreates it whenever missing while the VirtualNode is alive, and, when tearing the
// peering down, it first drains the pods and deletes the virtual kubelet deployment, and only afterwards
// deletes the NamespaceMap (hence no pod is left for the fallback reflector to act upon).
func (nh *Handler) IsNamespaceMapped(namespace string) (bool, error) {
	nsMapFilter := labels.SelectorFromSet(labels.Set{
		liqoconst.RemoteClusterID:             string(forge.RemoteCluster),
		liqoconst.ReplicationDestinationLabel: string(forge.RemoteCluster),
	})
	namespaceMaps, err := nh.lister.List(nsMapFilter)
	if err != nil {
		return false, fmt.Errorf("failed to list NamespaceMaps: %w", err)
	}
	if len(namespaceMaps) == 0 {
		return false, fmt.Errorf("no NamespaceMap is present at the moment")
	}

	for i := range namespaceMaps {
		if !namespaceMaps[i].DeletionTimestamp.IsZero() {
			continue
		}
		mapping, found := namespaceMaps[i].Status.CurrentMapping[namespace]
		if found && mapping.Phase == offloadingv1beta1.MappingAccepted {
			return true, nil
		}
	}
	return false, nil
}

func (nh *Handler) startNamespace(localNs string, remoteNamespaceStatus offloadingv1beta1.RemoteNamespaceStatus) {
	if remoteNamespaceStatus.Phase != offloadingv1beta1.MappingAccepted {
		return
	}

	remoteNs := remoteNamespaceStatus.RemoteNamespace
	klog.V(3).Infof("Enabling reflection for remote namespace %s for local namespace %s", remoteNs, localNs)
	nh.namespaceStartStopper.StartNamespace(localNs, remoteNs)
}

func (nh *Handler) stopNamespace(localNs string, remoteNamespaceStatus offloadingv1beta1.RemoteNamespaceStatus) {
	if remoteNamespaceStatus.Phase != offloadingv1beta1.MappingAccepted {
		return
	}

	remoteNs := remoteNamespaceStatus.RemoteNamespace
	klog.V(3).Infof("Stopping reflection for remote namespace %s for local namespace %s", remoteNs, localNs)
	nh.namespaceStartStopper.StopNamespace(localNs, remoteNs)
}
