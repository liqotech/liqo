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

package wireguard

import (
	"context"
	"fmt"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/gateway"
	"github.com/liqotech/liqo/pkg/gateway/forge"
	"github.com/liqotech/liqo/pkg/gateway/tunnel/wireguard"
	liqolabels "github.com/liqotech/liqo/pkg/utils/labels"
)

const (
	wireguardVolumeName = "wireguard-config"
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

func podEnquerer(_ context.Context, obj client.Object) []ctrl.Request {
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		return nil
	}

	if pod.Labels == nil {
		return nil
	}
	gwName, ok := pod.Labels[consts.GatewayNameLabel]
	if !ok {
		return nil
	}
	gwNs, ok := pod.Labels[consts.GatewayNamespaceLabel]
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

// ensureKeysSecret ensure the presence of the private and public keys for the Wireguard interface and save them inside a Secret resource and Options.
func ensureKeysSecret(ctx context.Context, cl client.Client, wgObj metav1.Object, mode gateway.Mode) error {
	var controllerRef metav1.OwnerReference
	for _, ref := range wgObj.GetOwnerReferences() {
		if ref.Controller != nil && *ref.Controller {
			switch ref.Kind {
			case networkingv1beta1.GatewayClientKind:
				controllerRef = ref
			case networkingv1beta1.GatewayServerKind:
				controllerRef = ref
			}
			break
		}
	}

	opts := &gateway.Options{
		Name:            controllerRef.Name,
		Namespace:       wgObj.GetNamespace(),
		RemoteClusterID: wgObj.GetLabels()[consts.RemoteClusterID],
		GatewayUID:      string(controllerRef.UID),
		Mode:            mode,
	}

	_, err := getWireGuardSecret(ctx, cl, wgObj)
	switch {
	case kerrors.IsNotFound(err):
		pri, err := wgtypes.GeneratePrivateKey()
		if err != nil {
			klog.Error(err)
			return err
		}
		pub := pri.PublicKey()
		if err := wireguard.CreateKeysSecret(ctx, cl, opts, pri, pub); err != nil {
			klog.Error(err)
			return err
		}
		klog.Infof("Keys secret for WireGuard gateway %q correctly enforced", wgObj.GetName())
		return nil
	case err != nil:
		klog.Error(err)
		return err
	default:
		return nil
	}
}

func checkExistingKeysSecret(ctx context.Context, cl client.Client, secretName, namespace string) error {
	var s corev1.Secret
	if err := cl.Get(ctx, types.NamespacedName{Name: secretName, Namespace: namespace}, &s); err != nil {
		return err
	}

	// check labels
	if s.Labels == nil {
		return fmt.Errorf("mandatory labels %q: \"true\" and %q are missing in secret %q", consts.GatewayResourceLabel, consts.RemoteClusterID, secretName)
	}

	if s.Labels[consts.GatewayResourceLabel] != consts.GatewayResourceLabelValue {
		return fmt.Errorf("missing %q: \"true\" label in secret %q", consts.GatewayResourceLabel, secretName)
	}
	if v, ok := s.Labels[consts.RemoteClusterID]; !ok || v == "" {
		return fmt.Errorf("missing %q label in secret %q", consts.RemoteClusterID, secretName)
	}

	return nil
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
		err = kerrors.NewNotFound(corev1.Resource("Secret"), wgObjNsName.Name)
		return nil, err
	case 1:
		return &secrets.Items[0], nil
	default:
		return nil, fmt.Errorf("found multiple secrets associated to WireGuard gateway %q", wgObjNsName)
	}
}
