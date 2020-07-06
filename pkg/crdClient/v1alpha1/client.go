package v1alpha1

import (
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	clientsetFake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	restFake "k8s.io/client-go/rest/fake"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"os"
)

var Fake bool

type NamespacedCRDClientInterface interface {
	Resource(resource string) CrdClientInterface
}

type CRDClient struct {
	crdClient rest.Interface
	client    kubernetes.Interface

	config *rest.Config
}

func NewKubeconfig(configPath string, gv *schema.GroupVersion) (*rest.Config, error) {
	config := &rest.Config{}

	if !Fake {
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
	} else {
		config.ContentConfig = rest.ContentConfig{ContentType: "application/json"}
	}

	config.ContentConfig.GroupVersion = gv
	config.APIPath = "/apis"
	config.NegotiatedSerializer = scheme.Codecs.WithoutConversion()
	config.UserAgent = rest.DefaultKubernetesUserAgent()

	return config, nil
}

func NewKubeconfigFromSecret(secret *v1.Secret, gv *schema.GroupVersion) (*rest.Config, error) {
	var err error
	config := &rest.Config{}

	if !Fake {
		// Check if the kubeConfig file exists.
		config, err = clientcmd.BuildConfigFromKubeconfigGetter("", func() (*clientcmdapi.Config, error) {
			return clientcmd.Load(secret.Data["kubeconfig"])
		})
		if err != nil {
			return nil, err
		}
	} else {
		config.ContentConfig = rest.ContentConfig{ContentType: "application/json"}
	}

	config.ContentConfig.GroupVersion = gv
	config.APIPath = "/apis"
	config.NegotiatedSerializer = scheme.Codecs.WithoutConversion()
	config.UserAgent = rest.DefaultKubernetesUserAgent()

	return config, nil
}

func NewFromConfig(config *rest.Config) (*CRDClient, error) {
	if Fake {
		var gv schema.GroupVersion
		if config == nil {
			gv = schema.GroupVersion{}
		} else {
			gv = *config.GroupVersion
		}

		return &CRDClient{
			crdClient: &restFake.RESTClient{NegotiatedSerializer: scheme.Codecs.WithoutConversion(),
				GroupVersion: gv},
			client: clientsetFake.NewSimpleClientset(),
			config: config}, nil
	}

	crdClient, err := rest.RESTClientFor(config)
	if err != nil {
		return nil, err
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &CRDClient{crdClient: crdClient,
		client: client,
		config: config}, nil
}

func (c *CRDClient) Resource(api string) CrdClientInterface {
	return &Client{
		Client:   c.crdClient,
		api:      api,
		resource: Registry[api],
	}
}

func (c *CRDClient) Client() kubernetes.Interface {
	if Fake {
		return c.client.(*clientsetFake.Clientset)
	}

	return c.client.(*kubernetes.Clientset)
}

func (c *CRDClient) Config() *rest.Config {
	return c.config
}
