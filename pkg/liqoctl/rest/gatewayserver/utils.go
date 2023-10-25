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

package gatewayserver

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	networkingv1alpha1 "github.com/liqotech/liqo/apis/networking/v1alpha1"
	liqoconsts "github.com/liqotech/liqo/pkg/consts"
	enutils "github.com/liqotech/liqo/pkg/liqo-controller-manager/external-network/utils"
)

// ForgeGatewayServer forges a GatewayServer.
func ForgeGatewayServer(name, namespace string, o *ForgeOptions) (*networkingv1alpha1.GatewayServer, error) {
	gwServer := &networkingv1alpha1.GatewayServer{
		TypeMeta: metav1.TypeMeta{
			Kind:       networkingv1alpha1.GatewayServerKind,
			APIVersion: networkingv1alpha1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				liqoconsts.RemoteClusterID: o.RemoteClusterID,
			},
		},
	}
	err := MutateGatewayServer(gwServer, o)
	if err != nil {
		return nil, err
	}
	return gwServer, nil
}

// MutateGatewayServer mutates a GatewayServer.
func MutateGatewayServer(gwServer *networkingv1alpha1.GatewayServer, o *ForgeOptions) error {
	gwServer.Kind = networkingv1alpha1.GatewayServerKind
	gwServer.APIVersion = networkingv1alpha1.GroupVersion.String()

	if gwServer.Labels == nil {
		gwServer.Labels = make(map[string]string)
	}
	gwServer.Labels[liqoconsts.RemoteClusterID] = o.RemoteClusterID

	gwServer.Spec.MTU = o.MTU
	gwServer.Spec.Endpoint = networkingv1alpha1.Endpoint{
		Port:        o.Port,
		ServiceType: o.ServiceType,
	}

	gvr, err := enutils.ParseGroupVersionResource(o.GatewayType)
	if err != nil {
		return err
	}
	kind, err := enutils.ResourceToKind(gvr, o.KubeClient)
	if err != nil {
		return err
	}
	gwServer.Spec.ServerTemplateRef = corev1.ObjectReference{
		Name:       o.TemplateName,
		Namespace:  o.TemplateNamespace,
		Kind:       kind,
		APIVersion: gvr.GroupVersion().String(),
	}

	return nil
}

// DefaultGatewayServerName returns the default name for a GatewayServer.
func DefaultGatewayServerName(remoteClusterIdentity *discoveryv1alpha1.ClusterIdentity) string {
	return remoteClusterIdentity.ClusterName
}
