package forge

import (
	"github.com/liqotech/liqo/pkg/virtualKubelet"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var defaultReplicas int32 = 1

func (f *apiForger) replicasetFromPod(pod *corev1.Pod) *appsv1.ReplicaSet {
	if pod.Labels == nil {
		pod.Labels = make(map[string]string)
	}
	pod.Labels[virtualKubelet.ReflectedpodKey] = pod.Name

	replicaset := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:        pod.Name,
			Namespace:   pod.Namespace,
			Labels:      pod.Labels,
			Annotations: pod.Annotations,
		},
		Spec: appsv1.ReplicaSetSpec{
			Replicas: &defaultReplicas,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      pod.Labels,
					Annotations: pod.Annotations,
				},
				Spec: pod.Spec,
			},
			Selector: &metav1.LabelSelector{
				MatchLabels: pod.Labels,
			},
		},
	}

	return replicaset
}
