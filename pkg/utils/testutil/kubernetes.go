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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"

	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
)

// FakeClusterIDConfigMap returns a fake ClusterID ConfigMap.
func FakeClusterIDConfigMap(namespace, clusterID, clusterName string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace, Name: "whatever",
			Labels: map[string]string{consts.K8sAppNameKey: consts.ClusterIDConfigMapNameLabelValue},
		},
		Data: map[string]string{
			consts.ClusterIDConfigMapKey:   clusterID,
			consts.ClusterNameConfigMapKey: clusterName,
		},
	}
}

// FakeIPAM returns an IPAM with the specified namespace and name.
func FakeIPAM(namespace string) *netv1alpha1.IpamStorage {
	return &netv1alpha1.IpamStorage{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
		},
		Spec: netv1alpha1.IpamSpec{
			PodCIDR:         PodCIDR,
			ServiceCIDR:     ServiceCIDR,
			ExternalCIDR:    ExternalCIDR,
			ReservedSubnets: ReservedSubnets,
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

// FakeService returns a service with the specified namespace and name and service info.
func FakeService(namespace, name, clusterIP, protocol string, port int32) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{{
				Protocol: corev1.Protocol(protocol),
				Port:     port,
			}},
			ClusterIP: clusterIP,
		},
	}
}
