// Copyright 2019-2024 The Liqo Authors
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
	"strconv"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/gateway/forge"
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
				Name:      forge.GatewayResourceName(secret.Name),
			},
		},
	}
}

func clusterRoleBindingEnquerer(_ context.Context, obj client.Object) []ctrl.Request {
	crb, ok := obj.(*rbacv1.ClusterRoleBinding)
	if !ok {
		return nil
	}

	if crb.Labels == nil {
		return nil
	}
	gwName, ok := crb.Labels[consts.GatewayNameLabel]
	if !ok {
		return nil
	}
	gwNs, ok := crb.Labels[consts.GatewayNamespaceLabel]
	if !ok {
		return nil
	}

	return []ctrl.Request{
		{
			NamespacedName: types.NamespacedName{
				Namespace: gwNs,
				Name:      gwName,
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

func checkServiceOverrides(service *corev1.Service, addresses *[]string, port *int32) error {
	if service == nil {
		return nil
	}

	if addresses == nil || port == nil {
		return fmt.Errorf("addresses and port must be non-nil")
	}

	if service.Annotations != nil {
		if v, ok := service.Annotations[consts.OverrideAddressAnnotation]; ok {
			*addresses = []string{v}
		}
		if v, ok := service.Annotations[consts.OverridePortAnnotation]; ok {
			p, err := strconv.ParseInt(v, 10, 32)
			if err != nil {
				klog.Errorf("unable to parse port %q from service %s/%s annotation: %v", v, service.Namespace, service.Name, err)
				return err
			}
			*port = int32(p)
		}
	}
	return nil
}
