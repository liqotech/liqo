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

package generic

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/options"
)

// NamespacedReflector implements the logic common to all namespaced reflectors.
type NamespacedReflector struct {
	record.EventRecorder

	ready func() bool

	local  string
	remote string

	reflectionType offloadingv1beta1.ReflectionType

	ForgingOpts *forge.ForgingOpts
}

// ResourceDeleter know how to delete a Kubernetes object with the given name.
type ResourceDeleter interface {
	Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error
}

// NewNamespacedReflector returns a new NamespacedReflector for the given namespaces.
func NewNamespacedReflector(opts *options.NamespacedOpts, name string) NamespacedReflector {
	return NamespacedReflector{
		EventRecorder: opts.EventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: "liqo-" + strings.ToLower(name) + "-reflection"}),
		local:         opts.LocalNamespace, remote: opts.RemoteNamespace, ready: opts.Ready,
		reflectionType: opts.ReflectionType, ForgingOpts: opts.ForgingOpts,
	}
}

// Ready returns whether the NamespacedReflector is completely initialized.
func (gnr *NamespacedReflector) Ready() bool {
	return gnr.ready()
}

// LocalNamespace returns the local namespace associated with the reflector.
func (gnr *NamespacedReflector) LocalNamespace() string {
	return gnr.local
}

// LocalRef returns the ObjectRef associated with the local namespace.
func (gnr *NamespacedReflector) LocalRef(name string) klog.ObjectRef {
	return klog.KRef(gnr.local, name)
}

// RemoteNamespace returns the remote namespace associated with the reflector.
func (gnr *NamespacedReflector) RemoteNamespace() string {
	return gnr.remote
}

// RemoteRef returns the ObjectRef associated with the remote namespace.
func (gnr *NamespacedReflector) RemoteRef(name string) klog.ObjectRef {
	return klog.KRef(gnr.remote, name)
}

// DeleteRemote deletes the given remote resource from the cluster.
func (gnr *NamespacedReflector) DeleteRemote(ctx context.Context, deleter ResourceDeleter, resource, name string, uid types.UID) error {
	err := deleter.Delete(ctx, name, *metav1.NewPreconditionDeleteOptions(string(uid)))
	if err != nil && !kerrors.IsNotFound(err) {
		klog.Errorf("Failed to delete remote %v %q: %v", resource, gnr.RemoteRef(name), err)
		return err
	}

	klog.Infof("Remote %v %q successfully deleted", resource, gnr.RemoteRef(name))
	return nil
}

// DeleteLocal deletes the given local resource from the cluster.
func (gnr *NamespacedReflector) DeleteLocal(ctx context.Context, deleter ResourceDeleter, resource, name string, uid types.UID) error {
	err := deleter.Delete(ctx, name, *metav1.NewPreconditionDeleteOptions(string(uid)))
	if err != nil && !kerrors.IsNotFound(err) {
		klog.Errorf("Failed to delete local %v %q: %v", resource, gnr.LocalRef(name), err)
		return err
	}

	klog.Infof("Local %v %q successfully deleted", resource, gnr.LocalRef(name))
	return nil
}

// ShouldSkipReflection returns whether the reflection of the given object should be skipped.
func (gnr *NamespacedReflector) ShouldSkipReflection(obj metav1.Object) (bool, error) {
	switch gnr.reflectionType {
	case offloadingv1beta1.AllowList:
		value, ok := obj.GetAnnotations()[consts.AllowReflectionAnnotationKey]
		if ok && strings.EqualFold(value, "false") {
			return true, nil
		}
		return !ok, nil
	case offloadingv1beta1.DenyList:
		value, ok := obj.GetAnnotations()[consts.SkipReflectionAnnotationKey]
		if ok && strings.EqualFold(value, "false") {
			return false, nil
		}
		return ok, nil
	default:
		return true, fmt.Errorf("ReflectionType value %q not supported", gnr.reflectionType)
	}
}

// GetReflectionType returns the reflection type of the reflector.
func (gnr *NamespacedReflector) GetReflectionType() offloadingv1beta1.ReflectionType {
	return gnr.reflectionType
}

// ForcedAllowOrSkip checks whether the given object is *explicitly* marked to be allowed or skipped
// (i.e., it has the allow or the deny annotation), independently from the reflection policy.
// If so, it returns whether the object should be skipped, or an error if unable to determine it.
// Otherwise, it return a nil bool as it is undeterminated, since we are not considering the reflection
// policy at this stage.
func (gnr *NamespacedReflector) ForcedAllowOrSkip(obj metav1.Object) (*bool, error) {
	allowAnnot, skipAnnot := false, false

	value, ok := obj.GetAnnotations()[consts.AllowReflectionAnnotationKey]
	if ok && !strings.EqualFold(value, "false") {
		allowAnnot = true
	}
	value, ok = obj.GetAnnotations()[consts.SkipReflectionAnnotationKey]
	if ok && !strings.EqualFold(value, "false") {
		skipAnnot = true
	}

	if allowAnnot && skipAnnot {
		return nil, fmt.Errorf("object %q can't have both the allow and deny annotations set", klog.KObj(obj))
	}

	switch {
	case allowAnnot:
		return pointer.Bool(false), nil
	case skipAnnot:
		return pointer.Bool(true), nil
	default:
		return nil, nil
	}
}
