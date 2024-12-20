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

package testutil

import (
	"fmt"
	"strings"

	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	authv1beta1 "github.com/liqotech/liqo/apis/authentication/v1beta1"
	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	ipamv1alpha1 "github.com/liqotech/liqo/apis/ipam/v1alpha1"
	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	liqoconsts "github.com/liqotech/liqo/pkg/consts"
	cidrutils "github.com/liqotech/liqo/pkg/utils/cidr"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
)

// FakeControllerManagerDeployment returns a fake liqo-controller-manager deployment.
func FakeControllerManagerDeployment(argsClusterLabels []string, networkEnabled bool) *appv1.Deployment {
	containerArgs := []string{}
	if len(argsClusterLabels) != 0 {
		containerArgs = append(containerArgs, "--cluster-labels="+strings.Join(argsClusterLabels, ","))
	}
	if !networkEnabled {
		containerArgs = append(containerArgs, "--networking-enabled=false")
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

// FakeIdentity returns a fake Identity.
func FakeIdentity(clusterID liqov1beta1.ClusterID, identityType authv1beta1.IdentityType) *authv1beta1.Identity {
	return &authv1beta1.Identity{
		ObjectMeta: metav1.ObjectMeta{
			Name: string(clusterID),
			Labels: map[string]string{
				liqoconsts.RemoteClusterID: string(clusterID),
			},
		},
		Spec: authv1beta1.IdentitySpec{
			ClusterID: clusterID,
			Type:      identityType,
			AuthParams: authv1beta1.AuthParams{
				APIServer: "https://192.168.0.6:6443",
			},
		},
	}
}

// FakeResourceSlice returns a fake ResourceSlice.
func FakeResourceSlice(name string, consumerClusterID, providerClusterID liqov1beta1.ClusterID,
	status authv1beta1.ResourceSliceConditionStatus, resoucesList corev1.ResourceList) *authv1beta1.ResourceSlice {
	return &authv1beta1.ResourceSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				liqoconsts.RemoteClusterID: string(providerClusterID),
			},
		},
		Spec: authv1beta1.ResourceSliceSpec{
			ConsumerClusterID: &consumerClusterID,
			ProviderClusterID: &providerClusterID,
			Resources:         resoucesList,
		},
		Status: authv1beta1.ResourceSliceStatus{
			Resources: resoucesList,
			Conditions: []authv1beta1.ResourceSliceCondition{
				{
					Type:    authv1beta1.ResourceSliceConditionTypeAuthentication,
					Status:  status,
					Message: fmt.Sprintf("Condition with status %s", status),
				},
			},
		},
	}
}

// FakeVirtualNode returns a fake VirtualNode.
func FakeVirtualNode(name string, clusterID liqov1beta1.ClusterID,
	status offloadingv1beta1.VirtualNodeConditionStatusType, resoucesList corev1.ResourceList) *offloadingv1beta1.VirtualNode {
	return &offloadingv1beta1.VirtualNode{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				liqoconsts.RemoteClusterID:           string(clusterID),
				liqoconsts.ResourceSliceNameLabelKey: name,
			},
		},
		Spec: offloadingv1beta1.VirtualNodeSpec{
			ClusterID: clusterID,
			ResourceQuota: corev1.ResourceQuotaSpec{
				Hard: resoucesList,
			},
			KubeconfigSecretRef: &corev1.LocalObjectReference{
				Name: fmt.Sprintf("%s-secret", name),
			},
		},
		Status: offloadingv1beta1.VirtualNodeStatus{
			Conditions: []offloadingv1beta1.VirtualNodeCondition{
				{
					Status: status,
				},
			},
		},
	}
}

