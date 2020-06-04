package v1

import (
	v1 "github.com/liqoTech/liqo/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
)

type FederationRequestInterface interface {
	List(opts metav1.ListOptions) (*v1.FederationRequestList, error)
	Get(name string, options metav1.GetOptions) (*v1.FederationRequest, error)
	Create(*v1.FederationRequest) (*v1.FederationRequest, error)
}

type federationRequestClient struct {
	restClient rest.Interface
}

func (c *federationRequestClient) List(opts metav1.ListOptions) (*v1.FederationRequestList, error) {
	result := v1.FederationRequestList{}
	err := c.restClient.
		Get().
		Resource("federationrequests").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(&result)
	return &result, err
}

func (c *federationRequestClient) Get(name string, opts metav1.GetOptions) (*v1.FederationRequest, error) {
	result := v1.FederationRequest{}
	err := c.restClient.
		Get().
		Resource("federationrequests").
		Name(name).
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(&result)
	return &result, err
}

func (c *federationRequestClient) Create(project *v1.FederationRequest) (*v1.FederationRequest, error) {
	result := v1.FederationRequest{}
	err := c.restClient.
		Post().
		Resource("federationrequests").
		Body(project).
		Do().
		Into(&result)
	return &result, err
}
