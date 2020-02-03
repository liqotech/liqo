package kubernetes

import (
	"fmt"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"time"

)


func H2FTranslate(podRemoteIn *v1.Pod) (podOriginOut *v1.Pod) {
	podOriginOut = podRemoteIn.DeepCopy()
	podOriginOut.SetUID(types.UID(podRemoteIn.Annotations["origin_uuid"]))
	podOriginOut.SetResourceVersion(podRemoteIn.Annotations["origin_resourceVersion"])
	t, err := time.Parse(time.RFC3339, podRemoteIn.Annotations["origin_creationTimestamp"],)
	if podRemoteIn.DeletionGracePeriodSeconds != nil {
		metav1.SetMetaDataAnnotation(&podOriginOut.ObjectMeta,"remote_deletionPeriodSeconds", string(*podRemoteIn.DeletionGracePeriodSeconds))
		podOriginOut.DeletionGracePeriodSeconds = nil
	}

	if err != nil {
		fmt.Errorf("Unable to parse time")
	}
	podOriginOut.SetCreationTimestamp(metav1.NewTime(t))
	podOriginOut.Spec.NodeName =   podRemoteIn.Annotations["origin_nodename"]
	delete(podOriginOut.Annotations, "origin_creationTimestamp")
	delete(podOriginOut.Annotations, "origin_resourceVersion")
	delete(podOriginOut.Annotations, "origin_uuid")
	delete(podOriginOut.Annotations, "origin_nodename")
	return podOriginOut
}

func F2HTranslate(pod *v1.Pod) *v1.Pod {
	// create an empty ObjectMeta for the output pod, copying only "Name" and "Namespace" fields
	objectMeta := metav1.ObjectMeta{
		Name:                       pod.ObjectMeta.Name,
		Namespace:                  pod.ObjectMeta.Namespace,
		Labels: pod.Labels,
	}

	// copy all containers from input pod
	containers := make([]v1.Container, len(pod.Spec.Containers))
	for i:=0 ; i < len(pod.Spec.Containers) ; i++ {
		containers[i].Name = pod.Spec.Containers[i].Name
		containers[i].Image = pod.Spec.Containers[i].Image
	}

	// create an empty Spec for the output pod, copying only "Containers" field
	podSpec := v1.PodSpec{
		Containers:                    containers,
	}

	metav1.SetMetaDataAnnotation(&objectMeta,"origin_nodename", pod.Spec.NodeName)
	metav1.SetMetaDataAnnotation(&objectMeta,"origin_resourceVersion", pod.ResourceVersion)
	metav1.SetMetaDataAnnotation(&objectMeta,"origin_uuid", string(pod.UID))
	metav1.SetMetaDataAnnotation(&objectMeta,"origin_creationTimestamp", pod.CreationTimestamp.String())

	return &v1.Pod{
		TypeMeta:   pod.TypeMeta,
		ObjectMeta: objectMeta,
		Spec:       podSpec,
		Status:     pod.Status,
	}
}