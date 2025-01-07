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

package forge

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/ptr"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	liqoconsts "github.com/liqotech/liqo/pkg/consts"
	enutils "github.com/liqotech/liqo/pkg/liqo-controller-manager/networking/external-network/utils"
)

// Default values for the GatewayClient.
const (
	DefaultGwClientType         = "networking.liqo.io/v1beta1/wggatewayclienttemplates"
	DefaultGwClientTemplateName = "wireguard-client"
)

// defaultGatewayClientName returns the default name for a GatewayClient.
func defaultGatewayClientName(remoteClusterID liqov1beta1.ClusterID) string {
	return string(remoteClusterID)
}

// GwClientOptions encapsulate the options to forge a GatewayClient.
type GwClientOptions struct {
	KubeClient        kubernetes.Interface
	RemoteClusterID   liqov1beta1.ClusterID
	GatewayType       string
	TemplateName      string
	TemplateNamespace string
	MTU               int
	Addresses         []string
	Port              int32
	Protocol          string
}

// GatewayClient forges a GatewayClient.
func GatewayClient(namespace string, name *string, o *GwClientOptions) (*networkingv1beta1.GatewayClient, error) {
	gwClient := &networkingv1beta1.GatewayClient{
		TypeMeta: metav1.TypeMeta{
			Kind:       networkingv1beta1.GatewayClientKind,
			APIVersion: networkingv1beta1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      ptr.Deref(name, defaultGatewayClientName(o.RemoteClusterID)),
			Namespace: namespace,
			Labels: map[string]string{
				liqoconsts.RemoteClusterID: string(o.RemoteClusterID),
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
func MutateGatewayClient(gwClient *networkingv1beta1.GatewayClient, o *GwClientOptions) error {
	// Metadata
	gwClient.Kind = networkingv1beta1.GatewayClientKind
	gwClient.APIVersion = networkingv1beta1.GroupVersion.String()

	if gwClient.Labels == nil {
		gwClient.Labels = make(map[string]string)
	}
	gwClient.Labels[liqoconsts.RemoteClusterID] = string(o.RemoteClusterID)

	// MTU
	gwClient.Spec.MTU = o.MTU

	// Server Endpoint
	gwClient.Spec.Endpoint = networkingv1beta1.EndpointStatus{
		Addresses: o.Addresses,
		Port:      o.Port,
		Protocol:  ptr.To(corev1.Protocol(o.Protocol)),
	}

	// Client Template Reference
	gvr, err := enutils.ParseGroupVersionResource(o.GatewayType)
	if err != nil {
		return err
	}
	kind, err := enutils.ResourceToKind(gvr, o.KubeClient)
	if err != nil {
		return err
	}
	gwClient.Spec.ClientTemplateRef = corev1.ObjectReference{
		Name:       o.TemplateName,
		Namespace:  o.TemplateNamespace,
		Kind:       kind,
		APIVersion: gvr.GroupVersion().String(),
	}

	return nil
}
