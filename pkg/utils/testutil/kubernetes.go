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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"

	liqoconsts "github.com/liqotech/liqo/pkg/consts"
)

// FakeLiqoNamespace returns a fake Liqo namespace.
func FakeLiqoNamespace(name string) *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"kubernetes.io/metadata.name": "liqo",
			},
		},
	}
}

// FakeClusterIDConfigMap returns a fake ClusterID ConfigMap.
func FakeClusterIDConfigMap(namespace, clusterID string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace, Name: "whatever",
			Labels: map[string]string{liqoconsts.K8sAppNameKey: liqoconsts.ClusterIDConfigMapNameLabelValue},
		},
		Data: map[string]string{
			liqoconsts.ClusterIDConfigMapKey: clusterID,
		},
	}
}

// FakeEventRecorder returns an event recorder that can be used to capture events.
func FakeEventRecorder(bufferSize int) *record.FakeRecorder {
	return record.NewFakeRecorder(bufferSize)
}

// FakePodWithSingleContainer returns a pod with the specified namespace and name, and having a single container with the specified image.
func FakePodWithSingleContainer(namespace, name, image string) *corev1.Pod {
	enableServiceLink := corev1.DefaultEnableServiceLinks

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  name,
					Image: image,
				},
			},
			EnableServiceLinks: &enableServiceLink,
		},
	}
}

// FakeSecret returns a secret with the specified namespace, name and data.
func FakeSecret(namespace, name string, data map[string]string) *corev1.Secret {
	res := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Data: make(map[string][]byte),
	}
	for key, val := range data {
		res.Data[key] = []byte(val)
	}
	return res
}

// FakeServiceClusterIP returns a ClusterIP service with the specified namespace, name and service info.
func FakeServiceClusterIP(namespace, name, clusterIP string, protocol corev1.Protocol, port int32) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: clusterIP,
			Ports: []corev1.ServicePort{{
				Protocol: protocol,
				Port:     port,
			}},
			Type: corev1.ServiceTypeClusterIP,
		},
	}
}

// FakeServiceNodePort returns a NodePort service with the specified namespace, name and service info.
func FakeServiceNodePort(namespace, name string, labels map[string]string, annotations map[string]string,
	protocol corev1.Protocol, port int32, portName string, nodePort int32) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   namespace,
			Name:        name,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{{
				Name:     portName,
				Protocol: protocol,
				Port:     port,
				NodePort: nodePort,
			}},
			Type: corev1.ServiceTypeNodePort,
		},
	}
}

// FakeServiceLoadBalancer returns a LoadBalancer service with the specified namespace, name and service info.
func FakeServiceLoadBalancer(namespace, name, externalIP string, labels map[string]string, annotations map[string]string,
	protocol corev1.Protocol, port int32, portName string) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   namespace,
			Name:        name,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: corev1.ServiceSpec{
			ExternalIPs: []string{externalIP},
			Ports: []corev1.ServicePort{{
				Name:     portName,
				Protocol: protocol,
				Port:     port,
			}},
			Type: corev1.ServiceTypeLoadBalancer,
		},
		Status: corev1.ServiceStatus{
			LoadBalancer: corev1.LoadBalancerStatus{Ingress: []corev1.LoadBalancerIngress{{IP: externalIP}}},
		},
	}
}

// FakeNode returns a node.
func FakeNode() *corev1.Node {
	return FakeNodeWithNameAndLabels("fake-node", map[string]string{
		"node-role.kubernetes.io/control-plane": "",
	})
}

// FakeNodeWithNameAndLabels returns a node with the specified name and labels.
func FakeNodeWithNameAndLabels(name string, labels map[string]string) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
		Status: corev1.NodeStatus{
			Addresses: []corev1.NodeAddress{
				{Type: corev1.NodeInternalIP, Address: EndpointIP},
			},
		},
	}
}
