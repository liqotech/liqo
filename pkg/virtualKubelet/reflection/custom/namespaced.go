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

package custom

import (
	"context"
	"fmt"
	"sync/atomic"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	"k8s.io/utils/trace"
	"sigs.k8s.io/controller-runtime/pkg/client"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/generic"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/manager"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/options"
)

// NamespacedGVRReflector manages the reflection of a single GVR between paired namespaces.
type NamespacedGVRReflector struct {
	generic.NamespacedReflector

	gvr schema.GroupVersionResource

	localLister  cache.GenericNamespaceLister
	remoteLister cache.GenericNamespaceLister

	localClient  dynamic.ResourceInterface
	remoteClient dynamic.ResourceInterface

	statusSupported atomic.Bool
}

// NewNamespacedGVRReflector returns a function generating NamespacedGVRReflector instances for the given GVR.
func NewNamespacedGVRReflector(gvr schema.GroupVersionResource) func(*options.NamespacedOpts) manager.NamespacedReflector {
	return func(opts *options.NamespacedOpts) manager.NamespacedReflector {
		localInformer := opts.LocalDynamicFactory.ForResource(gvr)
		remoteInformer := opts.RemoteDynamicFactory.ForResource(gvr)

		_, err := localInformer.Informer().AddEventHandler(opts.HandlerFactory(generic.NamespacedKeyer(opts.LocalNamespace)))
		utilruntime.Must(err)
		_, err = remoteInformer.Informer().AddEventHandler(opts.HandlerFactory(generic.NamespacedKeyer(opts.LocalNamespace)))
		utilruntime.Must(err)

		ncr := &NamespacedGVRReflector{
			NamespacedReflector: generic.NewNamespacedReflector(opts, "CustomResource["+gvr.String()+"]"),
			gvr:                 gvr,
			localLister:         localInformer.Lister().ByNamespace(opts.LocalNamespace),
			remoteLister:        remoteInformer.Lister().ByNamespace(opts.RemoteNamespace),
			localClient:         opts.LocalDynamicClient.Resource(gvr).Namespace(opts.LocalNamespace),
			remoteClient:        opts.RemoteDynamicClient.Resource(gvr).Namespace(opts.RemoteNamespace),
		}
		ncr.statusSupported.Store(true)
		return ncr
	}
}

// Handle is responsible for reconciling the given object and ensuring it is correctly reflected.
func (ncr *NamespacedGVRReflector) Handle(ctx context.Context, name string) error {
	tracer := trace.FromContext(ctx)

	klog.V(4).Infof("Handling reflection of local %v %q (remote: %q)", ncr.gvr, ncr.LocalRef(name), ncr.RemoteRef(name))

	localObj, lerr := ncr.localLister.Get(name)
	utilruntime.Must(client.IgnoreNotFound(lerr))
	remoteObj, rerr := ncr.remoteLister.Get(name)
	utilruntime.Must(client.IgnoreNotFound(rerr))
	tracer.Step("Retrieved the local and remote objects")

	var local, remote *unstructured.Unstructured
	if lerr == nil {
		var ok bool
		local, ok = localObj.(*unstructured.Unstructured)
		if !ok {
			return fmt.Errorf("unexpected local object type %T for %v %q", localObj, ncr.gvr, ncr.LocalRef(name))
		}
	}
	if rerr == nil {
		var ok bool
		remote, ok = remoteObj.(*unstructured.Unstructured)
		if !ok {
			return fmt.Errorf("unexpected remote object type %T for %v %q", remoteObj, ncr.gvr, ncr.RemoteRef(name))
		}
	}

	// Abort the reflection if the remote object is not managed by us.
	if rerr == nil && !forge.IsReflected(remote) {
		if lerr == nil {
			klog.Infof("Skipping reflection of local %v %q as remote already exists and is not managed by us",
				ncr.gvr, ncr.LocalRef(name))
			ncr.Event(local, corev1.EventTypeWarning, forge.EventFailedReflection, forge.EventFailedReflectionAlreadyExistsMsg())
		}
		return nil
	}

	// Abort the reflection if the local object should be skipped by policy.
	if !kerrors.IsNotFound(lerr) {
		skipReflection, err := ncr.ShouldSkipReflection(local)
		if err != nil {
			klog.Errorf("Failed to check whether local %v %q should be reflected: %v", ncr.gvr, ncr.LocalRef(name), err)
			return err
		}
		if skipReflection {
			if ncr.GetReflectionType() == offloadingv1beta1.DenyList {
				klog.Infof("Skipping reflection of local %v %q as marked with the skip annotation", ncr.gvr, ncr.LocalRef(name))
			} else {
				klog.Infof("Skipping reflection of local %v %q as not marked with the allow annotation", ncr.gvr, ncr.LocalRef(name))
			}
			ncr.Event(local, corev1.EventTypeNormal, forge.EventReflectionDisabled, forge.EventObjectReflectionDisabledMsg(ncr.GetReflectionType()))
			if kerrors.IsNotFound(rerr) {
				return nil
			}
			// Pretend the local object does not exist so that the remote twin is deleted.
			lerr = kerrors.NewNotFound(ncr.gvr.GroupResource(), name)
		}
	}

	tracer.Step("Performed the sanity checks")

	if kerrors.IsNotFound(lerr) {
		defer tracer.Step("Ensured the absence of the remote object")
		if !kerrors.IsNotFound(rerr) {
			klog.V(4).Infof("Deleting remote %v %q, since local %q does no longer exist",
				ncr.gvr, ncr.RemoteRef(name), ncr.LocalRef(name))
			return ncr.deleteRemote(ctx, remote)
		}
		klog.V(4).Infof("Local %v %q and remote %q both vanished", ncr.gvr, ncr.LocalRef(name), ncr.RemoteRef(name))
		return nil
	}

	// Ensure the remote twin exists with the local spec.
	var err error
	if kerrors.IsNotFound(rerr) {
		klog.Infof("Creating remote %v %q (local: %q)", ncr.gvr, ncr.RemoteRef(name), ncr.LocalRef(name))
		remote, err = createRemoteObject(ctx, ncr.remoteClient, local, ncr.RemoteNamespace(), ncr.ForgingOpts)
		if err != nil {
			return ncr.handleRemoteError(local, "create", err)
		}
	} else {
		remote, err = updateRemoteObjectSpec(ctx, ncr.remoteClient, local, remote.DeepCopy(), ncr.ForgingOpts)
		if err != nil {
			return ncr.handleRemoteError(local, "update", err)
		}
	}
	tracer.Step("Ensured the remote object")

	// Sync status from remote → local (OwnershipShared), if supported.
	if ncr.statusSupported.Load() {
		if err = updateObjectStatusShared(ctx, ncr.localClient, ncr.gvr, remote, local.DeepCopy()); err != nil {
			switch {
			case kerrors.IsMethodNotSupported(err):
				if ncr.statusSupported.CompareAndSwap(true, false) {
					klog.Warningf("Status subresource not supported for %v, skipping status sync", ncr.gvr)
				}
			case kerrors.IsNotFound(err):
				// Object vanished between get and UpdateStatus; retry via workqueue.
				return err
			case kerrors.IsForbidden(err):
				klog.Infof("Cannot update status of local %v %q (forbidden): %v", ncr.gvr, ncr.LocalRef(name), err)
				ncr.Event(local, corev1.EventTypeWarning, forge.EventFailedReflection, forge.EventFailedReflectionMsg(err))
				return nil
			default:
				klog.Errorf("Failed to update status of local %v %q: %v", ncr.gvr, ncr.LocalRef(name), err)
				ncr.Event(local, corev1.EventTypeWarning, forge.EventFailedReflection, forge.EventFailedReflectionMsg(err))
				return err
			}
		}
	}
	tracer.Step("Ensured the status is synchronized")

	klog.Infof("Remote %v %q successfully enforced (local: %q)", ncr.gvr, ncr.RemoteRef(name), ncr.LocalRef(name))
	ncr.Event(local, corev1.EventTypeNormal, forge.EventSuccessfulReflection, forge.EventSuccessfulReflectionMsg())
	return nil
}

