package v1

import (
	v1 "github.com/liqoTech/liqo/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
)

type ForeignClusterInterface interface {
	List(opts metav1.ListOptions) (*v1.ForeignClusterList, error)
	Get(name string, options metav1.GetOptions) (*v1.ForeignCluster, error)
	Create(*v1.ForeignCluster) (*v1.ForeignCluster, error)
}

type foreignClusterClient struct {
	restClient rest.Interface
}

func (c *foreignClusterClient) List(opts metav1.ListOptions) (*v1.ForeignClusterList, error) {
	result := v1.ForeignClusterList{}
	err := c.restClient.
		Get().
		Resource("foreignclusters").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(&result)
	return &result, err
}

func (c *foreignClusterClient) Get(name string, opts metav1.GetOptions) (*v1.ForeignCluster, error) {
	result := v1.ForeignCluster{}
	err := c.restClient.
		Get().
		Resource("foreignclusters").
		Name(name).
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(&result)
	return &result, err
}

func (c *foreignClusterClient) Create(project *v1.ForeignCluster) (*v1.ForeignCluster, error) {
	result := v1.ForeignCluster{}
	err := c.restClient.
		Post().
		Resource("foreignclusters").
		Body(project).
		Do().
		Into(&result)
	return &result, err
}
