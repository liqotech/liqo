package kubeconfig

import (
	"context"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/klog"
)

type LoadConfigError struct {
	error string
}

func (lce LoadConfigError) Error() string {
	return lce.error
}

func LoadFromSecret(secret *v1.Secret) (*rest.Config, error) {
	kubeconfig := func() (*clientcmdapi.Config, error) {
		return clientcmd.Load(secret.Data["kubeconfig"])
	}
	cnf, err := clientcmd.BuildConfigFromKubeconfigGetter("", kubeconfig)
	if err != nil {
		return nil, LoadConfigError{
			error: err.Error(),
		}
	}
	return cnf, nil
}

func CreateSecret(client kubernetes.Interface, namespace string, kubeconfig string, labels map[string]string) (*v1.Secret, error) {
	identitySecret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "remote-identity-",
			Labels:       labels,
		},
		StringData: map[string]string{
			"kubeconfig": kubeconfig,
		},
	}
	identitySecret, err := client.CoreV1().Secrets(namespace).Create(context.TODO(), identitySecret, metav1.CreateOptions{})
	if err != nil {
		klog.Error(err)
		return nil, err
	}
	return identitySecret, nil
}
