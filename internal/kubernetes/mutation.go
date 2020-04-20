package kubernetes

import (
	"fmt"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"strings"
	"time"
)

func F2HTranslate(podForeignIn *v1.Pod, newCidr string) (podHomeOut *v1.Pod) {
	podHomeOut = podForeignIn.DeepCopy()
	podHomeOut.SetUID(types.UID(podForeignIn.Annotations["home_uuid"]))
	podHomeOut.SetResourceVersion(podForeignIn.Annotations["home_resourceVersion"])
	t, err := time.Parse("2006-01-02 15:04:05 -0700 MST", podForeignIn.Annotations["home_creationTimestamp"])
	if podForeignIn.DeletionGracePeriodSeconds != nil {
		metav1.SetMetaDataAnnotation(&podHomeOut.ObjectMeta, "foreign_deletionPeriodSeconds", string(*podForeignIn.DeletionGracePeriodSeconds))
		podHomeOut.DeletionGracePeriodSeconds = nil
	}

	if err != nil {
		_ = fmt.Errorf("Unable to parse time")
	}
	if podHomeOut.Status.PodIP != "" {
		newIp := changePodIp(newCidr, podHomeOut.Status.PodIP)
		podHomeOut.Status.PodIP = newIp
		podHomeOut.Status.PodIPs[0].IP = newIp
	}
	podHomeOut.SetCreationTimestamp(metav1.NewTime(t))
	podHomeOut.Spec.NodeName = podForeignIn.Annotations["home_nodename"]
	delete(podHomeOut.Annotations, "home_creationTimestamp")
	delete(podHomeOut.Annotations, "home_resourceVersion")
	delete(podHomeOut.Annotations, "home_uuid")
	delete(podHomeOut.Annotations, "home_nodename")
	return podHomeOut
}

func H2FTranslate(pod *v1.Pod) *v1.Pod {
	// create an empty ObjectMeta for the output pod, copying only "Name" and "Namespace" fields
	objectMeta := metav1.ObjectMeta{
		Name:      pod.ObjectMeta.Name,
		Namespace: pod.ObjectMeta.Namespace,
		Labels:    pod.Labels,
	}

	affinity := v1.Affinity{
		NodeAffinity: &v1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &v1.NodeSelector{
				NodeSelectorTerms: []v1.NodeSelectorTerm{
					v1.NodeSelectorTerm{
						MatchExpressions: []v1.NodeSelectorRequirement{
							v1.NodeSelectorRequirement{
								Key:      "type",
								Operator: v1.NodeSelectorOpNotIn,
								Values:   []string{"virtual-node"},
							},
						},
					},
				},
			},
		},
	}
	// create an empty Spec for the output pod, copying only "Containers" field
	podSpec := v1.PodSpec{
		Containers: pod.Spec.Containers,
		Affinity:   affinity.DeepCopy(),
	}

	metav1.SetMetaDataAnnotation(&objectMeta, "home_nodename", pod.Spec.NodeName)
	metav1.SetMetaDataAnnotation(&objectMeta, "home_resourceVersion", pod.ResourceVersion)
	metav1.SetMetaDataAnnotation(&objectMeta, "home_uuid", string(pod.UID))
	metav1.SetMetaDataAnnotation(&objectMeta, "home_creationTimestamp", pod.CreationTimestamp.String())

	return &v1.Pod{
		TypeMeta:   pod.TypeMeta,
		ObjectMeta: objectMeta,
		Spec:       podSpec,
		Status:     pod.Status,
	}
}

func changePodIp(newPodCidr string, oldPodIp string) (newPodIp string) {
	//the last two slices are the suffix of the newPodIp
	oldPodIpTokenized := strings.Split(oldPodIp, ".")
	newPodCidrTokenized := strings.Split(newPodCidr, "/")
	//the first two slices are the prefix of the newPodIP
	ipFromPodCidrTokenized := strings.Split(newPodCidrTokenized[0], ".")
	//used to build the new IP
	var newPodIpBuilder strings.Builder
	for i, s := range ipFromPodCidrTokenized {
		if i < 2 {
			newPodIpBuilder.WriteString(s)
			newPodIpBuilder.WriteString(".")
		}
	}
	for i, s := range oldPodIpTokenized {
		if i > 1 && i < 4 {
			newPodIpBuilder.WriteString(s)
			newPodIpBuilder.WriteString(".")
		}
	}
	return strings.TrimSuffix(newPodIpBuilder.String(), ".")
}
