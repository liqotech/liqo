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
func EnsureNodePort(client kubernetes.Interface, clusterID, name, namespace string) (*v1.Service, error) {
	nodePort := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: v1.ServiceSpec{
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
		},
		Status: v1.ServiceStatus{},
	}
	nodePort, err := client.CoreV1().Services(namespace).Create(context.TODO(), nodePort, metav1.CreateOptions{})
	if kerrors.IsAlreadyExists(err) {
		_, err = client.CoreV1().Services(namespace).Update(context.TODO(), nodePort, metav1.UpdateOptions{})
		if err != nil {
			klog.Errorf("%s -> an error occurred while updating nodePort service %s : %s", clusterID, name, err)
			return nil, err
		}
	}
	if err != nil {
		klog.Errorf("%s -> an error occurred while creating nodePort service %s in namespace %s: %s", clusterID, name, namespace, err)
		return nil, err
	}
	return nodePort, nil
}
