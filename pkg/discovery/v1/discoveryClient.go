package v1

import (
	v1 "github.com/liqoTech/liqo/api/discovery/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/klog"
	"os"
)

type DiscoveryV1Interface interface {
	ForeignClusters() ForeignClusterInterface
	PeeringRequests() PeeringRequestInterface
}

type DiscoveryV1Client struct {
	restClient rest.Interface
}

func init() {
	err := v1.AddToScheme(scheme.Scheme)
	if err != nil {
		klog.Error(err, err.Error())
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

func (c *DiscoveryV1Client) PeeringRequests() PeeringRequestInterface {
	return &peeringRequestClient{
		restClient: c.restClient,
	}
}
