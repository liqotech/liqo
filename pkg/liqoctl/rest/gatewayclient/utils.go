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

package gatewayclient

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	networkingv1alpha1 "github.com/liqotech/liqo/apis/networking/v1alpha1"
	liqoconsts "github.com/liqotech/liqo/pkg/consts"
	enutils "github.com/liqotech/liqo/pkg/liqo-controller-manager/external-network/utils"
)

// ForgeGatewayClient forges a GatewayClient.
func ForgeGatewayClient(name, namespace string, o *ForgeOptions) (*networkingv1alpha1.GatewayClient, error) {
	gwClient := &networkingv1alpha1.GatewayClient{
		TypeMeta: metav1.TypeMeta{
			Kind:       networkingv1alpha1.GatewayClientKind,
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
	err := MutateGatewayClient(gwClient, o)
	if err != nil {
		return nil, err
	}
	return gwClient, nil
}

// MutateGatewayClient mutates a GatewayClient.
func MutateGatewayClient(gwClient *networkingv1alpha1.GatewayClient, o *ForgeOptions) error {
	gwClient.Kind = networkingv1alpha1.GatewayClientKind
	gwClient.APIVersion = networkingv1alpha1.GroupVersion.String()

	if gwClient.Labels == nil {
		gwClient.Labels = make(map[string]string)
	}
	gwClient.Labels[liqoconsts.RemoteClusterID] = o.RemoteClusterID

	gwClient.Spec.MTU = o.MTU

	protocol := v1.Protocol(o.Protocol)
	gwClient.Spec.Endpoint = networkingv1alpha1.EndpointStatus{
		Addresses: o.Addresses,
		Port:      o.Port,
		Protocol:  &protocol,
	}

	gvr, err := enutils.ParseGroupVersionResource(o.GatewayType)
	if err != nil {
		return err
	}
	kind, err := enutils.ResourceToKind(gvr, o.KubeClient)
	if err != nil {
		return err
	}
	gwClient.Spec.ClientTemplateRef = v1.ObjectReference{
		Name:       o.TemplateName,
		Namespace:  o.TemplateNamespace,
		Kind:       kind,
		APIVersion: gvr.GroupVersion().String(),
	}

	return nil
}
