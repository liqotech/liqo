package v1alpha1

import (
	"context"
	"github.com/liqotech/liqo/pkg/kubeconfig"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func (pr *PeeringRequest) GetConfig(clientset kubernetes.Interface) (*rest.Config, error) {
	return getConfig(clientset, pr.Spec.KubeConfigRef)
}

func getConfig(clientset kubernetes.Interface, reference *v1.ObjectReference) (*rest.Config, error) {
	secret, err := clientset.CoreV1().Secrets(reference.Namespace).Get(context.TODO(), reference.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return kubeconfig.LoadFromSecret(secret)
}
