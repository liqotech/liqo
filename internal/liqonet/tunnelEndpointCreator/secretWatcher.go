// Copyright 2019-2021 The Liqo Authors
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

package tunnelEndpointCreator

import (
	"context"
	"strings"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	crdreplicator "github.com/liqotech/liqo/internal/crdReplicator"
	"github.com/liqotech/liqo/pkg/liqonet/tunnel/wireguard"
)

var (
	secretResource = "secrets"
)

func (tec *TunnelEndpointCreator) StartSecretWatcher() {
	ctx := context.Background()
	started := tec.Manager.GetCache().WaitForCacheSync(ctx)
	if !started {
		klog.Errorf("unable to sync caches")
		return
	}
	dynFactory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(tec.DynClient, ResyncPeriod, tec.Namespace, setSecretFilteringLabel)
	go tec.Watcher(dynFactory, corev1.SchemeGroupVersion.WithResource(secretResource), cache.ResourceEventHandlerFuncs{
		AddFunc:    tec.secretHandlerAdd,
		UpdateFunc: tec.secretHandlerUpdate,
	}, tec.secretClusterStopChan)
}

func (tec *TunnelEndpointCreator) secretHandlerAdd(obj interface{}) {
	tec.Mutex.Lock()
	defer tec.Mutex.Unlock()
	objUnstruct, ok := obj.(*unstructured.Unstructured)
	if !ok {
		klog.Errorf("an error occurred while converting interface to unstructured object")
		return
	}
	s := &corev1.Secret{}
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(objUnstruct.Object, s)
	if err != nil {
		klog.Errorf("an error occurred while converting resource %s of type %s to typed object: %s", objUnstruct.GetName(), objUnstruct.GetKind(), err)
		return
	}
	pubKeyByte, found := s.Data[wireguard.PublicKey]
	if !found {
		klog.Errorf("no data with key '%s' found in secret %s", wireguard.PublicKey, s.GetName())
		return
	}
	pubKey, err := wgtypes.ParseKey(string(pubKeyByte))
	if err != nil {
		klog.Errorf("secret named %s: publicKey for wireguard backend has not been set yet", s.Name)
		return
	}

	if pubKey.String() == tec.wgPubKey {
		return
	}
	tec.wgPubKey = pubKey.String()
	if !tec.wgConfigured {
		tec.WaitConfig.Done()
		klog.Infof("called done on waitgroup")
		tec.wgConfigured = true
	}
	netConfigs := &netv1alpha1.NetworkConfigList{}
	labels := client.MatchingLabels{crdreplicator.LocalLabelSelector: "true"}
	err = tec.Client.List(context.Background(), netConfigs, labels)
	if err != nil {
		klog.Errorf("unable to retrieve the existing resources of type %s in order to update the publicKey for the vpn backend: %v", netv1alpha1.NetworkConfigGroupVersionResource.String(), err)
		return
	}
	for _, nc := range netConfigs.Items {
		retryError := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			var netConfig netv1alpha1.NetworkConfig
			err := tec.Get(context.Background(), client.ObjectKey{
				Name:      nc.GetName(),
				Namespace: nc.GetNamespace(),
			}, &netConfig)
			if err != nil {
				klog.Errorf("an error occurred while retrieving resource of type %s named %s/%s: %v",
					netv1alpha1.NetworkConfigGroupVersionResource.String(), nc.GetNamespace(), nc.GetName(), err)
				return err
			}
			netConfig.Spec.BackendConfig[wireguard.PublicKey] = pubKey.String()
			err = tec.Update(context.Background(), &netConfig)
			return err
		})
		if retryError != nil {
			klog.Errorf("an error occurred while updating spec of networkConfig resource %s: %s", nc.GetName(), retryError)
		}
	}
}

func (tec *TunnelEndpointCreator) secretHandlerUpdate(oldObj interface{}, newObj interface{}) {
	tec.secretHandlerAdd(newObj)
}

func setSecretFilteringLabel(options *metav1.ListOptions) {
	//we want to watch only the resources that should be replicated on a remote cluster
	if options.LabelSelector == "" {
		newLabelSelector := []string{wireguard.KeysLabel, "=", wireguard.DriverName}
		options.LabelSelector = strings.Join(newLabelSelector, "")
	} else {
		newLabelSelector := []string{options.LabelSelector, wireguard.KeysLabel, "=", wireguard.DriverName}
		options.LabelSelector = strings.Join(newLabelSelector, "")
	}
}
