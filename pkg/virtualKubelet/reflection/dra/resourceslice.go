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

package dra

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	corev1listers "k8s.io/client-go/listers/core/v1"
	resourcev1listers "k8s.io/client-go/listers/resource/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/leaderelection"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/generic"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/manager"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/options"
)

const (
	// ResourceSliceReflectorName is the name associated with the ResourceSlice reflector.
	ResourceSliceReflectorName = "ResourceSlice"
)

var _ manager.Reflector = (*ResourceSliceReflector)(nil)

// ResourceSliceReflector is a cluster-scoped, leader-only reflector that mirrors
// ResourceSlice objects from the remote cluster into the local cluster. The leader VK
// across the peering handles slices for every node in the peering, not just its own.
type ResourceSliceReflector struct {
	name    string
	workers uint

	localClient  kubernetes.Interface
	remoteClient kubernetes.Interface
	resync       time.Duration

	workqueue workqueue.TypedRateLimitingInterface[types.NamespacedName]

	// Listers populated in Start.
	localSlices  resourcev1listers.ResourceSliceLister
	remoteSlices resourcev1listers.ResourceSliceLister
	localNodes   corev1listers.NodeLister

	forgingOpts *forge.ForgingOpts
}

// NewResourceSliceReflector returns a new ResourceSliceReflector. When the configured
// number of workers is zero, a dummy reflector is returned that performs no work.
func NewResourceSliceReflector(localClient, remoteClient kubernetes.Interface,
	resync time.Duration, reflectorConfig *offloadingv1beta1.ReflectorConfig) manager.Reflector {
	if reflectorConfig.NumWorkers == 0 {
		return generic.NewDummyReflector(ResourceSliceReflectorName)
	}

	return &ResourceSliceReflector{
		name:         ResourceSliceReflectorName,
		workers:      reflectorConfig.NumWorkers,
		localClient:  localClient,
		remoteClient: remoteClient,
		resync:       resync,
		workqueue: workqueue.NewTypedRateLimitingQueueWithConfig(
			workqueue.DefaultTypedControllerRateLimiter[types.NamespacedName](),
			workqueue.TypedRateLimitingQueueConfig[types.NamespacedName]{Name: ResourceSliceReflectorName}),
	}
}

// String returns the name of the reflector.
func (rsr *ResourceSliceReflector) String() string { return rsr.name }

// Start sets up the cluster-scoped local and remote informers and launches the workers.
// StartNamespace is a no-op for this reflector.
func (rsr *ResourceSliceReflector) Start(ctx context.Context, opts *options.ReflectorOpts) {
	klog.Infof("Starting the %v reflector with %v workers (leader-only)", rsr.name, rsr.workers)
	rsr.forgingOpts = opts.ForgingOpts
	if rsr.forgingOpts == nil {
		rsr.forgingOpts = &forge.ForgingOpts{}
	}

	localFactory := informers.NewSharedInformerFactory(rsr.localClient, rsr.resync)
	remoteFactory := informers.NewSharedInformerFactory(rsr.remoteClient, rsr.resync)
	// Separate factory for nodes to just listen on the virtual nodes changes.
	nodeFactory := informers.NewSharedInformerFactoryWithOptions(rsr.localClient, rsr.resync,
		informers.WithTweakListOptions(func(opts *metav1.ListOptions) {
			opts.LabelSelector = consts.TypeLabel + "=" + consts.TypeNode
		}),
	)

	localSliceInformer := localFactory.Resource().V1().ResourceSlices()
	remoteSliceInformer := remoteFactory.Resource().V1().ResourceSlices()
	localNodeInformer := nodeFactory.Core().V1().Nodes()

	_, err := localSliceInformer.Informer().AddEventHandler(rsr.handlers())
	utilruntime.Must(err)
	_, err = remoteSliceInformer.Informer().AddEventHandler(rsr.handlers())
	utilruntime.Must(err)
	// When a virtual node appears, enqueue the remote slices for that node,
	// as they may have been skipped due to the missing local node.
	_, err = localNodeInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj any) {
			node, ok := obj.(*corev1.Node)
			if !ok {
				return
			}
			rsr.enqueueRemoteSlicesForNode(node.Name)
		},
	})
	utilruntime.Must(err)

	rsr.localSlices = localSliceInformer.Lister()
	rsr.remoteSlices = remoteSliceInformer.Lister()
	rsr.localNodes = localNodeInformer.Lister()

	localFactory.Start(ctx.Done())
	remoteFactory.Start(ctx.Done())
	nodeFactory.Start(ctx.Done())
	localFactory.WaitForCacheSync(ctx.Done())
	remoteFactory.WaitForCacheSync(ctx.Done())
	nodeFactory.WaitForCacheSync(ctx.Done())

	for i := uint(0); i < rsr.workers; i++ {
		go wait.Until(rsr.runWorker, time.Second, ctx.Done())
	}

	go func() {
		<-ctx.Done()
		rsr.workqueue.ShutDown()
	}()
}

