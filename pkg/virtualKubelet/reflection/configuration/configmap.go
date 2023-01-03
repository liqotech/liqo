// Copyright 2019-2023 The Liqo Authors
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

package configuration

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	corev1clients "k8s.io/client-go/kubernetes/typed/core/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/klog/v2"
	"k8s.io/utils/trace"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/generic"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/manager"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/options"
)

const (
	// ConfigMapReflectorName is the name associated with the ConfigMap reflector.
	ConfigMapReflectorName = "ConfigMap"
)

// NamespacedConfigMapReflector manages the ConfigMap reflection.
type NamespacedConfigMapReflector struct {
	generic.NamespacedReflector

	localConfigMaps        corev1listers.ConfigMapNamespaceLister
	remoteConfigMaps       corev1listers.ConfigMapNamespaceLister
	remoteConfigMapsClient corev1clients.ConfigMapInterface
}

// NewConfigMapReflector builds a ConfigMapReflector.
func NewConfigMapReflector(workers uint) manager.Reflector {
	return generic.NewReflector(ConfigMapReflectorName, NewNamespacedConfigMapReflector, generic.WithoutFallback(), workers)
}

// RemoteConfigMapNamespacedKeyer returns a keyer associated with the given namespace,
// which accounts for the root CA configmap name remapping.
func RemoteConfigMapNamespacedKeyer(namespace string) func(metadata metav1.Object) []types.NamespacedName {
	return func(metadata metav1.Object) []types.NamespacedName {
		return []types.NamespacedName{{Namespace: namespace, Name: forge.LocalConfigMapName(metadata.GetName())}}
	}
}

// NewNamespacedConfigMapReflector returns a function generating NamespacedConfigMapReflector instances.
func NewNamespacedConfigMapReflector(opts *options.NamespacedOpts) manager.NamespacedReflector {
	local := opts.LocalFactory.Core().V1().ConfigMaps()
	remote := opts.RemoteFactory.Core().V1().ConfigMaps()

	// Using opts.LocalNamespace for both event handlers so that the object will be put in the same workqueue
	// no matter the cluster, hence it will be processed by the handle function in the same way.
	local.Informer().AddEventHandler(opts.HandlerFactory(generic.NamespacedKeyer(opts.LocalNamespace)))
	remote.Informer().AddEventHandler(opts.HandlerFactory(RemoteConfigMapNamespacedKeyer(opts.LocalNamespace)))

	return &NamespacedConfigMapReflector{
		NamespacedReflector:    generic.NewNamespacedReflector(opts, ConfigMapReflectorName),
		localConfigMaps:        local.Lister().ConfigMaps(opts.LocalNamespace),
		remoteConfigMaps:       remote.Lister().ConfigMaps(opts.RemoteNamespace),
		remoteConfigMapsClient: opts.RemoteClient.CoreV1().ConfigMaps(opts.RemoteNamespace),
	}
}

// RemoteRef returns the ObjectRef associated with the remote namespace.
func (ncr *NamespacedConfigMapReflector) RemoteRef(name string) klog.ObjectRef {
	return klog.KRef(ncr.RemoteNamespace(), forge.RemoteConfigMapName(name))
}

// Handle is responsible for reconciling the given object and ensuring it is correctly reflected.
func (ncr *NamespacedConfigMapReflector) Handle(ctx context.Context, name string) error {
	tracer := trace.FromContext(ctx)

	// Retrieve the local and remote objects (only not found errors can occur).
	klog.V(4).Infof("Handling reflection of local ConfigMap %q (remote: %q)", ncr.LocalRef(name), ncr.RemoteRef(name))

	local, lerr := ncr.localConfigMaps.Get(name)
	utilruntime.Must(client.IgnoreNotFound(lerr))
	remote, rerr := ncr.remoteConfigMaps.Get(forge.RemoteConfigMapName(name))
	utilruntime.Must(client.IgnoreNotFound(rerr))
	tracer.Step("Retrieved the local and remote objects")

	// Abort the reflection if the remote object is not managed by us, as we do not want to mutate others' objects.
	if rerr == nil && !forge.IsReflected(remote) {
		if lerr == nil { // Do not output the warning event in case the event was triggered by the remote object (i.e., the local one does not exists).
			klog.Infof("Skipping reflection of local ConfigMap %q as remote already exists and is not managed by us", ncr.LocalRef(name))
			ncr.Event(local, corev1.EventTypeWarning, forge.EventFailedReflection, forge.EventFailedReflectionAlreadyExistsMsg())
		}
		return nil
	}

	// Abort the reflection if the local object has the "skip-reflection" annotation.
	if !kerrors.IsNotFound(lerr) && ncr.ShouldSkipReflection(local) {
		klog.Infof("Skipping reflection of local ConfigMap %q as marked with the skip annotation", ncr.LocalRef(name))
		ncr.Event(local, corev1.EventTypeNormal, forge.EventReflectionDisabled, forge.EventObjectReflectionDisabledMsg())
		if kerrors.IsNotFound(rerr) { // The remote object does not already exist, hence no further action is required.
			return nil
		}

		// Otherwise, let pretend the local object does not exist, so that the remote one gets deleted.
		lerr = kerrors.NewNotFound(corev1.Resource("configmap"), local.GetName())
	}

	tracer.Step("Performed the sanity checks")

	if kerrors.IsNotFound(lerr) {
		defer tracer.Step("Ensured the absence of the remote object")
		if !kerrors.IsNotFound(rerr) {
			klog.V(4).Infof("Deleting remote ConfigMap %q, since local %q does no longer exist", ncr.RemoteRef(name), ncr.LocalRef(name))
			return ncr.DeleteRemote(ctx, ncr.remoteConfigMapsClient, ConfigMapReflectorName, remote.GetName(), remote.GetUID())
		}

		klog.V(4).Infof("Local ConfigMap %q and remote ConfigMap %q both vanished", ncr.LocalRef(name), ncr.RemoteRef(name))
		return nil
	}

	// Forge the mutation to be applied to the remote cluster.
	mutation := forge.RemoteConfigMap(local, ncr.RemoteNamespace())
	tracer.Step("Remote mutation created")

	defer tracer.Step("Enforced the correctness of the remote object")
	if _, err := ncr.remoteConfigMapsClient.Apply(ctx, mutation, forge.ApplyOptions()); err != nil {
		klog.Errorf("Failed to enforce remote ConfigMap %q (local: %q): %v", ncr.RemoteRef(name), ncr.LocalRef(name), err)
		ncr.Event(local, corev1.EventTypeWarning, forge.EventFailedReflection, forge.EventFailedReflectionMsg(err))
		return err
	}

	klog.Infof("Remote ConfigMap %q successfully enforced (local: %q)", ncr.RemoteRef(name), ncr.LocalRef(name))
	ncr.Event(local, corev1.EventTypeNormal, forge.EventSuccessfulReflection, forge.EventSuccessfulReflectionMsg())

	return nil
}
