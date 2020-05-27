package v1

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func (pr *PeeringRequest) GetConfig(clientset *kubernetes.Clientset) (*rest.Config, error) {
	return getConfig(clientset, &pr.Spec.KubeConfigRef)
}
