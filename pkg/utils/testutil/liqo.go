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

package testutil

import (
	"fmt"
	"strconv"
	"strings"

	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
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
						{Args: containerArgs},
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
