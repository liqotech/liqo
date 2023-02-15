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
	"strings"

	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	liqoconsts "github.com/liqotech/liqo/pkg/consts"
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
func FakeControllerManagerDeployment(argsClusterLabels []string) *appv1.Deployment {
	containerArgs := []string{}
	if len(argsClusterLabels) != 0 {
		containerArgs = append(containerArgs, "--cluster-labels="+strings.Join(argsClusterLabels, ","))
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
