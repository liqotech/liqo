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

package exposition

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
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

var _ manager.NamespacedReflector = (*NamespacedServiceReflector)(nil)

const (
	// ServiceReflectorName -> The name associated with the Service reflector.
	ServiceReflectorName = "Service"

	// kubernetesServiceName -> The name of the kubernetes service.
	kubernetesServiceName = "kubernetes"
)

// NamespacedServiceReflector manages the Service reflection for a given pair of local and remote namespaces.
type NamespacedServiceReflector struct {
	generic.NamespacedReflector

	localServices        corev1listers.ServiceNamespaceLister
	remoteServices       corev1listers.ServiceNamespaceLister
	remoteServicesClient corev1clients.ServiceInterface
}

// NewServiceReflector returns a new ServiceReflector instance.
func NewServiceReflector(workers uint) manager.Reflector {
	return generic.NewReflector(ServiceReflectorName, NewNamespacedServiceReflector, generic.WithoutFallback(), workers)
}

// NewNamespacedServiceReflector returns a new NamespacedServiceReflector instance.
func NewNamespacedServiceReflector(opts *options.NamespacedOpts) manager.NamespacedReflector {
	local := opts.LocalFactory.Core().V1().Services()
	remote := opts.RemoteFactory.Core().V1().Services()

	local.Informer().AddEventHandler(opts.HandlerFactory(generic.NamespacedKeyer(opts.LocalNamespace)))
	remote.Informer().AddEventHandler(opts.HandlerFactory(generic.NamespacedKeyer(opts.LocalNamespace)))

	return &NamespacedServiceReflector{
		NamespacedReflector:  generic.NewNamespacedReflector(opts),
		localServices:        local.Lister().Services(opts.LocalNamespace),
		remoteServices:       remote.Lister().Services(opts.RemoteNamespace),
		remoteServicesClient: opts.RemoteClient.CoreV1().Services(opts.RemoteNamespace),
	}
}

// Handle reconciles service objects.
func (nsr *NamespacedServiceReflector) Handle(ctx context.Context, name string) error {
	tracer := trace.FromContext(ctx)

	// Skip the kubernetes service, as meaningless to reflect.
	if nsr.LocalNamespace() == corev1.NamespaceDefault && name == kubernetesServiceName {
		klog.V(4).Infof("Skipping reflection of local Service %q as blacklisted", nsr.LocalRef(name))
		return nil
	}

	// Retrieve the local and remote objects (only not found errors can occur).
	klog.V(4).Infof("Handling reflection of local Service %q (remote: %q)", nsr.LocalRef(name), nsr.RemoteRef(name))
	local, lerr := nsr.localServices.Get(name)
	utilruntime.Must(client.IgnoreNotFound(lerr))
	remote, rerr := nsr.remoteServices.Get(name)
	utilruntime.Must(client.IgnoreNotFound(rerr))
	tracer.Step("Retrieved the local and remote objects")

	// Abort the reflection if the remote object is not managed by us, as we do not want to mutate others' objects.
	if rerr == nil && !forge.IsReflected(remote) {
		klog.Infof("Skipping reflection of local Service %q as remote already exists and is not managed by us", nsr.LocalRef(name))
		return nil
	}
	tracer.Step("Performed the sanity checks")

	// The local service does no longer exist. Ensure it is also absent from the remote cluster.
	if kerrors.IsNotFound(lerr) {
		defer tracer.Step("Ensured the absence of the remote object")
		if !kerrors.IsNotFound(rerr) {
			klog.V(4).Infof("Deleting remote Service %q, since local %q does no longer exist", nsr.RemoteRef(name), nsr.LocalRef(name))
			return nsr.DeleteRemote(ctx, nsr.remoteServicesClient, ServiceReflectorName, name, remote.GetUID())
		}

		klog.V(4).Infof("Local Service %q and remote Service %q both vanished", nsr.LocalRef(name), nsr.RemoteRef(name))
		return nil
	}

	// Forge the mutation to be applied to the remote cluster.
	mutation := forge.RemoteService(local, nsr.RemoteNamespace())
	tracer.Step("Remote mutation created")

	defer tracer.Step("Enforced the correctness of the remote object")
	if _, err := nsr.remoteServicesClient.Apply(ctx, mutation, forge.ApplyOptions()); err != nil {
		klog.Errorf("Failed to enforce remote Service %q (local: %q): %v", nsr.RemoteRef(name), nsr.LocalRef(name), err)
		return err
	}

	klog.Infof("Remote Service %q successfully enforced (local: %q)", nsr.RemoteRef(name), nsr.LocalRef(name))
	return nil
}
