package test

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"
)

var (
	nodeName1 = "testNode"
	nodeName2 = "testNode2"
)

var EndpointsTestCases = struct {
	InputEndpoints         corev1.Endpoints
	InputSubsets           [][]corev1.EndpointSubset
	ExpectedEndpoints      corev1.Endpoints
	ExpectedNumberOfEvents int
}{
	ExpectedNumberOfEvents: 2,
	InputEndpoints: corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name: EndpointsName,
		},
		Subsets: nil,
	},
	InputSubsets: [][]corev1.EndpointSubset{
		{
			{
				Addresses: []corev1.EndpointAddress{
					{
						IP:        "10.0.0.1",
						Hostname:  strings.Join([]string{HostName, "1"}, ""),
						NodeName:  &nodeName2,
						TargetRef: nil,
					},
				},
				NotReadyAddresses: nil,
				Ports: []corev1.EndpointPort{
					{
						Name:     "TestPort",
						Port:     8000,
						Protocol: corev1.ProtocolTCP,
					},
				},
			},
		},
		{
			{
				Addresses: []corev1.EndpointAddress{
					{
						IP:        "10.0.0.1",
						Hostname:  strings.Join([]string{HostName, "1"}, ""),
						NodeName:  &nodeName1,
						TargetRef: nil,
					},
					{
						IP:        "10.0.0.2",
						Hostname:  "testHost",
						NodeName:  &nodeName2,
						TargetRef: nil,
					},
				},
				NotReadyAddresses: nil,
				Ports: []corev1.EndpointPort{
					{
						Name:     "TestPort",
						Port:     8000,
						Protocol: corev1.ProtocolTCP,
					},
				},
			},
		},
	},
	ExpectedEndpoints: corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "serviceTest",
			Namespace: Namespace,
		},
		Subsets: []corev1.EndpointSubset{
			{
				Addresses: []corev1.EndpointAddress{
					{
						IP:        "10.0.0.1",
						Hostname:  strings.Join([]string{HostName, "1"}, ""),
						NodeName:  nil,
						TargetRef: nil,
					},
					{
						IP:        "10.0.0.2",
						Hostname:  "testHost",
						NodeName:  nil,
						TargetRef: nil,
					},
				},
				NotReadyAddresses: nil,
				Ports: []corev1.EndpointPort{
					{
						Name:     "TestPort",
						Port:     8000,
						Protocol: corev1.ProtocolTCP,
					},
				},
			},
		},
	},
}
