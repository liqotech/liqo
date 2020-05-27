package v1

import (
	v1 "github.com/liqoTech/liqo/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
)

type PeeringRequestInterface interface {
	List(opts metav1.ListOptions) (*v1.PeeringRequestList, error)
	Get(name string, options metav1.GetOptions) (*v1.PeeringRequest, error)
	Create(*v1.PeeringRequest) (*v1.PeeringRequest, error)
	Delete(name string, opts metav1.DeleteOptions) error
}

type peeringRequestClient struct {
	restClient rest.Interface
}

func (c *peeringRequestClient) List(opts metav1.ListOptions) (*v1.PeeringRequestList, error) {
	result := v1.PeeringRequestList{}
	err := c.restClient.
		Get().
		Resource("peeringrequests").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(&result)
	return &result, err
}

func (c *peeringRequestClient) Get(name string, opts metav1.GetOptions) (*v1.PeeringRequest, error) {
	result := v1.PeeringRequest{}
	err := c.restClient.
		Get().
		Resource("peeringrequests").
		Name(name).
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(&result)
	return &result, err
}

func (c *peeringRequestClient) Create(request *v1.PeeringRequest) (*v1.PeeringRequest, error) {
	result := v1.PeeringRequest{}
	err := c.restClient.
		Post().
		Resource("peeringrequests").
		Body(request).
		Do().
		Into(&result)
	return &result, err
}

func (c *peeringRequestClient) Delete(name string, opts metav1.DeleteOptions) error {
	return c.restClient.
		Delete().
		Resource("peeringrequests").
		VersionedParams(&opts, scheme.ParameterCodec).
		Name(name).
		Body(&opts).
		Do().
		Error()
}
