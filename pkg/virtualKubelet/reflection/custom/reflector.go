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

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"
	"k8s.io/utils/trace"
	"sigs.k8s.io/controller-runtime/pkg/client"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/generic"
)

// objectLister abstracts the operations required to retrieve the reflected objects from
// the informer caches, independently from the resource scope (cache.GenericLister for
// cluster-scoped resources, cache.GenericNamespaceLister for namespaced ones).
type objectLister interface {
	Get(name string) (runtime.Object, error)
	List(selector labels.Selector) ([]runtime.Object, error)
}

// gvrReflector implements the reconciliation logic shared by the namespaced and the
// cluster-scoped custom resource reflectors. The embedded generic.NamespacedReflector is
// forged with empty namespaces in case of cluster-scoped reflection, so that the object
// refs correctly identify resources not bound to any namespace.
type gvrReflector struct {
	generic.NamespacedReflector

	gvr schema.GroupVersionResource

	localLister  objectLister
	remoteLister objectLister

	localClient  dynamic.ResourceInterface
	remoteClient dynamic.ResourceInterface

	// clusterScoped indicates whether the reflected resource is cluster-scoped
	// (i.e. not associated with any namespace).
	clusterScoped bool

	// statusSyncEnabled indicates whether the remote→local status sync is active,
	// i.e. the statusSync option is set and the CRD exposes the status subresource.
	// It is immutable: when false, the status sync is skipped entirely.
	statusSyncEnabled bool
}

// Handle is responsible for reconciling the given object and ensuring it is correctly reflected.
func (r *gvrReflector) Handle(ctx context.Context, name string) error {
	tracer := trace.FromContext(ctx)

	klog.V(4).Infof("Handling reflection of local %v %q (remote: %q)", r.gvr, r.LocalRef(name), r.RemoteRef(name))

	localObj, lerr := r.localLister.Get(name)
	utilruntime.Must(client.IgnoreNotFound(lerr))
	remoteObj, rerr := r.remoteLister.Get(name)
	utilruntime.Must(client.IgnoreNotFound(rerr))
	tracer.Step("Retrieved the local and remote objects")

	var local, remote *unstructured.Unstructured
	if lerr == nil {
		var ok bool
		local, ok = localObj.(*unstructured.Unstructured)
		if !ok {
			return fmt.Errorf("unexpected local object type %T for %v %q", localObj, r.gvr, r.LocalRef(name))
		}
	}
	if rerr == nil {
		var ok bool
		remote, ok = remoteObj.(*unstructured.Unstructured)
		if !ok {
			return fmt.Errorf("unexpected remote object type %T for %v %q", remoteObj, r.gvr, r.RemoteRef(name))
		}
	}

	// Abort the reflection if the remote object is not managed by us.
	if rerr == nil && !forge.IsReflected(remote) {
		if lerr == nil {
			klog.Infof("Skipping reflection of local %v %q as remote already exists and is not managed by us",
				r.gvr, r.LocalRef(name))
			r.Event(local, corev1.EventTypeWarning, forge.EventFailedReflection, forge.EventFailedReflectionAlreadyExistsMsg())
		}
		return nil
	}

	// Abort the reflection if the local object should be skipped by policy.
	if !kerrors.IsNotFound(lerr) {
		skipReflection, err := r.ShouldSkipReflection(local)
		if err != nil {
			klog.Errorf("Failed to check whether local %v %q should be reflected: %v", r.gvr, r.LocalRef(name), err)
			return err
		}
		if skipReflection {
			if r.GetReflectionType() == offloadingv1beta1.DenyList {
				klog.Infof("Skipping reflection of local %v %q as marked with the skip annotation", r.gvr, r.LocalRef(name))
			} else {
				klog.Infof("Skipping reflection of local %v %q as not marked with the allow annotation", r.gvr, r.LocalRef(name))
			}
			r.Event(local, corev1.EventTypeNormal, forge.EventReflectionDisabled, forge.EventObjectReflectionDisabledMsg(r.GetReflectionType()))
			if kerrors.IsNotFound(rerr) {
				return nil
			}
			// Pretend the local object does not exist so that the remote twin is deleted.
			lerr = kerrors.NewNotFound(r.gvr.GroupResource(), name)
		}
	}

	tracer.Step("Performed the sanity checks")

	if kerrors.IsNotFound(lerr) {
		defer tracer.Step("Ensured the absence of the remote object")
		if !kerrors.IsNotFound(rerr) {
			klog.V(4).Infof("Deleting remote %v %q, since local %q does no longer exist",
				r.gvr, r.RemoteRef(name), r.LocalRef(name))
			return r.deleteRemote(ctx, remote)
		}
		klog.V(4).Infof("Local %v %q and remote %q both vanished", r.gvr, r.LocalRef(name), r.RemoteRef(name))
		return nil
	}

	// Ensure the remote twin matches the local spec through server-side apply: the
	// reflector owns the spec and the reflected metadata, while fields owned by other
	// field managers (e.g. annotations added on the remote twin by other controllers)
	// are preserved.
	klog.V(4).Infof("Enforcing remote %v %q (local: %q)", r.gvr, r.RemoteRef(name), r.LocalRef(name))
	remote, err := applyRemoteObject(ctx, r.remoteClient, local, r.RemoteNamespace(), r.ForgingOpts)
	if err != nil {
		return r.handleRemoteError(local, err)
	}
	tracer.Step("Ensured the remote object")

	// Sync status from remote → local (OwnershipShared), if the status sync is enabled.
	if r.statusSyncEnabled {
		if err = updateObjectStatusShared(ctx, r.localClient, r.gvr, remote, local.DeepCopy()); err != nil {
			switch {
			case kerrors.IsNotFound(err):
				// Object vanished between get and UpdateStatus; retry via workqueue.
				return err
			case kerrors.IsForbidden(err):
				klog.Infof("Cannot update status of local %v %q (forbidden): %v", r.gvr, r.LocalRef(name), err)
				r.Event(local, corev1.EventTypeWarning, forge.EventFailedReflection, forge.EventFailedReflectionMsg(err))
				return nil
			default:
				klog.Errorf("Failed to update status of local %v %q: %v", r.gvr, r.LocalRef(name), err)
				r.Event(local, corev1.EventTypeWarning, forge.EventFailedReflection, forge.EventFailedReflectionMsg(err))
				return err
			}
		}
	}
	tracer.Step("Ensured the status is synchronized")

	klog.Infof("Remote %v %q successfully enforced (local: %q)", r.gvr, r.RemoteRef(name), r.LocalRef(name))
	r.Event(local, corev1.EventTypeNormal, forge.EventSuccessfulReflection, forge.EventSuccessfulReflectionMsg())
	return nil
}

