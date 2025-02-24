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

// Default values for the GatewayServer.
const (
	DefaultGwServerType         = "networking.liqo.io/v1beta1/wggatewayservertemplates"
	DefaultGwServerTemplateName = "wireguard-server"
	DefaultGwServerLocation     = liqov1beta1.ProviderRole
	DefaultGwServerServiceType  = corev1.ServiceTypeLoadBalancer
	DefaultGwServerPort         = 51840
	DefaultKeysDir              = "/etc/wireguard/keys"
)

// defaultGatewayServerName returns the default name for a GatewayServer.
func defaultGatewayServerName(remoteClusterID liqov1beta1.ClusterID) string {
	return string(remoteClusterID)
}

// GwServerOptions encapsulate the options to forge a GatewayServer.
type GwServerOptions struct {
	KubeClient        kubernetes.Interface
	RemoteClusterID   liqov1beta1.ClusterID
	GatewayType       string
	TemplateName      string
	TemplateNamespace string
	ServiceType       corev1.ServiceType
	MTU               int
	Port              int32
	NodePort          *int32
	LoadBalancerIP    *string
}

// GatewayServer forges a GatewayServer.
func GatewayServer(namespace string, name *string, o *GwServerOptions) (*networkingv1beta1.GatewayServer, error) {
	gwServer := &networkingv1beta1.GatewayServer{
		TypeMeta: metav1.TypeMeta{
			Kind:       networkingv1beta1.GatewayServerKind,
			APIVersion: networkingv1beta1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      ptr.Deref(name, defaultGatewayServerName(o.RemoteClusterID)),
			Namespace: namespace,
			Labels: map[string]string{
				liqoconsts.RemoteClusterID: string(o.RemoteClusterID),
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
func MutateGatewayServer(gwServer *networkingv1beta1.GatewayServer, o *GwServerOptions) error {
	// Metadata
	gwServer.Kind = networkingv1beta1.GatewayServerKind
	gwServer.APIVersion = networkingv1beta1.GroupVersion.String()

	if gwServer.Labels == nil {
		gwServer.Labels = make(map[string]string)
	}
	gwServer.Labels[liqoconsts.RemoteClusterID] = string(o.RemoteClusterID)

	// MTU
	gwServer.Spec.MTU = o.MTU

	// Server Endpoint
	gwServer.Spec.Endpoint = networkingv1beta1.Endpoint{
		Port:        o.Port,
		ServiceType: o.ServiceType,
	}
	if o.NodePort != nil && *o.NodePort != 0 {
		gwServer.Spec.Endpoint.NodePort = o.NodePort
	}
	if o.LoadBalancerIP != nil && *o.LoadBalancerIP != "" {
		gwServer.Spec.Endpoint.LoadBalancerIP = o.LoadBalancerIP
	}

	// Server Template Reference
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