func (ncr *NamespacedGVRReflector) handleRemoteError(local *unstructured.Unstructured, op string, err error) error {
	if kerrors.IsForbidden(err) {
		klog.Infof("Cannot %s remote %v %q (forbidden): %v", op, ncr.gvr, ncr.RemoteRef(local.GetName()), err)
		ncr.Event(local, corev1.EventTypeWarning, forge.EventFailedReflection, forge.EventFailedReflectionMsg(err))
		return nil
	}
	if kerrors.IsNotFound(err) {
		// CRD missing on the remote cluster.
		klog.Warningf("Cannot %s remote %v %q (CRD may be missing): %v", op, ncr.gvr, ncr.RemoteRef(local.GetName()), err)
		ncr.Event(local, corev1.EventTypeWarning, forge.EventFailedReflection, forge.EventFailedReflectionMsg(err))
		return nil
	}
	klog.Errorf("Failed to %s remote %v %q (local: %q): %v", op, ncr.gvr, ncr.RemoteRef(local.GetName()), ncr.LocalRef(local.GetName()), err)
	ncr.Event(local, corev1.EventTypeWarning, forge.EventFailedReflection, forge.EventFailedReflectionMsg(err))
	return err
}

func (ncr *NamespacedGVRReflector) deleteRemote(ctx context.Context, remote *unstructured.Unstructured) error {
	err := ncr.remoteClient.Delete(ctx, remote.GetName(), *metav1.NewPreconditionDeleteOptions(string(remote.GetUID())))
	if err != nil && !kerrors.IsNotFound(err) {
		if kerrors.IsForbidden(err) {
			klog.Infof("Cannot delete remote %v %q (forbidden): %v", ncr.gvr, ncr.RemoteRef(remote.GetName()), err)
			return nil
		}
		klog.Errorf("Failed to delete remote %v %q: %v", ncr.gvr, ncr.RemoteRef(remote.GetName()), err)
		return err
	}
	klog.Infof("Remote %v %q successfully deleted", ncr.gvr, ncr.RemoteRef(remote.GetName()))
	return nil
}

// List returns the list of objects to be reflected.
func (ncr *NamespacedGVRReflector) List() ([]interface{}, error) {
	localObjs, err := ncr.localLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}
	remoteObjs, err := ncr.remoteLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}

	list := make([]interface{}, 0, len(localObjs)+len(remoteObjs))
	for _, obj := range localObjs {
		meta, ok := obj.(metav1.Object)
		if !ok {
			continue
		}
		list = append(list, types.NamespacedName{Name: meta.GetName(), Namespace: ncr.LocalNamespace()})
	}
	for _, obj := range remoteObjs {
		meta, ok := obj.(metav1.Object)
		if !ok {
			continue
		}
		list = append(list, types.NamespacedName{Name: meta.GetName(), Namespace: ncr.LocalNamespace()})
	}
	return list, nil
}
