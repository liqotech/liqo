package v1alpha1

import (
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"os"
)

type NamespacedCRDClientInterface interface {
	NamespacedCRDClient(namespace string) CrdClientInterface
}

type CRDClient struct {
	crdClient *rest.RESTClient
	client *kubernetes.Clientset

	config *rest.Config
}

func NewKubeconfig(configPath string, gv schema.GroupVersion) (*rest.Config, error) {
	var config *rest.Config

	// Check if the kubeConfig file exists.
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		// Get the kubeconfig from the filepath.
		config, err = clientcmd.BuildConfigFromFlags("", configPath)
		if err != nil {
			return nil, errors.Wrap(err, "error building Client config")
		}
	} else {
		// Set to in-cluster config.
		config, err = rest.InClusterConfig()
		if err != nil {
			return nil, errors.Wrap(err, "error building in cluster config")
		}
	}

	config.ContentConfig.GroupVersion = &schema.GroupVersion{gv.Group, gv.Version}
	config.APIPath = "/apis"
	config.NegotiatedSerializer = scheme.Codecs.WithoutConversion()
	config.UserAgent = rest.DefaultKubernetesUserAgent()

	return config, nil
}

func NewFromConfig(config *rest.Config) (*CRDClient, error) {

	crdClient, err := rest.RESTClientFor(config)
	if err != nil {
		return nil, err
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &CRDClient{crdClient: crdClient, client:client, config:config}, nil
}

func (c *CRDClient) NamespacedCRDClient(namespace string) CrdClientInterface {
	return &Client{
		Client: c.crdClient,
		ns:     namespace,
	}
}

func (c *CRDClient) Client() *kubernetes.Clientset {
	return c.client
}

func (c *CRDClient) Config() *rest.Config {
	return c.config
}