func (r *gvrReflector) handleRemoteError(local *unstructured.Unstructured, err error) error {
	if kerrors.IsForbidden(err) {
		klog.Infof("Cannot apply remote %v %q (forbidden): %v", r.gvr, r.RemoteRef(local.GetName()), err)
		if r.clusterScoped {
			// Cluster-scoped resources are only granted the required remote cluster-wide permissions
			// when explicitly marked as clusterScoped in the chart values.
			klog.Infof("Hint: cluster-scoped resources require cluster-wide permissions on the remote cluster; " +
				"ensure the provider has the necessary cluster-wide permissions")
		}
		r.Event(local, corev1.EventTypeWarning, forge.EventFailedReflection, forge.EventFailedReflectionMsg(err))
		return nil
	}
	if kerrors.IsNotFound(err) {
		// An apply creates the twin if absent, hence a NotFound error means the
		// CRD (or the remote namespace) is missing on the remote cluster.
		klog.Warningf("Cannot apply remote %v %q (CRD may be missing on the remote cluster): %v",
			r.gvr, r.RemoteRef(local.GetName()), err)
		r.Event(local, corev1.EventTypeWarning, forge.EventFailedReflection, forge.EventFailedReflectionMsg(err))
		return nil
	}
	klog.Errorf("Failed to apply remote %v %q (local: %q): %v",
		r.gvr, r.RemoteRef(local.GetName()), r.LocalRef(local.GetName()), err)
	r.Event(local, corev1.EventTypeWarning, forge.EventFailedReflection, forge.EventFailedReflectionMsg(err))
	return err
}

func (r *gvrReflector) deleteRemote(ctx context.Context, remote *unstructured.Unstructured) error {
	err := r.remoteClient.Delete(ctx, remote.GetName(), *metav1.NewPreconditionDeleteOptions(string(remote.GetUID())))
	if err != nil && !kerrors.IsNotFound(err) {
		if kerrors.IsForbidden(err) {
			klog.Infof("Cannot delete remote %v %q (forbidden): %v", r.gvr, r.RemoteRef(remote.GetName()), err)
			return nil
		}
		klog.Errorf("Failed to delete remote %v %q: %v", r.gvr, r.RemoteRef(remote.GetName()), err)
		return err
	}
	klog.Infof("Remote %v %q successfully deleted", r.gvr, r.RemoteRef(remote.GetName()))
	return nil
}

// list returns the list of the objects to be reflected, with the given namespace as key
// (empty in case of cluster-scoped resources).
func (r *gvrReflector) list(namespace string) ([]interface{}, error) {
	localObjs, err := r.localLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}
	remoteObjs, err := r.remoteLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}

	list := make([]interface{}, 0, len(localObjs)+len(remoteObjs))
	for _, obj := range localObjs {
		meta, ok := obj.(metav1.Object)
		if !ok {
			continue
		}
		list = append(list, types.NamespacedName{Name: meta.GetName(), Namespace: namespace})
	}
	for _, obj := range remoteObjs {
		meta, ok := obj.(metav1.Object)
		if !ok {
			continue
		}
		list = append(list, types.NamespacedName{Name: meta.GetName(), Namespace: namespace})
	}
	return list, nil
}
