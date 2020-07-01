package test

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

var ServiceTestCases = struct {
	InputServices  map[string]*corev1.Service
	UpdateServices map[string]*corev1.Service
	DeleteServices map[string]*corev1.Service
}{
	InputServices: map[string]*corev1.Service{
		"service1": {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "service1",
				Namespace: Namespace,
			},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{
					{
						Name:       "sp1",
						Protocol:   corev1.ProtocolTCP,
						Port:       1000,
						TargetPort: intstr.IntOrString{},
						NodePort:   5000,
					},
				},
			},
		},
		"service2": {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "service2",
				Namespace: Namespace,
			},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{
					{
						Name:       "sp2",
						Protocol:   corev1.ProtocolUDP,
						Port:       1001,
						TargetPort: intstr.IntOrString{},
						NodePort:   5001,
					},
				},
			},
		},
	},
	UpdateServices: map[string]*corev1.Service{
		"service1": {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "service1",
				Namespace: Namespace,
			},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{
					{
						Name:       "sp1",
						Protocol:   corev1.ProtocolTCP,
						Port:       10000,
						TargetPort: intstr.IntOrString{},
						NodePort:   50000,
					},
				},
			},
		},
		"service2": {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "service2",
				Namespace: Namespace,
			},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{
					{
						Name:       "sp2",
						Protocol:   corev1.ProtocolUDP,
						Port:       10001,
						TargetPort: intstr.IntOrString{},
						NodePort:   50001,
					},
				},
			},
		},
	},
	DeleteServices: map[string]*corev1.Service{
		"service1": {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "service1",
				Namespace: Namespace,
			},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{
					{
						Name:       "sp1",
						Protocol:   corev1.ProtocolTCP,
						Port:       10000,
						TargetPort: intstr.IntOrString{},
						NodePort:   50000,
					},
				},
			},
		},
	},
}
