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
	"fmt"
	"strconv"
	"sync"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
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

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	"github.com/liqotech/liqo/internal/crdReplicator/resources"
	"github.com/liqotech/liqo/pkg/consts"
	traceutils "github.com/liqotech/liqo/pkg/utils/trace"
)

// Reflector represents an object managing the reflection of resources towards a given remote cluster.
type Reflector struct {
	mu sync.RWMutex

	manager        *Manager
	localNamespace string
	localClusterID liqov1beta1.ClusterID

	remoteClient    dynamic.Interface
	remoteNamespace string
	remoteClusterID liqov1beta1.ClusterID

	resources map[schema.GroupVersionResource]*reflectedResource

	secretHash string

	workqueue workqueue.RateLimitingInterface
	cancel    context.CancelFunc
}

// reflectedResource wraps the listers associated with a reflected resource.
type reflectedResource struct {
	gvr       schema.GroupVersionResource
	ownership consts.OwnershipType

	local  cache.GenericNamespaceLister
	remote cache.GenericNamespaceLister

	cancel      context.CancelFunc
	initialized bool
}

// GetRemoteTenantNamespace returns the remote namespace where the reflector reflects the resources.
func (r *Reflector) GetRemoteTenantNamespace() string {
	return r.remoteNamespace
}

// GetSecretHash returns the hash of the secret that generated this reflector.
func (r *Reflector) GetSecretHash() string {
	return r.secretHash
}

// Start starts the reflection towards the remote cluster.
func (r *Reflector) Start(ctx context.Context) {
	ctx, r.cancel = context.WithCancel(ctx)
	klog.Infof("[%v] Starting reflection towards remote cluster", r.remoteClusterID)
	for i := uint(0); i < r.manager.workers; i++ {
		go wait.Until(r.runWorker, time.Second, ctx.Done())
	}

	go func() {
		// Make sure the working queue is shutdown when the context is canceled.
		<-ctx.Done()
		r.workqueue.ShutDown()
	}()
}

// Stop stops the reflection towards the remote cluster, it returns an error if there are replicated resources.
func (r *Reflector) Stop() error {
	return r.stop(false)
}

// StopForce stops the reflection towards the remote cluster, ignoring any replicated resource. This means that if replication is not
// restored, then there might be some orphan replicated resource in the remote cluster.
func (r *Reflector) StopForce() error {
	return r.stop(true)
}

// ResourceStarted returns whether the reflection for the given resource has been started.
func (r *Reflector) ResourceStarted(resource *resources.Resource) bool {
	_, found := r.get(resource.GroupVersionResource)
	return found
}

// StartForResource starts the reflection of the given resource. It panics if executed for
// a resource with the reflection already started.
func (r *Reflector) StartForResource(ctx context.Context, resource *resources.Resource) {
	r.mu.Lock()
	defer r.mu.Unlock()

	gvr := resource.GroupVersionResource
	if _, found := r.resources[gvr]; found {
		klog.Fatalf("[%v] Attempted to start reflection of %v while already in progress", r.remoteClusterID, gvr)
	}

	// Create the informer towards the remote cluster
	klog.Infof("[%v] Starting reflection of %v", r.remoteClusterID, gvr)
	tweakListOptions := func(opts *metav1.ListOptions) { opts.LabelSelector = r.remoteLabelSelector().String() }
	factory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(r.remoteClient, r.manager.resync, r.remoteNamespace, tweakListOptions)
	informer := factory.ForResource(gvr)
	informer.Informer().AddEventHandler(r.eventHandlers(gvr))

	ctx, cancel := context.WithCancel(ctx)
	r.resources[gvr] = &reflectedResource{
		gvr:       gvr,
		ownership: resource.Ownership,

		local:  r.manager.listers[gvr].ByNamespace(r.localNamespace),
		remote: informer.Lister().ByNamespace(r.remoteNamespace),

		cancel: cancel,
	}

	// The initialization is executed in a separate go routine, as cache synchronization might require some time to complete.
	go func() {
		tracer := trace.New("Initialization", trace.Field{Key: "RemoteClusterID", Value: r.remoteClusterID}, trace.Field{Key: "Resource", Value: gvr})
		defer tracer.LogIfLong(traceutils.LongThreshold())

		// Start the informer, and wait for its caches to sync
		factory.Start(ctx.Done())
		synced := factory.WaitForCacheSync(ctx.Done())

		if !synced[gvr] {
			// The context was closed before the cache was ready, let abort the setup
			return
		}

		// The informer has synced, and we are now ready to start te replication
		klog.Infof("[%v] Reflection of %v correctly started", r.remoteClusterID, gvr)
		r.manager.registerHandler(gvr, r.localNamespace, func(key item) { r.workqueue.Add(key) })

		if res, found := r.get(gvr); found {
			res.initialized = true
		}
	}()
}