// FakeForeignCluster returns a fake ForeignCluster.
func FakeForeignCluster(
	clusterID liqov1beta1.ClusterID, modules *liqov1beta1.Modules) *liqov1beta1.ForeignCluster {
	return &liqov1beta1.ForeignCluster{
		TypeMeta: metav1.TypeMeta{
			Kind:       liqov1beta1.ForeignClusterKind,
			APIVersion: liqov1beta1.ForeignClusterGroupVersionResource.GroupVersion().String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: string(clusterID),
			Labels: map[string]string{
				liqoconsts.RemoteClusterID: string(clusterID),
			},
		},
		Spec: liqov1beta1.ForeignClusterSpec{
			ClusterID: clusterID,
		},
		Status: liqov1beta1.ForeignClusterStatus{
			Modules:      *modules,
			Role:         liqov1beta1.UnknownRole,
			APIServerURL: ForeignAPIServerURL,
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

// FakeNetwork returns a fake Network.
func FakeNetwork(name, namespace, cidr string, labels map[string]string) *ipamv1alpha1.Network {
	return &ipamv1alpha1.Network{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: ipamv1alpha1.NetworkSpec{
			CIDR: networkingv1beta1.CIDR(cidr),
		},
		Status: ipamv1alpha1.NetworkStatus{
			CIDR: networkingv1beta1.CIDR(cidr),
		},
	}
}

// FakeNetworkPodCIDR returns a fake Network of type PodCIDR.
func FakeNetworkPodCIDR() *ipamv1alpha1.Network {
	return FakeNetwork("pod-cidr", liqoconsts.DefaultLiqoNamespace, PodCIDR, map[string]string{
		liqoconsts.NetworkNotRemappedLabelKey: liqoconsts.NetworkNotRemappedLabelValue,
		liqoconsts.NetworkTypeLabelKey:        string(liqoconsts.NetworkTypePodCIDR),
	})
}

// FakeNetworkServiceCIDR returns a fake Network of type ServiceCIDR.
func FakeNetworkServiceCIDR() *ipamv1alpha1.Network {
	return FakeNetwork("service-cidr", liqoconsts.DefaultLiqoNamespace, ServiceCIDR, map[string]string{
		liqoconsts.NetworkNotRemappedLabelKey: liqoconsts.NetworkNotRemappedLabelValue,
		liqoconsts.NetworkTypeLabelKey:        string(liqoconsts.NetworkTypeServiceCIDR),
	})
}

// FakeNetworkExternalCIDR returns a fake Network of type ExternalCIDR.
func FakeNetworkExternalCIDR() *ipamv1alpha1.Network {
	return FakeNetwork("external-cidr", liqoconsts.DefaultLiqoNamespace, ExternalCIDR, map[string]string{
		liqoconsts.NetworkTypeLabelKey: string(liqoconsts.NetworkTypeExternalCIDR),
	})
}

// FakeNetworkInternalCIDR returns a fake Network of type InternalCIDR.
func FakeNetworkInternalCIDR() *ipamv1alpha1.Network {
	return FakeNetwork("internal-cidr", liqoconsts.DefaultLiqoNamespace, InternalCIDR, map[string]string{
		liqoconsts.NetworkTypeLabelKey: string(liqoconsts.NetworkTypeInternalCIDR),
	})
}

// FakeNetworkReservedSubnet returns a fake Network of type Reserved Subnet.
func FakeNetworkReservedSubnet(i int) *ipamv1alpha1.Network {
	return FakeNetwork(ReservedSubnets[i], liqoconsts.DefaultLiqoNamespace, ReservedSubnets[i], map[string]string{
		liqoconsts.NetworkTypeLabelKey:        string(liqoconsts.NetworkTypeReserved),
		liqoconsts.NetworkNotRemappedLabelKey: liqoconsts.NetworkNotRemappedLabelValue,
	})
}

// FakeIP returns a fake IP.
func FakeIP(name, namespace, ip, cidr string, labels map[string]string, networkRef *corev1.ObjectReference, masquerade bool) *ipamv1alpha1.IP {
	return &ipamv1alpha1.IP{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: ipamv1alpha1.IPSpec{
			IP:         networkingv1beta1.IP(ip),
			NetworkRef: networkRef,
			Masquerade: &masquerade,
		},
		Status: ipamv1alpha1.IPStatus{
			IP:   networkingv1beta1.IP(ip),
			CIDR: networkingv1beta1.CIDR(cidr),
		},
	}
}

// FakeConfiguration returns a fake Configuration.
func FakeConfiguration(remoteClusterID, podCIDR, extCIDR, remotePodCIDR, remoteExtCIDR,
	remoteRemappedPodCIDR, remoteRemappedExtCIDR string) *networkingv1beta1.Configuration {
	return &networkingv1beta1.Configuration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fake-configuration",
			Namespace: "fake-tentant-namespace",
			Labels: map[string]string{
				liqoconsts.RemoteClusterID: remoteClusterID,
			},
		},
		Spec: networkingv1beta1.ConfigurationSpec{
			Local: &networkingv1beta1.ClusterConfig{
				CIDR: networkingv1beta1.ClusterConfigCIDR{
					Pod:      cidrutils.SetPrimary(networkingv1beta1.CIDR(podCIDR)),
					External: cidrutils.SetPrimary(networkingv1beta1.CIDR(extCIDR)),
				},
			},
			Remote: networkingv1beta1.ClusterConfig{
				CIDR: networkingv1beta1.ClusterConfigCIDR{
					Pod:      cidrutils.SetPrimary(networkingv1beta1.CIDR(remotePodCIDR)),
					External: cidrutils.SetPrimary(networkingv1beta1.CIDR(remoteExtCIDR)),
				},
			},
		},
		Status: networkingv1beta1.ConfigurationStatus{
			Remote: &networkingv1beta1.ClusterConfig{
				CIDR: networkingv1beta1.ClusterConfigCIDR{
					Pod:      cidrutils.SetPrimary(networkingv1beta1.CIDR(remoteRemappedPodCIDR)),
					External: cidrutils.SetPrimary(networkingv1beta1.CIDR(remoteRemappedExtCIDR)),
				},
			},
		},
	}
}

// FakeConnection returns a fake Connection.
func FakeConnection(remoteClusterID string) *networkingv1beta1.Connection {
	return &networkingv1beta1.Connection{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fake-connection",
			Namespace: "fake-tentant-namespace",
			Labels: map[string]string{
				liqoconsts.RemoteClusterID: remoteClusterID,
			},
		},
		Spec: networkingv1beta1.ConnectionSpec{
			Type: networkingv1beta1.ConnectionTypeServer,
		},
		Status: networkingv1beta1.ConnectionStatus{
			Latency: networkingv1beta1.ConnectionLatency{
				Value:     "fake-latency",
				Timestamp: metav1.Now(),
			},
			Value: networkingv1beta1.Connected,
		},
	}
}

