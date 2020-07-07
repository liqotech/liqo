package test

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	Cpu1, _    = resource.ParseQuantity("1")
	Cpu2, _    = resource.ParseQuantity("2")
	Memory1, _ = resource.ParseQuantity("1000")
	Memory2, _ = resource.ParseQuantity("2000")
)

var NodeTestCases = struct {
	InputNode     *corev1.Node
	ExpectedNodes []*corev1.Node
}{
	InputNode: &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: NodeName,
		},
	},
	ExpectedNodes: []*corev1.Node{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: NodeName,
			},
			Status: corev1.NodeStatus{
				Capacity: corev1.ResourceList{
					"cpu":    Cpu1,
					"memory": Memory1,
				},
				Allocatable: corev1.ResourceList{
					"cpu":    Cpu1,
					"memory": Memory1,
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: NodeName,
			},
			Status: corev1.NodeStatus{
				Capacity: corev1.ResourceList{
					"cpu":    Cpu2,
					"memory": Memory2,
				},
				Allocatable: corev1.ResourceList{
					"cpu":    Cpu2,
					"memory": Memory2,
				},
			},
		},
	},
}
