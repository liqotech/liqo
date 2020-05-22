package v1

import (
	v1 "github.com/netgroup-polito/dronev2/api/discovery/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
)

var (
	Log = ctrl.Log.WithName("discoveryV1Client")
)

type DiscoveryV1Interface interface {
	ForeignClusters() ForeignClusterInterface
	FederationRequests() FederationRequestInterface
}

type DiscoveryV1Client struct {
	restClient rest.Interface
}

func init() {
	err := v1.AddToScheme(scheme.Scheme)
	if err != nil {
		Log.Error(err, err.Error())
		os.Exit(1)
	}
}

func NewForConfig(c *rest.Config) (*DiscoveryV1Client, error) {
	config := *c
	config.ContentConfig.GroupVersion = &schema.GroupVersion{Group: v1.GroupVersion.Group, Version: v1.GroupVersion.Version}
	config.APIPath = "/apis"
	config.NegotiatedSerializer = serializer.NewCodecFactory(scheme.Scheme)
	config.UserAgent = rest.DefaultKubernetesUserAgent()

	client, err := rest.RESTClientFor(&config)
	if err != nil {
		return nil, err
	}

	return &DiscoveryV1Client{restClient: client}, nil
}

func (c *DiscoveryV1Client) ForeignClusters() ForeignClusterInterface {
	return &foreignClusterClient{
		restClient: c.restClient,
	}
}

func (c *DiscoveryV1Client) FederationRequests() FederationRequestInterface {
	return &federationRequestClient{
		restClient: c.restClient,
	}
}
