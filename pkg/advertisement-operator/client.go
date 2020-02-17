package advertisement_operator

import (
	"context"
	"fmt"
	"os"

	protocolv1beta1 "github.com/netgroup-polito/dronev2/api/v1beta1"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// get config to create a client
// parameters:
// - path: the path to the kubeconfig file
// - configMapName: the name of the configMap containing the kubeconfig to the foreign cluster
// if path is specified create a config from a kubeconfig file, otherwise create or a inCluster config or read the kubeconfig from a configMap
func GetConfig(path string, configMapName string, crdClient client.Client) (*rest.Config, error) {
	var config *rest.Config
	var err error

	if path == "" && configMapName == "" {
		config, err = rest.InClusterConfig()
		if err != nil {
			return nil, err
		}
	} else if path == "" && configMapName != "" {
		// Get the kubeconfig from configMap
		if crdClient == nil {
			c, err := NewK8sClient(path, configMapName)
			if err != nil {
				return nil, err
			}
			kubeconfigGetter := GetKubeconfigFromConfigMap(configMapName, c)
			config, err = clientcmd.BuildConfigFromKubeconfigGetter("", kubeconfigGetter)
			if err != nil {
				return nil, err
			}
		} else {
			kubeconfigGetter := GetKubeconfigFromConfigMapWithCRDClient(configMapName, crdClient)
			config, err = clientcmd.BuildConfigFromKubeconfigGetter("", kubeconfigGetter)
			if err != nil {
				return nil, err
			}
		}
	} else if path != "" {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			// Get the kubeconfig from the filepath.
			config, err = clientcmd.BuildConfigFromFlags("", path)
			if err != nil {
				return nil, err
			}
		}
	}

	return config, err
}

// create a standard K8s client -> to access use client.CoreV1().<resource>(<namespace>).<method>())
func NewK8sClient(path string, configMapName string) (*kubernetes.Clientset, error) {
	config, err := GetConfig(path, configMapName, nil)
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(config)
}

// create a crd client (kubebuilder-like) -> to access use client.<method>(context, <NamespacedName>, <resource>)
func NewCRDClient(path string, configMapName string, c client.Client) (client.Client, error) {
	config, err := GetConfig(path, configMapName, c)
	if err != nil {
		return nil, err
	}

	scheme := k8sruntime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = protocolv1beta1.AddToScheme(scheme)

	remoteClient, err := client.New(config, client.Options{
		Scheme: scheme,
	})
	if err != nil {
		return nil, err
	}

	return remoteClient, nil
}

// extract kubeconfig from a configMap.
// parameters:
// - configMapName: the name of the configMap
// - client: the k8s client to the local cluster
func GetKubeconfigFromConfigMap(configMapName string, client *kubernetes.Clientset) clientcmd.KubeconfigGetter {
	return func() (*clientcmdapi.Config, error) {
		// Get the namespace this is running in from the env variable.
		namespace := os.Getenv("POD_NAMESPACE")
		if namespace == "" {
			namespace = "default"
		}

		data := []byte{}
		if configMapName != "" {
			cm, err := client.CoreV1().ConfigMaps(namespace).Get(configMapName, metav1.GetOptions{})
			if err != nil {
				return nil, fmt.Errorf("error in fetching configMap: %s", err)
			}
			data = []byte(cm.Data["remote"])
		}
		return clientcmd.Load(data)
	}
}

// extract kubeconfig from a configMap
// parameters:
// - configMapName: the name of the configMap
// - crdClient: the kubebuilder-like client to the local cluster
func GetKubeconfigFromConfigMapWithCRDClient(configMapName string, crdClient client.Client) clientcmd.KubeconfigGetter {
	return func() (*clientcmdapi.Config, error) {
		// Get the namespace this is running in from the env variable.
		namespace := os.Getenv("POD_NAMESPACE")
		if namespace == "" {
			namespace = "default"
		}

		data := []byte{}
		var cm v1.ConfigMap
		if configMapName != "" {
			err := crdClient.Get(context.Background(), types.NamespacedName{
				Namespace: namespace,
				Name:      configMapName,
			}, &cm)
			if err != nil {
				return nil, fmt.Errorf("error in fetching configMap: %s", err)
			}
			data = []byte(cm.Data["remote"])
		}
		return clientcmd.Load(data)
	}
}

