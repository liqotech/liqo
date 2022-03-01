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

package netcfgcreator

import (
	"context"
	"sync"

	corev1 "k8s.io/api/core/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/utils/getters"
	liqolabels "github.com/liqotech/liqo/pkg/utils/labels"
)

// SecretWatcher reconciles Secret objects to retrieve the Wireguard public key.
type SecretWatcher struct {
	sync.RWMutex
	wiregardPublicKey string

	configured bool
	wait       chan struct{}

	enqueuefn func(workqueue.RateLimitingInterface)
}

// NewSecretWatcher returns a new initialized SecretWatcher instance.
func NewSecretWatcher(enqueuefn func(workqueue.RateLimitingInterface)) *SecretWatcher {
	return &SecretWatcher{
		configured: false,
		wait:       make(chan struct{}),

		enqueuefn: enqueuefn,
	}
}

// WiregardPublicKey returns the retrieved Wireguard public key.
func (sw *SecretWatcher) WiregardPublicKey() string {
	sw.RLock()
	defer sw.RUnlock()

	return sw.wiregardPublicKey
}

// WaitForConfigured waits until a valid key is retrieved for the first time.
func (sw *SecretWatcher) WaitForConfigured(ctx context.Context) bool {
	sw.RLock()

	if !sw.configured {
		sw.RUnlock()
		klog.Info("Waiting for the configuration of the secret watcher")

		select {
		case <-sw.wait:
			klog.Info("Secret watcher correctly configured")
			return true
		case <-ctx.Done():
			klog.Warning("Context expired before configuring the secret watcher")
			return false
		}
	}

	sw.RUnlock()
	return true
}

// Handlers returns the set of handlers used for the Watch configuration.
func (sw *SecretWatcher) Handlers() handler.EventHandler {
	return handler.Funcs{
		CreateFunc: func(ce event.CreateEvent, rli workqueue.RateLimitingInterface) {
			secret := ce.Object.(*corev1.Secret)
			sw.handle(secret, rli)
		},
		UpdateFunc: func(ue event.UpdateEvent, rli workqueue.RateLimitingInterface) {
			secret := ue.ObjectNew.(*corev1.Secret)
			sw.handle(secret, rli)
		},
	}
}

// Predicates returns the set of predicates used for the Watch configuration.
func (sw *SecretWatcher) Predicates() predicate.Predicate {
	secretsPredicate, err := predicate.LabelSelectorPredicate(liqolabels.WireGuardSecretLabelSelector)
	utilruntime.Must(err)

	return secretsPredicate
}

// handle processes creation and update events of a Secret object.
func (sw *SecretWatcher) handle(secret *corev1.Secret, rli workqueue.RateLimitingInterface) {
	klog.V(4).Infof("Handling Secret %q", klog.KObj(secret))

	sw.Lock()
	defer sw.Unlock()

	pubKey, err := getters.RetrieveWGPubKeyFromSecret(secret, consts.PublicKey)
	if err != nil {
		klog.Error(err)
		return
	}

	// The key did not change, nothing to do
	if pubKey.String() == sw.wiregardPublicKey {
		return
	}

	// Configure the new key, and set as configured if not yet done
	klog.Infof("Wiregard public key correctly retrieved")
	sw.wiregardPublicKey = pubKey.String()
	if !sw.configured {
		close(sw.wait)
		sw.configured = true
	}

	// Enqueue all foreign clusters for update (which in turn update the respective network configs)
	sw.enqueuefn(rli)
}
