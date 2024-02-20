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

package testutil

import (
	"fmt"
	"strconv"
	"strings"

	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	ipamv1alpha1 "github.com/liqotech/liqo/apis/ipam/v1alpha1"
	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	networkingv1alpha1 "github.com/liqotech/liqo/apis/networking/v1alpha1"
	sharingv1alpha1 "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	liqoconsts "github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
)

// FakeLiqoAuthService returns a fake liqo-auth service.
func FakeLiqoAuthService(serviceType corev1.ServiceType) *corev1.Service {
	switch serviceType {
	case corev1.ServiceTypeLoadBalancer:
		return FakeServiceLoadBalancer(liqoconsts.DefaultLiqoNamespace, liqoconsts.AuthServiceName, EndpointIP, nil, nil,
			corev1.ProtocolTCP, AuthenticationPort, "https")
	case corev1.ServiceTypeNodePort:
		return FakeServiceNodePort(liqoconsts.DefaultLiqoNamespace, liqoconsts.AuthServiceName, nil, nil,
			corev1.ProtocolTCP, AuthenticationPort, "https", AuthenticationPort)
	default:
		return nil
	}
}

// FakeLiqoGatewayService returns a fake liqo-gateway service.
func FakeLiqoGatewayService(serviceType corev1.ServiceType) *corev1.Service {
	switch serviceType {
	case corev1.ServiceTypeLoadBalancer:
		return FakeServiceLoadBalancer(liqoconsts.DefaultLiqoNamespace, liqoconsts.LiqoGatewayOperatorName, EndpointIP,
			map[string]string{
				liqoconsts.GatewayServiceLabelKey: liqoconsts.GatewayServiceLabelValue,
			},
			nil, corev1.ProtocolUDP, VPNGatewayPort, liqoconsts.DriverName)
	case corev1.ServiceTypeNodePort:
		return FakeServiceNodePort(liqoconsts.DefaultLiqoNamespace, liqoconsts.LiqoGatewayOperatorName,
			map[string]string{
				liqoconsts.GatewayServiceLabelKey: liqoconsts.GatewayServiceLabelValue,
			},
			map[string]string{
				liqoconsts.GatewayServiceAnnotationKey: EndpointIP,
			},
			corev1.ProtocolUDP, VPNGatewayPort, liqoconsts.DriverName, VPNGatewayPort)
	default:
		return nil
	}
}

// FakeControllerManagerDeployment returns a fake liqo-controller-manager deployment.
func FakeControllerManagerDeployment(argsClusterLabels []string, networkEnabled bool) *appv1.Deployment {
	containerArgs := []string{}
	if len(argsClusterLabels) != 0 {
		containerArgs = append(containerArgs, "--cluster-labels="+strings.Join(argsClusterLabels, ","))
	}
	if !networkEnabled {
		containerArgs = append(containerArgs, "--disable-internal-network")
	}
	return &appv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "liqo-controller-manager",
			Namespace: liqoconsts.DefaultLiqoNamespace,
			Labels: map[string]string{
				liqoconsts.K8sAppNameKey:      "controller-manager",
				liqoconsts.K8sAppComponentKey: "controller-manager",
			},
		},
		Spec: appv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Args:  containerArgs,
							Image: fmt.Sprintf("liqo-controller-manager:%s", FakeLiqoVersion),
							Name:  "controller-manager",
						},
					},
				},
			},
		},
	}
}

// FakeLiqoAuthDeployment returns a fake liqo-auth deployment.
func FakeLiqoAuthDeployment(addressOverride string) *appv1.Deployment {
	containerArgs := []string{}
	if addressOverride != "" {
		containerArgs = append(containerArgs, "--advertise-api-server-address="+addressOverride)
	}
	return &appv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "liqo-auth",
			Namespace: liqoconsts.DefaultLiqoNamespace,
			Labels: map[string]string{
				liqoconsts.K8sAppNameKey: liqoconsts.AuthAppName,
			},
		},
		Spec: appv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Args: containerArgs},
					},
				},
			},
		},
	}
}

