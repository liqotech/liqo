package crdClient

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
)

type CrdClientInterface interface {
	Namespace(namespace string) CrdClientInterface
	List(opts *metav1.ListOptions) (runtime.Object, error)
	Get(name string, opts *metav1.GetOptions) (runtime.Object, error)
	Create(obj runtime.Object, opts *metav1.CreateOptions) (runtime.Object, error)
	Watch(opts *metav1.ListOptions) (watch.Interface, error)
	Update(name string, obj runtime.Object, opts *metav1.UpdateOptions) (runtime.Object, error)
	UpdateStatus(name string, obj runtime.Object, opts *metav1.UpdateOptions) (runtime.Object, error)
	Delete(name string, opts *metav1.DeleteOptions) error
}
