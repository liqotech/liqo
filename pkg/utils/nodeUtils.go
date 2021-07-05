package utils

import corev1 "k8s.io/api/core/v1"

// IsNodeReady returns true if the passed node has the NodeReady condition = True, false otherwise.
func IsNodeReady(node *corev1.Node) bool {
	for _, condition := range node.Status.Conditions {
		if condition.Type == corev1.NodeReady && condition.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}