// FakeForeignCluster returns a fake ForeignCluster.
func FakeForeignCluster(clusterIdentity discoveryv1alpha1.ClusterIdentity, tenantNamespace string,
	peeringType discoveryv1alpha1.PeeringType,
	outgoingEnabled, incomingEnabled discoveryv1alpha1.PeeringEnabledType,
	outgoingConditionStatus, incomingConditionStatus, networkConditionStatus discoveryv1alpha1.PeeringConditionStatusType,
) *discoveryv1alpha1.ForeignCluster {
	return &discoveryv1alpha1.ForeignCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterIdentity.ClusterName,
			Namespace: tenantNamespace,
		},
		Spec: discoveryv1alpha1.ForeignClusterSpec{
			PeeringType:            peeringType,
			ClusterIdentity:        clusterIdentity,
			OutgoingPeeringEnabled: outgoingEnabled,
			IncomingPeeringEnabled: incomingEnabled,
			ForeignAuthURL:         ForeignAuthURL,
			ForeignProxyURL:        ForeignProxyURL,
		},
		Status: discoveryv1alpha1.ForeignClusterStatus{
			PeeringConditions: []discoveryv1alpha1.PeeringCondition{
				{
					Type:   discoveryv1alpha1.OutgoingPeeringCondition,
					Status: outgoingConditionStatus,
				},
				{
					Type:   discoveryv1alpha1.IncomingPeeringCondition,
					Status: incomingConditionStatus,
				},
				{
					Type:   discoveryv1alpha1.AuthenticationStatusCondition,
					Status: discoveryv1alpha1.PeeringConditionStatusEstablished,
				},
				{
					Type:   discoveryv1alpha1.NetworkStatusCondition,
					Status: networkConditionStatus,
				},
				{
					Type:   discoveryv1alpha1.APIServerStatusCondition,
					Status: discoveryv1alpha1.PeeringConditionStatusEstablished,
				},
			},
			APIServerURL: ForeignAPIServerURL,
		},
	}
}

// FakeTunnelEndpoint returns a fake TunnelEndpoint.
func FakeTunnelEndpoint(remoteClusterIdentity *discoveryv1alpha1.ClusterIdentity, remoteClusterTenant string) *netv1alpha1.TunnelEndpoint {
	return &netv1alpha1.TunnelEndpoint{
		ObjectMeta: metav1.ObjectMeta{
			Name:      remoteClusterIdentity.ClusterName,
			Namespace: remoteClusterTenant,
			Labels: map[string]string{
				liqoconsts.ClusterIDLabelName:     remoteClusterIdentity.ClusterID,
				liqoconsts.GatewayServiceLabelKey: liqoconsts.GatewayServiceLabelValue,
			},
		},
		Status: netv1alpha1.TunnelEndpointStatus{
			Connection: netv1alpha1.Connection{
				PeerConfiguration: map[string]string{
					liqoconsts.WgEndpointIP:  EndpointIP,
					liqoconsts.ListeningPort: fmt.Sprintf("%d", VPNGatewayPort),
				},
			},
		},
	}
}

// FakeResourceOffer returns a fake ResourceOffer.
func FakeResourceOffer(name, tenant string, resources corev1.ResourceList) *sharingv1alpha1.ResourceOffer {
	return &sharingv1alpha1.ResourceOffer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: tenant,
			Labels:    make(map[string]string),
		},
		Spec: sharingv1alpha1.ResourceOfferSpec{
			ResourceQuota: corev1.ResourceQuotaSpec{
				Hard: resources,
			},
		},
	}
}

// FakeAcquiredResourceOffer returns a fake ResourceOffer containing acquired resources.
func FakeAcquiredResourceOffer(remoteClusterIdentity *discoveryv1alpha1.ClusterIdentity,
	remoteClusterTenant string, resources corev1.ResourceList) *sharingv1alpha1.ResourceOffer {
	offer := FakeResourceOffer(remoteClusterIdentity.ClusterName, remoteClusterTenant, resources)
	offer.ObjectMeta.Labels[liqoconsts.ReplicationOriginLabel] = remoteClusterIdentity.ClusterID
	offer.ObjectMeta.Labels[liqoconsts.ReplicationStatusLabel] = strconv.FormatBool(true)
	return offer
}

// FakeSharedResourceOffer returns a fake ResourceOffer containing shared resources.
func FakeSharedResourceOffer(remoteClusterIdentity *discoveryv1alpha1.ClusterIdentity,
	remoteClusterTenant, localClusterName string, resources corev1.ResourceList) *sharingv1alpha1.ResourceOffer {
	offer := FakeResourceOffer(localClusterName, remoteClusterTenant, resources)
	offer.ObjectMeta.Labels[liqoconsts.ReplicationDestinationLabel] = remoteClusterIdentity.ClusterID
	offer.ObjectMeta.Labels[liqoconsts.ReplicationRequestedLabel] = strconv.FormatBool(true)
	return offer
}

