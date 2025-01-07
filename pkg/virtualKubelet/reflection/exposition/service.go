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

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/pkg/utils/virtualkubelet"
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

	enableLoadBalancer              bool
	remoteRealLoadBalancerClassName string
}

// NewServiceReflector returns a new ServiceReflector instance.
func NewServiceReflector(reflectorConfig *offloadingv1beta1.ReflectorConfig,
	enableLoadBalancer bool, remoteRealLoadBalancerClassName string) manager.Reflector {
	return generic.NewReflector(ServiceReflectorName,
		NewNamespacedServiceReflector(enableLoadBalancer, remoteRealLoadBalancerClassName), generic.WithoutFallback(),
		reflectorConfig.NumWorkers, reflectorConfig.Type, generic.ConcurrencyModeLeader)
}

// NewNamespacedServiceReflector returns a new NamespacedServiceReflector instance.
func NewNamespacedServiceReflector(
	enableLoadBalancer bool, remoteRealLoadBalancerClassName string) func(*options.NamespacedOpts) manager.NamespacedReflector {
	return func(opts *options.NamespacedOpts) manager.NamespacedReflector {
		local := opts.LocalFactory.Core().V1().Services()
		remote := opts.RemoteFactory.Core().V1().Services()

		_, err := local.Informer().AddEventHandler(opts.HandlerFactory(generic.NamespacedKeyer(opts.LocalNamespace)))
		utilruntime.Must(err)
		_, err = remote.Informer().AddEventHandler(opts.HandlerFactory(generic.NamespacedKeyer(opts.LocalNamespace)))
		utilruntime.Must(err)

		return &NamespacedServiceReflector{
			NamespacedReflector:             generic.NewNamespacedReflector(opts, ServiceReflectorName),
			localServices:                   local.Lister().Services(opts.LocalNamespace),
			remoteServices:                  remote.Lister().Services(opts.RemoteNamespace),
			remoteServicesClient:            opts.RemoteClient.CoreV1().Services(opts.RemoteNamespace),
			enableLoadBalancer:              enableLoadBalancer,
			remoteRealLoadBalancerClassName: remoteRealLoadBalancerClassName,
		}
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
		if lerr == nil { // Do not output the warning event in case the event was triggered by the remote object (i.e., the local one does not exists).
			klog.Infof("Skipping reflection of local Service %q as remote already exists and is not managed by us", nsr.LocalRef(name))
			nsr.Event(local, corev1.EventTypeWarning, forge.EventFailedReflection, forge.EventFailedReflectionAlreadyExistsMsg())
		}
		return nil
	}

	// Abort the reflection if the local object has the "skip-reflection" annotation.
	if !kerrors.IsNotFound(lerr) {
		skipReflection, err := nsr.ShouldSkipReflection(local)
		if err != nil {
			klog.Errorf("Failed to check whether local Service %q should be reflected: %v", nsr.LocalRef(name), err)
			return err
		}
		if skipReflection {
			if nsr.GetReflectionType() == offloadingv1beta1.DenyList {
				klog.Infof("Skipping reflection of local Service %q as marked with the skip annotation", nsr.LocalRef(name))
			} else { // AllowList
				klog.Infof("Skipping reflection of local Service %q as not marked with the allow annotation", nsr.LocalRef(name))
			}
			nsr.Event(local, corev1.EventTypeNormal, forge.EventReflectionDisabled, forge.EventObjectReflectionDisabledMsg(nsr.GetReflectionType()))
			if kerrors.IsNotFound(rerr) { // The remote object does not already exist, hence no further action is required.
				return nil
			}

			// Otherwise, let pretend the local object does not exist, so that the remote one gets deleted.
			lerr = kerrors.NewNotFound(corev1.Resource("service"), local.GetName())
		}
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
	mutation := forge.RemoteService(local, nsr.RemoteNamespace(), nsr.enableLoadBalancer, nsr.remoteRealLoadBalancerClassName, nsr.ForgingOpts)
	tracer.Step("Remote mutation created")

	defer tracer.Step("Enforced the correctness of the remote object")
	if _, err := nsr.remoteServicesClient.Apply(ctx, mutation, forge.ApplyOptions()); err != nil {
		klog.Errorf("Failed to enforce remote Service %q (local: %q): %v", nsr.RemoteRef(name), nsr.LocalRef(name), err)
		nsr.Event(local, corev1.EventTypeWarning, forge.EventFailedReflection, forge.EventFailedReflectionMsg(err))
		return err
	}

	klog.Infof("Remote Service %q successfully enforced (local: %q)", nsr.RemoteRef(name), nsr.LocalRef(name))
	nsr.Event(local, corev1.EventTypeNormal, forge.EventSuccessfulReflection, forge.EventSuccessfulReflectionMsg())

	return nil
}

// List returns the list of services to be reflected.
func (nsr *NamespacedServiceReflector) List() ([]interface{}, error) {
	return virtualkubelet.List[virtualkubelet.Lister[*corev1.Service], *corev1.Service](
		nsr.localServices,
		nsr.remoteServices,
	)
}