// StopForResource stops the reflection of the given resource. It fails if there are replicated objects.
func (r *Reflector) StopForResource(resource *resources.Resource) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.stopForResource(resource.GroupVersionResource, false)
}

func (r *Reflector) stop(skipResourcePresenceCheck bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	klog.Infof("[%v] Stopping reflection towards remote cluster", r.remoteClusterID)

	for gvr := range r.resources {
		if err := r.stopForResource(gvr, skipResourcePresenceCheck); err != nil {
			return err
		}
	}

	r.cancel()
	return nil
}

// stopForResource stops the reflection of the given resource. Unless skipResourcePresenceCheck is false, it fails if there are replicated objects.
func (r *Reflector) stopForResource(gvr schema.GroupVersionResource, skipResourcePresenceCheck bool) error {
	rs, found := r.resources[gvr]
	if !found {
		// This resource was already stopped, just return
		return nil
	}

	klog.Infof("[%v] Stopping reflection of %v", r.remoteClusterID, gvr)

	if !skipResourcePresenceCheck {
		// Check if any object is still present in the local or in the remote cluster
		for key, lister := range map[string]cache.GenericNamespaceLister{"local": rs.local, "remote": rs.remote} {
			objects, err := lister.List(labels.Everything())

			if key == "remote" && apierrors.IsForbidden(err) {
				// The remote cluster has probably removed the necessary permissions to operate on reflected resources, let's ignore the error
				klog.Infof("[%v] Cannot list %v objects in the remote cluster (permission removed by provider).", r.remoteClusterID, gvr)
				continue
			}

			if err != nil {
				klog.Errorf("[%v] Failed to stop reflection of %v: %v", r.remoteClusterID, gvr, err)
				return err
			}

			if len(objects) > 0 {
				klog.Errorf("[%v] Cannot stop reflection of %v, since %v objects are still present", r.remoteClusterID, gvr, key)
				return fmt.Errorf("%v %v still present for cluster %v", key, gvr, r.remoteClusterID)
			}
		}
	}

	// Stop receiving updates from the informers
	r.manager.unregisterHandler(gvr, r.localNamespace)
	rs.cancel()

	delete(r.resources, gvr)
	return nil
}

// get atomically returns the reflected resource structure associated with a given GVR.
func (r *Reflector) get(gvr schema.GroupVersionResource) (*reflectedResource, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	res, found := r.resources[gvr]
	return res, found
}

// eventHandlers returns the event handlers which add the elements of a given GroupVersionResource to the working queue.
func (r *Reflector) eventHandlers(gvr schema.GroupVersionResource) cache.ResourceEventHandlerFuncs {
	eh := func(obj interface{}) {
		metadata, err := meta.Accessor(obj)
		utilruntime.Must(err)

		r.workqueue.Add(item{gvr: gvr, name: metadata.GetName()})
	}

	return cache.ResourceEventHandlerFuncs{
		AddFunc:    eh,
		UpdateFunc: func(_, obj interface{}) { eh(obj) },
		DeleteFunc: eh,
	}
}

// remoteLabelSelector returns a function which configures the label selector targeting the resources reflected
// by us in the given remote cluster.
func (r *Reflector) remoteLabelSelector() labels.Selector {
	req1, err := labels.NewRequirement(consts.ReplicationOriginLabel, selection.Equals, []string{string(r.localClusterID)})
	utilruntime.Must(err)
	req2, err := labels.NewRequirement(consts.ReplicationStatusLabel, selection.Equals, []string{strconv.FormatBool(true)})
	utilruntime.Must(err)
	return labels.NewSelector().Add(*req1, *req2)
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

// IsReplicated returns whether the given object is replicated by the CRD replicator.
func IsReplicated(obj metav1.Object) bool {
	if obj.GetLabels() == nil {
		return false
	}

	return obj.GetLabels()[consts.ReplicationStatusLabel] == strconv.FormatBool(true)
}
