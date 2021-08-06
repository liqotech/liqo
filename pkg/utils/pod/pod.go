package pod

import corev1 "k8s.io/api/core/v1"

// IsPodReady returns true if a pod is ready; false otherwise.
func IsPodReady(pod *corev1.Pod) bool {
	conditions := pod.Status.Conditions
	for i := range conditions {
		if conditions[i].Type == corev1.PodReady {
			return conditions[i].Status == corev1.ConditionTrue
		}
	}
	return false
}