// FakeGatewayServer returns a fake GatewayServer.
func FakeGatewayServer(remoteClusterID string) *networkingv1beta1.GatewayServer {
	return &networkingv1beta1.GatewayServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fake-gateway-server",
			Namespace: "fake-tentant-namespace",
			Labels: map[string]string{
				liqoconsts.RemoteClusterID: remoteClusterID,
			},
		},
		Spec: networkingv1beta1.GatewayServerSpec{
			MTU: 1500,
			Endpoint: networkingv1beta1.Endpoint{
				ServiceType: "fake-service-type",
				Port:        1234,
			},
		},
		Status: networkingv1beta1.GatewayServerStatus{
			Endpoint: &networkingv1beta1.EndpointStatus{
				Addresses: []string{"fake-address"},
				Port:      1234,
				Protocol:  ptr.To(corev1.ProtocolTCP),
			},
		},
	}
}

// FakeGatewayClient returns a fake GatewayClient.
func FakeGatewayClient(remoteClusterID string) *networkingv1beta1.GatewayClient {
	return &networkingv1beta1.GatewayClient{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fake-gateway-client",
			Namespace: "fake-tentant-namespace",
			Labels: map[string]string{
				liqoconsts.RemoteClusterID: remoteClusterID,
			},
		},
		Spec: networkingv1beta1.GatewayClientSpec{
			MTU: 1500,
			Endpoint: networkingv1beta1.EndpointStatus{
				Addresses: []string{"fake-address"},
				Port:      1234,
				Protocol:  ptr.To(corev1.ProtocolTCP),
			},
		},
	}
}
