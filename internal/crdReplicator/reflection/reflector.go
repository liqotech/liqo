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

package reflection

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	"k8s.io/utils/trace"

	"github.com/liqotech/liqo/internal/crdReplicator/resources"
	"github.com/liqotech/liqo/pkg/consts"
	traceutils "github.com/liqotech/liqo/pkg/utils/trace"
)

// Reflector represents an object managing the reflection of resources from a source namespace of a source cluster to a target namespace of a target cluster.
type Reflector struct {
	sync.RWMutex

	// TenantNamespaces is a map of clusterIDs and tenant namespaces.
	TenantNamespaces map[string]string

	clientForTarget dynamic.Interface

	resources map[schema.GroupVersionResource][]*resourceToReflect
	workqueue workqueue.RateLimitingInterface

	getWorkers               func() uint
	getLocalClient           func() dynamic.Interface
	getResync                func() time.Duration
	getLister                func(schema.GroupVersionResource) cache.GenericLister
	registerManagerHandler   func(gvr schema.GroupVersionResource, namespace string, handler func(key item))
	unregisterManagerHandler func(gvr schema.GroupVersionResource, namespace string)
	cancel                   context.CancelFunc

	isLocalToLocal bool
}

// resourceToReflect wraps the listers associated with a resource to reflect.
type resourceToReflect struct {
	gvr       schema.GroupVersionResource
	ownership consts.OwnershipType

	sourceNamespace string
	targetNamespace string

	sourceClusterID string
	targetClusterID string
	localClusterID  string

	listerForSource cache.GenericNamespaceLister
	listerForTarget cache.GenericNamespaceLister

	cancel      context.CancelFunc
	initialized bool
}

// Start starts the reflection towards the target cluster.
func (r *Reflector) Start(ctx context.Context) {
	ctx, r.cancel = context.WithCancel(ctx)
	klog.Infof("Starting reflection workers")
	for i := uint(0); i < r.getWorkers(); i++ {
		go wait.Until(r.runWorker, time.Second, ctx.Done())
	}

	go func() {
		// Make sure the working queue is shutdown when the context is canceled.
		<-ctx.Done()
		r.workqueue.ShutDown()
	}()
}

// Stop stops the reflection towards the target cluster, and removes the replicated resources.
func (r *Reflector) Stop(sourceClusterID, targetClusterID string) error {
	r.Lock()
	defer r.Unlock()

	klog.Infof("[%v] Stopping reflection towards target cluster", targetClusterID)

	for gvr := range r.resources {
		if err := r.stopForResource(gvr, sourceClusterID, targetClusterID); err != nil {
			return err
		}
	}
	if r.cancel != nil {
		r.cancel()
	}
	return nil
}

// ResourceStarted returns whether the reflection for the given resource has been started.
func (r *Reflector) ResourceStarted(resource *resources.Resource, sourceClusterID, targetClusterID string) bool {
	_, found := r.get(resource.GroupVersionResource, sourceClusterID, targetClusterID)
	return found
}

