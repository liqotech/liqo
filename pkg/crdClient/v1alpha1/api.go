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
	Namespace(namespace string) CrdClientInterface
	List(opts metav1.ListOptions) (runtime.Object, error)
	Get(name string, opts metav1.GetOptions) (runtime.Object, error)
	Create(obj runtime.Object, opts metav1.CreateOptions) (runtime.Object, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	Update(name string, obj runtime.Object, opts metav1.UpdateOptions) (runtime.Object, error)
	UpdateStatus(name string, obj runtime.Object, opts metav1.UpdateOptions) (runtime.Object, error)
	Delete(name string, opts metav1.DeleteOptions) error
}

type Client struct {
	Client rest.Interface

	api      string
	resource RegistryType
	ns       string
}

func (c *Client) Namespace(namespace string) CrdClientInterface {
	c.ns = namespace
	return c
}

func (c *Client) Get(name string, opts metav1.GetOptions) (runtime.Object, error) {
	result := reflect.New(c.resource.SingularType).Interface()
	var namespaced bool
	if c.ns != "" {
		namespaced = true
	}

	err := c.Client.
		Get().
		Resource(c.api).
		VersionedParams(&opts, scheme.ParameterCodec).
		NamespaceIfScoped(c.ns, namespaced).
		Name(name).
		Do().
		Into(result.(runtime.Object))

	return result.(runtime.Object), err
}

func (c *Client) List(opts metav1.ListOptions) (runtime.Object, error) {
	result := reflect.New(c.resource.PluralType).Interface()
	var namespaced bool
	if c.ns != "" {
		namespaced = true
	}

	err := c.Client.
		Get().
		Resource(c.api).
		VersionedParams(&opts, scheme.ParameterCodec).
		NamespaceIfScoped(c.ns, namespaced).
		Do().
		Into(result.(runtime.Object))

	return result.(runtime.Object), err
}

func (c *Client) Watch(opts metav1.ListOptions) (watch.Interface, error) {
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
		Resource(c.api).
		VersionedParams(&opts, scheme.ParameterCodec).
		NamespaceIfScoped(c.ns, namespaced).
		Timeout(timeout).
		Watch()
}

func (c *Client) Create(obj runtime.Object, opts metav1.CreateOptions) (runtime.Object, error) {
	result := reflect.New(c.resource.SingularType).Interface()

	var namespaced bool
	if c.ns != "" {
		namespaced = true
	}

	err := c.Client.
		Post().
		Resource(c.api).
		VersionedParams(&opts, scheme.ParameterCodec).
		NamespaceIfScoped(c.ns, namespaced).
		Body(obj).
		Do().
		Into(result.(runtime.Object))

	return result.(runtime.Object), err
}

func (c *Client) Delete(name string, opts metav1.DeleteOptions) error {
	var namespaced bool
	if c.ns != "" {
		namespaced = true
	}

	return c.Client.
		Delete().
		Resource(c.api).
		VersionedParams(&opts, scheme.ParameterCodec).
		NamespaceIfScoped(c.ns, namespaced).
		Name(name).
		Body(&opts).
		Do().
		Error()
}

func (c *Client) Update(name string, obj runtime.Object, opts metav1.UpdateOptions) (runtime.Object, error) {
	result := reflect.New(c.resource.SingularType).Interface()

	var namespaced bool
	if c.ns != "" {
		namespaced = true
	}

	err := c.Client.Put().
		Resource(c.api).
		VersionedParams(&opts, scheme.ParameterCodec).
		NamespaceIfScoped(c.ns, namespaced).
		Name(name).
		Body(obj).
		Do().
		Into(result.(runtime.Object))

	return result.(runtime.Object), err
}

func (c *Client) UpdateStatus(name string, obj runtime.Object, opts metav1.UpdateOptions) (runtime.Object, error) {
	result := reflect.New(c.resource.SingularType).Interface()

	var namespaced bool
	if c.ns != "" {
		namespaced = true
	}

	err := c.Client.Put().
		Resource(c.api).
		VersionedParams(&opts, scheme.ParameterCodec).
		NamespaceIfScoped(c.ns, namespaced).
		Name(name).
		SubResource("status").
		Body(obj).
		Do().
		Into(result.(runtime.Object))

	return result.(runtime.Object), err
}
