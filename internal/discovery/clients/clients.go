package clients

import (
	v1 "github.com/liqoTech/liqo/pkg/discovery/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"log"
	"os"
	"path/filepath"
)

func NewConfig() (*rest.Config, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return clientcmd.BuildConfigFromFlags("", filepath.Join(os.Getenv("HOME"), ".kube", "config"))
	}
	return config, err
}

func NewK8sClient() (*kubernetes.Clientset, error) {
	config, err := NewConfig()
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(config)
}

// TODO: delete
func NewDiscoveryClient() (*v1.DiscoveryV1Client, error) {
	config, err := NewConfig()
	if err != nil {
		log.Println(err.Error())
		os.Exit(1)
	}
	return v1.NewForConfig(config)
}

func NewCRDClient() (*v1alpha1.CRDClient, error) {
	config, err := v1alpha1.NewKubeconfig(filepath.Join(os.Getenv("HOME"), ".kube", "config"), discoveryv1.GroupVersion)
	if err != nil {
		return nil, err
	}
	return v1alpha1.NewFromConfig(config)
}