// FakeNetworkConfig returns a fake NetworkConfig.
func FakeNetworkConfig(local bool, clusterName, tenantNamespace,
	podCIDR, extCIDR, podCIDRNAT, extCIDRNAT string) *netv1alpha1.NetworkConfig {
	labels := make(map[string]string)
	if local {
		labels[liqoconsts.ReplicationRequestedLabel] = strconv.FormatBool(true)
	}
	return &netv1alpha1.NetworkConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterName,
			Namespace: tenantNamespace,
			Labels:    labels,
		},
		Spec: netv1alpha1.NetworkConfigSpec{
			PodCIDR:      podCIDR,
			ExternalCIDR: extCIDR,
		},
		Status: netv1alpha1.NetworkConfigStatus{
			PodCIDRNAT:      podCIDRNAT,
			ExternalCIDRNAT: extCIDRNAT,
		},
	}
}

// FakeForgingOpts returns a fake ForgingOpts.
func FakeForgingOpts() *forge.ForgingOpts {
	return &forge.ForgingOpts{
		LabelsNotReflected:      []string{FakeNotReflectedLabelKey},
		AnnotationsNotReflected: []string{FakeNotReflectedAnnotKey},
	}
}

// FakeNetworkPodCIDR returns a fake Network of type PodCIDR.
func FakeNetworkPodCIDR() *ipamv1alpha1.Network {
	return &ipamv1alpha1.Network{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod-cidr",
			Namespace: liqoconsts.DefaultLiqoNamespace,
			Labels: map[string]string{
				liqoconsts.NetworkTypeLabelKey:        string(liqoconsts.NetworkTypePodCIDR),
				liqoconsts.NetworkNotRemappedLabelKey: liqoconsts.NetworkNotRemappedLabelValue,
			},
		},
		Spec: ipamv1alpha1.NetworkSpec{
			CIDR: networkingv1alpha1.CIDR(PodCIDR),
		},
	}
}

// FakeNetworkServiceCIDR returns a fake Network of type ServiceCIDR.
func FakeNetworkServiceCIDR() *ipamv1alpha1.Network {
	return &ipamv1alpha1.Network{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "service-cidr",
			Namespace: liqoconsts.DefaultLiqoNamespace,
			Labels: map[string]string{
				liqoconsts.NetworkTypeLabelKey:        string(liqoconsts.NetworkTypeServiceCIDR),
				liqoconsts.NetworkNotRemappedLabelKey: liqoconsts.NetworkNotRemappedLabelValue,
			},
		},
		Spec: ipamv1alpha1.NetworkSpec{
			CIDR: networkingv1alpha1.CIDR(ServiceCIDR),
		},
	}
}

// FakeNetworkExternalCIDR returns a fake Network of type ExternalCIDR.
func FakeNetworkExternalCIDR() *ipamv1alpha1.Network {
	return &ipamv1alpha1.Network{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "external-cidr",
			Namespace: liqoconsts.DefaultLiqoNamespace,
			Labels: map[string]string{
				liqoconsts.NetworkTypeLabelKey: string(liqoconsts.NetworkTypeExternalCIDR),
			},
		},
		Spec: ipamv1alpha1.NetworkSpec{
			CIDR: networkingv1alpha1.CIDR(ExternalCIDR),
		},
		Status: ipamv1alpha1.NetworkStatus{
			CIDR: networkingv1alpha1.CIDR(ExternalCIDR),
		},
	}
}

// FakeNetworkInternalCIDR returns a fake Network of type InternalCIDR.
func FakeNetworkInternalCIDR() *ipamv1alpha1.Network {
	return &ipamv1alpha1.Network{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "internal-cidr",
			Namespace: liqoconsts.DefaultLiqoNamespace,
			Labels: map[string]string{
				liqoconsts.NetworkTypeLabelKey: string(liqoconsts.NetworkTypeInternalCIDR),
			},
		},
		Spec: ipamv1alpha1.NetworkSpec{
			CIDR: networkingv1alpha1.CIDR(InternalCIDR),
		},
		Status: ipamv1alpha1.NetworkStatus{
			CIDR: networkingv1alpha1.CIDR(InternalCIDR),
		},
	}
}

