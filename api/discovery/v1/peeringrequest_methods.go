package v1

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func (pr *PeeringRequest) GetConfig(clientset *kubernetes.Clientset) (*rest.Config, error) {
	return getConfig(clientset, &pr.Spec.KubeConfigRef)
}

func getConfig(clientset *kubernetes.Clientset, reference *v1.ObjectReference) (*rest.Config, error) {
	secret, err := clientset.CoreV1().Secrets(reference.Namespace).Get(reference.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	kubeconfig := func() (*clientcmdapi.Config, error) {
		return clientcmd.Load(secret.Data["kubeconfig"])
	}
	return clientcmd.BuildConfigFromKubeconfigGetter("", kubeconfig)
}
