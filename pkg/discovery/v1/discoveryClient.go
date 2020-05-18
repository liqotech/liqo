package v1

import (
	v1 "github.com/netgroup-polito/dronev2/api/discovery/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"log"
	"os"
)

type V1Interface interface {
	ForeignClusters(namespace string) ForeignClusterInterface
	FederationRequests(namespace string) FederationRequestInterface
}

type V1Client struct {
	restClient rest.Interface
}

func init() {
	err := v1.AddToScheme(scheme.Scheme)
	if err != nil {
		log.Println(err.Error())
		os.Exit(1)
	}
}

func NewForConfig(c *rest.Config) (*V1Client, error) {
	config := *c
	config.ContentConfig.GroupVersion = &schema.GroupVersion{Group: v1.GroupVersion.Group, Version: v1.GroupVersion.Version}
	config.APIPath = "/apis"
	config.NegotiatedSerializer = serializer.NewCodecFactory(scheme.Scheme)
	config.UserAgent = rest.DefaultKubernetesUserAgent()

	client, err := rest.RESTClientFor(&config)
	if err != nil {
		return nil, err
	}

	return &V1Client{restClient: client}, nil
}

func (c *V1Client) ForeignClusters(namespace string) ForeignClusterInterface {
	return &foreignClusterClient{
		restClient: c.restClient,
		ns:         namespace,
	}
}

func (c *V1Client) FederationRequests(namespace string) FederationRequestInterface {
	return &federationRequestClient{
		restClient: c.restClient,
		ns:         namespace,
	}
}
