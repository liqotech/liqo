package net

import (
	"context"

	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

// EnsureNodePort creates a Service of type NodePort for the netTest.
func EnsureNodePort(ctx context.Context, client kubernetes.Interface, clusterID, name, namespace string) (*v1.Service, error) {
	serviceSpec := v1.ServiceSpec{
		Ports: []v1.ServicePort{{
			Name:        "http",
			Protocol:    "TCP",
			AppProtocol: nil,
			Port:        80,
			TargetPort: intstr.IntOrString{
				IntVal: 80,
			},
		}},
		Selector: map[string]string{"app": name},
		Type:     v1.ServiceTypeNodePort,
	}
	nodePort := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec:   serviceSpec,
		Status: v1.ServiceStatus{},
	}
	_, err := client.CoreV1().Services(namespace).Create(ctx, nodePort, metav1.CreateOptions{})
	if kerrors.IsAlreadyExists(err) {
		nodePort, err = client.CoreV1().Services(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			klog.Error(err)
			return nil, err
		}
	}
	nodePort, err = client.CoreV1().Services(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		klog.Error(err)
		return nil, err
	}
	nodePort, err = client.CoreV1().Services(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		klog.Error(err)
		return nil, err
	}
	return nodePort, nil
}
