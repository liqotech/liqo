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

package generic

import (
	"context"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"

	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/options"
)

// NamespacedReflector implements the logic common to all namespaced reflectors.
type NamespacedReflector struct {
	ready func() bool

	local  string
	remote string
}

// ResourceDeleter know how to delete a Kubernetes object with the given name.
type ResourceDeleter interface {
	Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error
}

// NewNamespacedReflector returns a new NamespacedReflector for the given namespaces.
func NewNamespacedReflector(opts *options.NamespacedOpts) NamespacedReflector {
	return NamespacedReflector{local: opts.LocalNamespace, remote: opts.RemoteNamespace, ready: opts.Ready}
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
		klog.Errorf("Failed to delete remote %v %q (local: %q): %v", resource, gnr.RemoteRef(name), gnr.LocalRef(name), err)
		return err
	}

	klog.Infof("Remote %v %q successfully deleted (local: %q)", resource, gnr.RemoteRef(name), gnr.LocalRef(name))
	return nil
}
