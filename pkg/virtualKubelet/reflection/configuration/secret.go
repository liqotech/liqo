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

package configuration

import (
	"context"

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

const (
	// SecretReflectorName is the name associated with the Secret reflector.
	SecretReflectorName = "Secret"
)

// NamespacedSecretReflector manages the Secret reflection.
type NamespacedSecretReflector struct {
	generic.NamespacedReflector

	localSecrets        corev1listers.SecretNamespaceLister
	remoteSecrets       corev1listers.SecretNamespaceLister
	remoteSecretsClient corev1clients.SecretInterface
}

// NewSecretReflector builds a SecretReflector.
func NewSecretReflector(workers uint) manager.Reflector {
	return generic.NewReflector(SecretReflectorName, NewNamespacedSecretReflector, generic.WithoutFallback(), workers)
}

// NewNamespacedSecretReflector returns a function generating NamespacedSecretReflector instances.
func NewNamespacedSecretReflector(opts *options.NamespacedOpts) manager.NamespacedReflector {
	local := opts.LocalFactory.Core().V1().Secrets()
	remote := opts.RemoteFactory.Core().V1().Secrets()

	// Using opts.LocalNamespace for both event handlers so that the object will be put in the same workqueue
	// no matter the cluster, hence it will be processed by the handle function in the same way.
	local.Informer().AddEventHandler(opts.HandlerFactory(generic.NamespacedKeyer(opts.LocalNamespace)))
	remote.Informer().AddEventHandler(opts.HandlerFactory(generic.NamespacedKeyer(opts.LocalNamespace)))

	return &NamespacedSecretReflector{
		NamespacedReflector: generic.NewNamespacedReflector(opts),
		localSecrets:        local.Lister().Secrets(opts.LocalNamespace),
		remoteSecrets:       remote.Lister().Secrets(opts.RemoteNamespace),
		remoteSecretsClient: opts.RemoteClient.CoreV1().Secrets(opts.RemoteNamespace),
	}
}

// Handle is responsible for reconciling the given object and ensuring it is correctly reflected.
func (nsr *NamespacedSecretReflector) Handle(ctx context.Context, name string) error {
	tracer := trace.FromContext(ctx)

	// Retrieve the local and remote objects (only not found errors can occur).
	klog.V(4).Infof("Handling reflection of local Secret %q (remote: %q)", nsr.LocalRef(name), nsr.RemoteRef(name))

	local, lerr := nsr.localSecrets.Get(name)
	utilruntime.Must(client.IgnoreNotFound(lerr))
	remote, rerr := nsr.remoteSecrets.Get(name)
	utilruntime.Must(client.IgnoreNotFound(rerr))
	tracer.Step("Retrieved the local and remote objects")

	// Abort the reflection if the remote object is not managed by us, as we do not want to mutate others' objects.
	if rerr == nil && !forge.IsReflected(remote) {
		if lerr == nil { // Do not output the warning event in case the event was triggered by the remote object (i.e., the local one does not exists).
			klog.Infof("Skipping reflection of local Secret %q as remote already exists and is not managed by us", nsr.LocalRef(name))
		}
		return nil
	}
	tracer.Step("Performed the sanity checks")

	if kerrors.IsNotFound(lerr) {
		defer tracer.Step("Ensured the absence of the remote object")
		if !kerrors.IsNotFound(rerr) {
			klog.V(4).Infof("Deleting remote Secret %q, since local %q does no longer exist", nsr.RemoteRef(name), nsr.LocalRef(name))
			return nsr.DeleteRemote(ctx, nsr.remoteSecretsClient, SecretReflectorName, name, remote.GetUID())
		}

		klog.V(4).Infof("Local Secret %q and remote Secret %q both vanished", nsr.LocalRef(name), nsr.RemoteRef(name))
		return nil
	}

	// Forge the mutation to be applied to the remote cluster.
	mutation := forge.RemoteSecret(local, nsr.RemoteNamespace())
	tracer.Step("Remote mutation created")

	defer tracer.Step("Enforced the correctness of the remote object")
	if _, err := nsr.remoteSecretsClient.Apply(ctx, mutation, forge.ApplyOptions()); err != nil {
		klog.Errorf("Failed to enforce remote Secret %q (local: %q): %v", nsr.RemoteRef(name), nsr.LocalRef(name), err)
		return err
	}

	klog.Infof("Remote Secret %q successfully enforced (local: %q)", nsr.RemoteRef(name), nsr.LocalRef(name))

	return nil
}
