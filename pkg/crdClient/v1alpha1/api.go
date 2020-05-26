package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"reflect"
	"time"
)

type CrdClientInterface interface {
	List(resources string, opts metav1.ListOptions) (runtime.Object, error)
	Get(resources, resource, name string, opts metav1.GetOptions) (runtime.Object, error)
	Create(resources, resource string, obj runtime.Object, opts metav1.CreateOptions) (runtime.Object, error)
	Watch(resources string, opts metav1.ListOptions) (watch.Interface, error)
	Update(resources, resource, name string, obj runtime.Object, opts metav1.UpdateOptions) (runtime.Object, error)
	Delete(resource, name string, opts metav1.DeleteOptions) error
}

type Client struct {
	Client rest.Interface

	ns string
}

func (c *Client) Get(resources, resource, name string, opts metav1.GetOptions) (runtime.Object, error) {
	result := reflect.New(Registry[resource]).Interface()
	var namespaced bool
	if c.ns != "" {
		namespaced = true
	}

	err := c.Client.
		Get().
		Resource(resources).
		VersionedParams(&opts, scheme.ParameterCodec).
		NamespaceIfScoped(c.ns, namespaced).
		Name(name).
		Do().
		Into(result.(runtime.Object))

	return result.(runtime.Object), err
}


func (c *Client) List(resources string, opts metav1.ListOptions) (runtime.Object, error) {
	result := reflect.New(Registry[resources]).Interface()
	var namespaced bool
	if c.ns != "" {
		namespaced = true
	}

	err := c.Client.
		Get().
		Resource(resources).
		VersionedParams(&opts, scheme.ParameterCodec).
		NamespaceIfScoped(c.ns, namespaced).
		Do().
		Into(result.(runtime.Object))

	return result.(runtime.Object), err
}

func (c *Client) Watch(resources string, opts metav1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true

	var namespaced bool
	if c.ns != "" {
		namespaced = true
	}

	return c.Client.
		Get().
		Resource(resources).
		VersionedParams(&opts, scheme.ParameterCodec).
		NamespaceIfScoped(c.ns, namespaced).
		Timeout(timeout).
		Watch()
}

func (c *Client) Create(resources, resource string, obj runtime.Object, opts metav1.CreateOptions) (runtime.Object, error) {
	result := reflect.New(Registry[resource]).Interface()

	var namespaced bool
	if c.ns != "" {
		namespaced = true
	}

	err := c.Client.
		Post().
		Resource(resources).
		VersionedParams(&opts, scheme.ParameterCodec).
		NamespaceIfScoped(c.ns, namespaced).
		Body(obj).
		Do().
		Into(result.(runtime.Object))

	return result.(runtime.Object), err
}


func (c *Client) Delete(resources, name string, opts metav1.DeleteOptions) error {
	var namespaced bool
	if c.ns != "" {
		namespaced = true
	}

	return c.Client.
		Delete().
		Resource(resources).
		VersionedParams(&opts, scheme.ParameterCodec).
		NamespaceIfScoped(c.ns, namespaced).
		Name(name).
		Body(&opts).
		Do().
		Error()
}

func (c *Client) Update(resources, resource, name string, obj runtime.Object, opts metav1.UpdateOptions) (runtime.Object, error) {
	result := reflect.New(Registry[resource]).Interface()

	var namespaced bool
	if c.ns != "" {
		namespaced = true
	}

	err := c.Client.Put().
		Namespace(c.ns).
		Resource(resources).
		VersionedParams(&opts, scheme.ParameterCodec).
		NamespaceIfScoped(c.ns, namespaced).
		Name(name).
		Body(obj).
		Do().
		Into(result.(runtime.Object))

	return result.(runtime.Object), err
}

