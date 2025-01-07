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

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/gateway"
	"github.com/liqotech/liqo/pkg/gateway/forge"
	"github.com/liqotech/liqo/pkg/utils/resource"
)

// CheckKeysSecret checks if the keys secret exists and if it contains the private and public keys.
func CheckKeysSecret(ctx context.Context, cl client.Client, opts *Options) (wgtypes.Key, error) {
	secret := &corev1.Secret{}
	if err := cl.Get(ctx, types.NamespacedName{
		Name:      forge.GatewayResourceName(opts.GwOptions.Name),
		Namespace: opts.GwOptions.Namespace,
	}, secret); err != nil {
		return wgtypes.Key{}, err
	}
	if secret.Data == nil {
		return wgtypes.Key{}, nil
	}
	if k, ok := secret.Data[consts.PrivateKeyField]; !ok || len(k) != wgtypes.KeyLen {
		return wgtypes.Key{}, nil
	}
	if k, ok := secret.Data[consts.PublicKeyField]; !ok || len(k) != wgtypes.KeyLen {
		return wgtypes.Key{}, nil
	}
	return wgtypes.Key(secret.Data[consts.PrivateKeyField]), nil
}

// CreateKeysSecret creates the private and public keys for the Wireguard interface and save them inside a Secret resource.
func CreateKeysSecret(ctx context.Context, cl client.Client, opts *gateway.Options, pri, pub wgtypes.Key) error {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      forge.GatewayResourceName(opts.Name),
			Namespace: opts.Namespace,
		},
	}

	if _, err := resource.CreateOrUpdate(ctx, cl, secret, func() error {
		secret.SetLabels(map[string]string{
			string(consts.RemoteClusterID):      opts.RemoteClusterID,
			string(consts.GatewayResourceLabel): string(consts.GatewayResourceLabelValue),
		})
		if err := gateway.SetOwnerReferenceWithMode(opts, secret, cl.Scheme()); err != nil {
			return err
		}
		secret.Data = map[string][]byte{
			consts.PrivateKeyField: pri[:],
			consts.PublicKeyField:  pub[:],
		}
		return nil
	}); err != nil {
		return err
	}

	return nil
}

// EnsureConnection creates or updates the connection resource.
func EnsureConnection(ctx context.Context, cl client.Client, scheme *runtime.Scheme, opts *Options) error {
	conn := &networkingv1beta1.Connection{ObjectMeta: metav1.ObjectMeta{
		Name: forge.GatewayResourceName(opts.GwOptions.Name), Namespace: opts.GwOptions.Namespace,
		Labels: map[string]string{
			string(consts.RemoteClusterID): opts.GwOptions.RemoteClusterID,
		},
	}}

	klog.Infof("Creating connection %q", conn.Name)

	_, err := resource.CreateOrUpdate(ctx, cl, conn, func() error {
		if err := gateway.SetOwnerReferenceWithMode(opts.GwOptions, conn, scheme); err != nil {
			return err
		}
		conn.Spec.GatewayRef.APIVersion = networkingv1beta1.GroupVersion.String()
		conn.Spec.GatewayRef.Name = opts.GwOptions.Name
		conn.Spec.GatewayRef.Namespace = opts.GwOptions.Namespace
		conn.Spec.GatewayRef.UID = types.UID(opts.GwOptions.GatewayUID)
		switch opts.GwOptions.Mode {
		case gateway.ModeServer:
			conn.Spec.Type = networkingv1beta1.ConnectionTypeServer
			conn.Spec.GatewayRef.Kind = networkingv1beta1.WgGatewayServerKind
		case gateway.ModeClient:
			conn.Spec.Type = networkingv1beta1.ConnectionTypeClient
			conn.Spec.GatewayRef.Kind = networkingv1beta1.WgGatewayClientKind
		}
		return nil
	})

	if err != nil {
		return err
	}

	klog.Infof("Connection %q created", conn.Name)

	conn.Status.Value = networkingv1beta1.Connecting
	return cl.Status().Update(ctx, conn)
}