// StartForResource starts the reflection of the given resource. It panics if executed for
// a resource with the reflection already started.
func (r *Reflector) StartForResource(ctx context.Context, resource *resources.Resource, sourceClusterID, targetClusterID, localClusterID string) {
	r.Lock()
	defer r.Unlock()

	gvr := resource.GroupVersionResource
	if _, found := r.find(gvr, sourceClusterID, targetClusterID); found {
		klog.Fatalf("[%v] Attempted to start reflection of %v while already in progress", targetClusterID, gvr)
	}

	// Create the informer towards the remote cluster
	klog.Infof("[%v] Starting reflection of %v", targetClusterID, gvr)

	// Source
	sourceNamespace := r.TenantNamespaces[sourceClusterID]
	var factorySource dynamicinformer.DynamicSharedInformerFactory
	var listerForSource cache.GenericNamespaceLister
	if resource.Forwardable {
		var tweakListOptions func(opts *metav1.ListOptions)
		if r.isLocalToLocal {
			tweakListOptions = func(opts *metav1.ListOptions) {
				opts.LabelSelector = r.passthroughLabelSelector(sourceClusterID, targetClusterID, localClusterID).String()
			}
		} else {
			tweakListOptions = func(opts *metav1.ListOptions) { opts.LabelSelector = r.localLabelSelector(targetClusterID).String() }
		}
		factorySource = dynamicinformer.NewFilteredDynamicSharedInformerFactory(r.getLocalClient(), r.getResync(), sourceNamespace, tweakListOptions)
		informer := factorySource.ForResource(gvr)
		informer.Informer().AddEventHandler(r.eventHandlers(gvr, sourceClusterID, targetClusterID))
		listerForSource = informer.Lister().ByNamespace(sourceNamespace)
	} else {
		// Use the already defined lister for local resources
		listerForSource = r.getLister(gvr).ByNamespace(sourceNamespace)
	}

	// Target
	targetNamespace := r.TenantNamespaces[targetClusterID]
	var tweakListOptions func(opts *metav1.ListOptions)
	if r.isLocalToLocal {
		tweakListOptions = func(opts *metav1.ListOptions) {
			opts.LabelSelector = r.passthroughLabelSelector(sourceClusterID, targetClusterID, targetClusterID).String()
		}
	} else {
		tweakListOptions = func(opts *metav1.ListOptions) { opts.LabelSelector = r.remoteLabelSelector(targetClusterID).String() }
	}
	factoryTarget := dynamicinformer.NewFilteredDynamicSharedInformerFactory(r.clientForTarget, r.getResync(), targetNamespace, tweakListOptions)
	informer := factoryTarget.ForResource(gvr)
	informer.Informer().AddEventHandler(r.eventHandlers(gvr, sourceClusterID, targetClusterID))
	listerForTarget := informer.Lister().ByNamespace(targetNamespace)

	ctx, cancel := context.WithCancel(ctx)
	resourceToReflect := &resourceToReflect{
		gvr:       gvr,
		ownership: resource.Ownership,

		sourceNamespace: sourceNamespace,
		targetNamespace: targetNamespace,

		sourceClusterID: sourceClusterID,
		targetClusterID: targetClusterID,
		localClusterID:  localClusterID,

		listerForSource: listerForSource,
		listerForTarget: listerForTarget,

		cancel: cancel,
	}
	r.resources[gvr] = append(r.resources[gvr], resourceToReflect)

	// The initialization is executed in a separate go routine, as cache synchronization might require some time to complete.
	go func() {
		tracer := trace.New("Initialization", trace.Field{Key: "RemoteClusterID", Value: targetClusterID}, trace.Field{Key: "Resource", Value: gvr})
		defer tracer.LogIfLong(traceutils.LongThreshold())

		// Start the informer, and wait for its caches to sync
		factoryTarget.Start(ctx.Done())
		synced := factoryTarget.WaitForCacheSync(ctx.Done())
		if !synced[gvr] {
			// The context was closed before the cache was ready, abort the setup
			return
		}

		if resource.Forwardable {
			factorySource.Start(ctx.Done())
			synced := factorySource.WaitForCacheSync(ctx.Done())
			if !synced[gvr] {
				// The context was closed before the cache was ready, abort the setup
				return
			}
		}

		// The informer has synced, and we are now ready to start the replication
		klog.Infof("[%v] Reflection of %v correctly started", targetClusterID, gvr)
		r.registerManagerHandler(gvr, sourceNamespace, func(key item) { r.workqueue.Add(key) })

		if res, found := r.get(gvr, sourceClusterID, targetClusterID); found {
			res.initialized = true
		}
	}()
}

// StopForResource stops the reflection of the given resource, and removes the replicated objects.
func (r *Reflector) StopForResource(resource *resources.Resource, sourceClusterID, targetClusterID string) error {
	r.Lock()
	defer r.Unlock()

	return r.stopForResource(resource.GroupVersionResource, sourceClusterID, targetClusterID)
}

// stopForResource stops the reflection of the given resource, and removes the replicated objects.
func (r *Reflector) stopForResource(gvr schema.GroupVersionResource, sourceClusterID, targetClusterID string) error {
	rs, found := r.find(gvr, sourceClusterID, targetClusterID)
	if !found {
		// This resource was already stopped, just return
		return nil
	}

	klog.Infof("[%v] Stopping reflection of %v", targetClusterID, gvr)

	// Check if any object is still present in the local or in the remote cluster
	for key, lister := range map[string]cache.GenericNamespaceLister{"local": rs.listerForSource, "remote": rs.listerForTarget} {
		objects, err := lister.List(labels.Everything())
		if err != nil {
			klog.Errorf("[%v] Failed to stop reflection of %v: %v", targetClusterID, gvr, err)
			return err
		}

		if len(objects) > 0 {
			klog.Errorf("[%v] Cannot stop reflection of %v, since remote objects are still present", targetClusterID, gvr)
			return fmt.Errorf("%v %v still present for cluster %v", key, gvr, targetClusterID)
		}
	}

	// Stop receiving updates from the informers
	sourceNamespace := r.TenantNamespaces[sourceClusterID]
	r.unregisterManagerHandler(gvr, sourceNamespace)
	rs.cancel()

	delete(r.resources, gvr)
	return nil
}