// StartNamespace is a no-op: the reflector is cluster-scoped.
func (rsr *ResourceSliceReflector) StartNamespace(_ *options.NamespacedOpts) {}

// StopNamespace is a no-op: the reflector is cluster-scoped.
func (rsr *ResourceSliceReflector) StopNamespace(_, _ string) {}

// Resync re-enqueues every known remote and local slice so orphaned local
// slices (missed remote-delete events) also get reconciled.
func (rsr *ResourceSliceReflector) Resync() error {
	if rsr.remoteSlices != nil {
		remote, err := rsr.remoteSlices.List(labels.Everything())
		if err != nil {
			klog.Errorf("Failed to list remote ResourceSlices for resync: %v", err)
		} else {
			for _, s := range remote {
				rsr.workqueue.Add(types.NamespacedName{Name: s.Name})
			}
		}
	}
	if rsr.localSlices != nil {
		local, err := rsr.localSlices.List(labels.Everything())
		if err != nil {
			klog.Errorf("Failed to list local ResourceSlices for resync: %v", err)
		} else {
			for _, s := range local {
				rsr.workqueue.Add(types.NamespacedName{Name: s.Name})
			}
		}
	}
	return nil
}

// enqueueRemoteSlicesForNode enqueues only the remote slices whose Spec.NodeName
// matches the given local virtual node name.
func (rsr *ResourceSliceReflector) enqueueRemoteSlicesForNode(nodeName string) {
	if rsr.remoteSlices == nil {
		return
	}
	slices, err := rsr.remoteSlices.List(labels.Everything())
	if err != nil {
		klog.Errorf("Failed to list remote ResourceSlices: %v", err)
		return
	}
	for _, s := range slices {
		if s.Spec.NodeName != nil && *s.Spec.NodeName == nodeName {
			rsr.workqueue.Add(types.NamespacedName{Name: s.Name})
		}
	}
}

func (rsr *ResourceSliceReflector) handlers() cache.ResourceEventHandler {
	enqueue := func(ev watch.EventType, obj any) {
		if ev == watch.Deleted {
			switch t := obj.(type) {
			case cache.DeletedFinalStateUnknown:
				obj = t.Obj
			case *cache.DeletedFinalStateUnknown:
				obj = t.Obj
			}
		}

		md, err := meta.Accessor(obj)
		if err != nil {
			klog.Errorf("Failed metadata accessor in %v reflector: obj=%T err=%v", rsr.name, obj, err)
			return
		}
		rsr.workqueue.Add(types.NamespacedName{Name: md.GetName()})
	}
	return cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj any) { enqueue(watch.Added, obj) },
		UpdateFunc: func(_, obj any) { enqueue(watch.Modified, obj) },
		DeleteFunc: func(obj any) { enqueue(watch.Deleted, obj) },
	}
}

func (rsr *ResourceSliceReflector) runWorker() {
	for rsr.processNext() {
	}
}

func (rsr *ResourceSliceReflector) processNext() bool {
	key, shutdown := rsr.workqueue.Get()
	if shutdown {
		return false
	}
	defer rsr.workqueue.Done(key)

	if !leaderelection.IsLeader() {
		klog.V(4).Infof("Skipping %v reflector item %v because the node is not the leader", rsr.name, key)
		return true
	}

	err := rsr.handle(context.Background(), key.Name)
	if err != nil {
		rsr.workqueue.AddRateLimited(key)
		return true
	}

	rsr.workqueue.Forget(key)
	return true
}

