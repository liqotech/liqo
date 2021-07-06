package util

import (
	"context"

	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	"github.com/liqotech/liqo/test/e2e/testutils"
)

// EnforceNamespace creates and returns a namespace. If it already exists, it just returns the namespace.
func EnforceNamespace(ctx context.Context, client kubernetes.Interface, clusterID, name string) (*v1.Namespace, error) {
	ns := &v1.Namespace{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: testutils.LiqoTestNamespaceLabels,
		},
		Spec:   v1.NamespaceSpec{},
		Status: v1.NamespaceStatus{},
	}
	ns, err := client.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	if kerrors.IsAlreadyExists(err) {
		ns, err = client.CoreV1().Namespaces().Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			klog.Errorf("%s -> an error occurred while creating namespace %s : %s", clusterID, name, err)
			return nil, err
		}
	} else if err != nil {
		klog.Errorf("%s -> an error occurred while creating namespace %s : %s", clusterID, name, err)
		return nil, err
	}
	return ns, nil
}

// DeleteNamespace wrap the deletion of a namespace.
func DeleteNamespace(ctx context.Context, client kubernetes.Interface, labelSelector map[string]string) error {
	list, err := client.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(labelSelector).String(),
	})
	if err != nil && !kerrors.IsNotFound(err) {
		return err
	}
	for i := range list.Items {
		if e := client.CoreV1().Namespaces().Delete(ctx, list.Items[i].Name, metav1.DeleteOptions{}); e != nil && !kerrors.IsNotFound(err) {
			return e
		}
	}
	return nil
}