// get atomically returns the reflected resource structure associated with a given GVR and optionally a source and target clusterID.
func (r *Reflector) get(gvr schema.GroupVersionResource, sourceClusterID, targetClusterID string) (*resourceToReflect, bool) {
	r.RLock()
	defer r.RUnlock()
	return r.find(gvr, sourceClusterID, targetClusterID)
}

func (r *Reflector) find(gvr schema.GroupVersionResource, sourceClusterID, targetClusterID string) (*resourceToReflect, bool) {
	list := r.resources[gvr]

	if len(list) == 1 && !r.isLocalToLocal {
		// ignore source and target clusterIDs and return the only element present in the slice
		return list[0], list[0] != nil
	}

	var i int
	for i = 0; i < len(list); i++ {
		if list[i].sourceClusterID == sourceClusterID && list[i].targetClusterID == targetClusterID {
			break
		}
	}
	if i == len(list) {
		return nil, false
	}
	return list[i], list[i] != nil
}

// eventHandlers returns the event handlers which add the elements of a given GroupVersionResource to the working queue.
func (r *Reflector) eventHandlers(gvr schema.GroupVersionResource, sourceClusterID, targetClusterID string) cache.ResourceEventHandlerFuncs {
	eh := func(obj interface{}) {
		metadata, err := meta.Accessor(obj)
		utilruntime.Must(err)
		r.workqueue.Add(item{gvr: gvr, sourceClusterID: sourceClusterID, targetClusterID: targetClusterID, name: metadata.GetName()})
	}

	return cache.ResourceEventHandlerFuncs{
		AddFunc:    eh,
		UpdateFunc: func(_, obj interface{}) { eh(obj) },
		DeleteFunc: eh,
	}
}

// localLabelSelector returns a function which configures the label selector targeting the local resources
// that originated from the local cluster and did not originate from a target cluster.
func (r *Reflector) localLabelSelector(remoteClusterID string) labels.Selector {
	req1, err := labels.NewRequirement(consts.ReplicationDestinationLabel, selection.Equals, []string{remoteClusterID})
	utilruntime.Must(err)
	req2, err := labels.NewRequirement(consts.ReplicationRequestedLabel, selection.Equals, []string{strconv.FormatBool(true)})
	utilruntime.Must(err)
	req3, err := labels.NewRequirement(consts.ReplicationStatusLabel, selection.NotEquals, []string{strconv.FormatBool(true)})
	utilruntime.Must(err)
	return labels.NewSelector().Add(*req1, *req2, *req3)
}

// remoteLabelSelector returns a function which configures the label selector targeting the resources reflected
// by the local cluster in the given target cluster.
func (r *Reflector) remoteLabelSelector(targetClusterID string) labels.Selector {
	req1, err := labels.NewRequirement(consts.ReplicationOriginLabel, selection.NotEquals, []string{targetClusterID})
	utilruntime.Must(err)
	req2, err := labels.NewRequirement(consts.ReplicationDestinationLabel, selection.Equals, []string{targetClusterID})
	utilruntime.Must(err)
	req3, err := labels.NewRequirement(consts.ReplicationStatusLabel, selection.Equals, []string{strconv.FormatBool(true)})
	utilruntime.Must(err)
	selector := labels.NewSelector().Add(*req1, *req2, *req3)
	return selector
}

func (r *Reflector) passthroughLabelSelector(originClusterID, destinationClusterID, clusterID string) labels.Selector {
	req1, err := labels.NewRequirement("passthrough", selection.Equals, []string{strconv.FormatBool(true)})
	utilruntime.Must(err)
	req2, err := labels.NewRequirement(consts.ReplicationOriginLabel, selection.Equals, []string{originClusterID})
	utilruntime.Must(err)
	req3, err := labels.NewRequirement(consts.ReplicationDestinationLabel, selection.Equals, []string{clusterID})
	utilruntime.Must(err)
	req4, err := labels.NewRequirement("destination", selection.Equals, []string{destinationClusterID})
	utilruntime.Must(err)
	return labels.NewSelector().Add(*req1, *req2, *req3, *req4)
}

// ReplicatedResourcesLabelSelector is an helper function which returns a label selector to list all the replicated resources.
func ReplicatedResourcesLabelSelector() metav1.LabelSelector {
	return metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{
			{
				Key:      consts.ReplicationOriginLabel,
				Operator: metav1.LabelSelectorOpExists,
			},
			{
				Key:      consts.ReplicationStatusLabel,
				Operator: metav1.LabelSelectorOpIn,
				Values:   []string{strconv.FormatBool(true)},
			},
		},
	}
}