// handle reconciles a single ResourceSlice by name.
func (rsr *ResourceSliceReflector) handle(ctx context.Context, name string) error {
	klog.V(4).Infof("Handling reflection of ResourceSlice %q", name)

	local, lerr := rsr.localSlices.Get(name)
	if lerr != nil && !kerrors.IsNotFound(lerr) {
		klog.Errorf("Failed to get local ResourceSlice %q: %v", name, lerr)
		return fmt.Errorf("getting local ResourceSlice %q: %w", name, lerr)
	}

	// Refuse to mutate a local slice we don't own.
	if lerr == nil && !forge.IsReflected(local) {
		klog.V(4).Infof("Skipping reflection of ResourceSlice %q: local exists and is not managed by us", name)
		return nil
	}

	remote, rerr := rsr.remoteSlices.Get(name)
	if rerr != nil && !kerrors.IsNotFound(rerr) {
		klog.Errorf("Failed to get remote ResourceSlice %q: %v", name, rerr)
		return fmt.Errorf("getting remote ResourceSlice %q: %w", name, rerr)
	}

	// Remote vanished -> ensure local is also gone.
	if kerrors.IsNotFound(rerr) {
		if local != nil {
			klog.Infof("Deleting local ResourceSlice %q since remote no longer exists", name)
			err := rsr.localClient.ResourceV1().ResourceSlices().Delete(ctx, name,
				*metav1.NewPreconditionDeleteOptions(string(local.GetUID())))
			if err != nil && !kerrors.IsNotFound(err) {
				return fmt.Errorf("deleting ResourceSlice %q: %w", name, err)
			}
		}
		return nil
	}

	// We currently only handle slices whose Spec.NodeName is set.
	// TODO: support spec.NodeSelector, spec.AllNodes and spec.PerDeviceNodeSelection.
	if remote.Spec.NodeName == nil || *remote.Spec.NodeName == "" {
		klog.V(4).Infof("Skipping reflection of ResourceSlice %q: spec.NodeName is unset (NodeSelector/AllNodes not yet supported)", name)
		return nil
	}

	// Resolve the local virtual node by name. The 1:1 mapping in this fork means
	// remote.Spec.NodeName == local virtual node name.
	localNode, err := rsr.localNodes.Get(*remote.Spec.NodeName)
	if err != nil {
		if kerrors.IsNotFound(err) {
			klog.V(4).Infof("Skipping reflection of ResourceSlice %q: no local node %q yet", name, *remote.Spec.NodeName)
			return nil
		}
		return fmt.Errorf("getting local node %q: %w", *remote.Spec.NodeName, err)
	}

	desired := forge.LocalResourceSlice(remote, localNode, rsr.forgingOpts.LabelsNotReflected, rsr.forgingOpts.AnnotationsNotReflected)

	if kerrors.IsNotFound(lerr) {
		if _, err := rsr.localClient.ResourceV1().ResourceSlices().Create(ctx, desired, metav1.CreateOptions{}); err != nil {
			klog.Errorf("Failed to create local ResourceSlice %q: %v", name, err)
			return fmt.Errorf("creating local ResourceSlice %q: %w", name, err)
		}
		klog.Infof("Local ResourceSlice %q successfully created (owner node: %q)", name, localNode.Name)
		return nil
	}

	if apiequality.Semantic.DeepEqual(local.Labels, desired.Labels) &&
		apiequality.Semantic.DeepEqual(local.Annotations, desired.Annotations) &&
		apiequality.Semantic.DeepEqual(local.OwnerReferences, desired.OwnerReferences) &&
		apiequality.Semantic.DeepEqual(local.Spec, desired.Spec) {
		klog.V(4).Infof("Local ResourceSlice %q is already up-to-date", name)
		return nil
	}

	updated := local.DeepCopy()
	updated.Labels = desired.Labels
	updated.Annotations = desired.Annotations
	updated.OwnerReferences = desired.OwnerReferences
	updated.Spec = desired.Spec
	if _, err := rsr.localClient.ResourceV1().ResourceSlices().Update(ctx, updated, metav1.UpdateOptions{}); err != nil {
		klog.Errorf("Failed to update local ResourceSlice %q: %v", name, err)
		return fmt.Errorf("updating local ResourceSlice %q: %w", name, err)
	}
	klog.V(4).Infof("Local ResourceSlice %q successfully updated", name)
	return nil
}