// FakeNetworkReservedSubnet returns a fake Network of type Reserved Subnet.
func FakeNetworkReservedSubnet(i int) *ipamv1alpha1.Network {
	return &ipamv1alpha1.Network{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ReservedSubnets[i],
			Namespace: liqoconsts.DefaultLiqoNamespace,
			Labels: map[string]string{
				liqoconsts.NetworkTypeLabelKey:        string(liqoconsts.NetworkTypeReserved),
				liqoconsts.NetworkNotRemappedLabelKey: liqoconsts.NetworkNotRemappedLabelValue,
			},
		},
		Spec: ipamv1alpha1.NetworkSpec{
			CIDR: networkingv1alpha1.CIDR(ReservedSubnets[i]),
		},
	}
}

// FakeConfiguration returns a fake Configuration.
func FakeConfiguration(remoteClusterID, podCIDR, extCIDR, remotePodCIDR, remoteExtCIDR,
	remoteRemappedPodCIDR, remoteRemappedExtCIDR string) *networkingv1alpha1.Configuration {
	return &networkingv1alpha1.Configuration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fake-configuration",
			Namespace: "fake-tentant-namespace",
			Labels: map[string]string{
				liqoconsts.RemoteClusterID: remoteClusterID,
			},
		},
		Spec: networkingv1alpha1.ConfigurationSpec{
			Local: &networkingv1alpha1.ClusterConfig{
				CIDR: networkingv1alpha1.ClusterConfigCIDR{
					Pod:      networkingv1alpha1.CIDR(podCIDR),
					External: networkingv1alpha1.CIDR(extCIDR),
				},
			},
			Remote: networkingv1alpha1.ClusterConfig{
				CIDR: networkingv1alpha1.ClusterConfigCIDR{
					Pod:      networkingv1alpha1.CIDR(remotePodCIDR),
					External: networkingv1alpha1.CIDR(remoteExtCIDR),
				},
			},
		},
		Status: networkingv1alpha1.ConfigurationStatus{
			Remote: &networkingv1alpha1.ClusterConfig{
				CIDR: networkingv1alpha1.ClusterConfigCIDR{
					Pod:      networkingv1alpha1.CIDR(remoteRemappedPodCIDR),
					External: networkingv1alpha1.CIDR(remoteRemappedExtCIDR),
				},
			},
		},
	}
}

// FakeConnection returns a fake Connection.
func FakeConnection(remoteClusterID string) *networkingv1alpha1.Connection {
	return &networkingv1alpha1.Connection{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fake-connection",
			Namespace: "fake-tentant-namespace",
			Labels: map[string]string{
				liqoconsts.RemoteClusterID: remoteClusterID,
			},
		},
		Spec: networkingv1alpha1.ConnectionSpec{
			Type: networkingv1alpha1.ConnectionTypeServer,
		},
		Status: networkingv1alpha1.ConnectionStatus{
			Latency: networkingv1alpha1.ConnectionLatency{
				Value:     "fake-latency",
				Timestamp: metav1.Now(),
			},
			Value: networkingv1alpha1.Connected,
		},
	}
}

// FakeGatewayServer returns a fake GatewayServer.
func FakeGatewayServer(remoteClusterID string) *networkingv1alpha1.GatewayServer {
	return &networkingv1alpha1.GatewayServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fake-gateway-server",
			Namespace: "fake-tentant-namespace",
			Labels: map[string]string{
				liqoconsts.RemoteClusterID: remoteClusterID,
			},
		},
		Spec: networkingv1alpha1.GatewayServerSpec{
			MTU: 1500,
			Endpoint: networkingv1alpha1.Endpoint{
				ServiceType: "fake-service-type",
				Port:        1234,
			},
		},
		Status: networkingv1alpha1.GatewayServerStatus{
			Endpoint: &networkingv1alpha1.EndpointStatus{
				Addresses: []string{"fake-address"},
				Port:      1234,
				Protocol:  ptr.To(corev1.ProtocolTCP),
			},
		},
	}
}

// FakeGatewayClient returns a fake GatewayClient.
func FakeGatewayClient(remoteClusterID string) *networkingv1alpha1.GatewayClient {
	return &networkingv1alpha1.GatewayClient{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fake-gateway-client",
			Namespace: "fake-tentant-namespace",
			Labels: map[string]string{
				liqoconsts.RemoteClusterID: remoteClusterID,
			},
		},
		Spec: networkingv1alpha1.GatewayClientSpec{
			MTU: 1500,
			Endpoint: networkingv1alpha1.EndpointStatus{
				Addresses: []string{"fake-address"},
				Port:      1234,
				Protocol:  ptr.To(corev1.ProtocolTCP),
			},
		},
	}
}
