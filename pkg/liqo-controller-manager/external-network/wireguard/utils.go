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

package wireguard

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/liqotech/liqo/pkg/consts"
	tunnel "github.com/liqotech/liqo/pkg/gateway/tunnel/wireguard"
	liqolabels "github.com/liqotech/liqo/pkg/utils/labels"
)

func filterWireGuardSecretsPredicate() predicate.Predicate {
	filterGatewayResources, err := predicate.LabelSelectorPredicate(liqolabels.GatewayResourceLabelSelector)
	utilruntime.Must(err)

	filterResourcesForRemote, err := predicate.LabelSelectorPredicate(liqolabels.ResourceForRemoteClusterLabelSelector)
	utilruntime.Must(err)

	return predicate.And(filterGatewayResources, filterResourcesForRemote)
}

func wireGuardSecretEnquerer(_ context.Context, obj client.Object) []ctrl.Request {
	secret, ok := obj.(*corev1.Secret)
	if !ok {
		return nil
	}

	return []ctrl.Request{
		{
			NamespacedName: types.NamespacedName{
				Namespace: secret.Namespace,
				Name:      tunnel.GenerateResourceName(secret.Name),
			},
		},
	}
}

func getWireGuardSecret(ctx context.Context, cl client.Client, wgObj metav1.Object) (*corev1.Secret, error) {
	wgObjNsName := types.NamespacedName{Name: wgObj.GetName(), Namespace: wgObj.GetNamespace()}

	remoteClusterID, exists := wgObj.GetLabels()[consts.RemoteClusterID]
	if !exists {
		err := fmt.Errorf("missing %q label in WireGuard gateway %q", consts.RemoteClusterID, wgObjNsName)
		klog.Error(err)
		return nil, err
	}
	wgSecretSelector := client.MatchingLabels{
		consts.GatewayResourceLabel: consts.GatewayResourceLabelValue,
		consts.RemoteClusterID:      remoteClusterID,
	}

	var secrets corev1.SecretList
	err := cl.List(ctx, &secrets, client.InNamespace(wgObj.GetNamespace()), wgSecretSelector)
	if err != nil {
		klog.Errorf("Unable to list secrets associated to WireGuard gateway %q: %v", wgObjNsName, err)
		return nil, err
	}

	switch len(secrets.Items) {
	case 0:
		klog.Warningf("Secret associated to WireGuard gateway %q not found", wgObjNsName)
		return nil, nil
	case 1:
		return &secrets.Items[0], nil
	default:
		return nil, fmt.Errorf("found multiple secrets associated to WireGuard gateway %q", wgObjNsName)
	}
}
