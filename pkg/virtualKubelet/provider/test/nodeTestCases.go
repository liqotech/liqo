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
	Condition1 = []corev1.NodeCondition{
		{
			Type:   corev1.NodeReady,
			Status: corev1.ConditionFalse,
		},
		{
			Type:   corev1.NodeMemoryPressure,
			Status: corev1.ConditionFalse,
		},
		{
			Type:   corev1.NodeDiskPressure,
			Status: corev1.ConditionFalse,
		},
		{
			Type:   corev1.NodePIDPressure,
			Status: corev1.ConditionFalse,
		},
		{
			Type:   corev1.NodeNetworkUnavailable,
			Status: corev1.ConditionTrue,
		},
	}
	Condition2 = []corev1.NodeCondition{
		{
			Type:   corev1.NodeReady,
			Status: corev1.ConditionTrue,
		},
		{
			Type:   corev1.NodeMemoryPressure,
			Status: corev1.ConditionFalse,
		},
		{
			Type:   corev1.NodeDiskPressure,
			Status: corev1.ConditionFalse,
		},
		{
			Type:   corev1.NodePIDPressure,
			Status: corev1.ConditionFalse,
		},
		{
			Type:   corev1.NodeNetworkUnavailable,
			Status: corev1.ConditionFalse,
		},
	}
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
	// ExpectedNodes[0] : cpu1, mem1, condition1 (NodeNetworkUnavailable)
	// ExpectedNodes[1] : cpu2, mem2, condition1 (NodeNetworkUnavailable)
	// ExpectedNodes[2] : cpu2, mem2, condition2 (NodeReady)
	// ExpectedNodes[0] -> ExpectedNodes[1] : Advertisement update  => different resources, same node conditions
	// ExpectedNodes[1] -> ExpectedNodes[2] : TunnelEndpoint update => same resources, different node conditions
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
				Conditions: Condition1,
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
				Conditions: Condition1,
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
				Conditions: Condition2,
			},
		},
	},
}
