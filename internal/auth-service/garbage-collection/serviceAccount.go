package garbage_collection

import (
	"context"
	"github.com/liqotech/liqo/pkg/discovery"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// delete ClusterRoles and ClusterRoleBindings related to a ServiceAccount, this garbage collection is not automatically done
// by the default garbage collector starting from Kubernetes versions 1.20
func OnDeleteServiceAccount(client kubernetes.Interface, serviceAccount *v1.ServiceAccount) {
	if liqoManaged, ok := serviceAccount.Labels[discovery.LiqoManagedLabel]; !ok || liqoManaged != "true" {
		// it is not a Liqo Managed ServiceAccount
		return
	}

	remoteClusterId, ok := serviceAccount.Labels[discovery.ClusterIdLabel]
	if !ok {
		klog.Errorf("No %v label is set on ServiceAccount %v/%v", discovery.ClusterIdLabel, serviceAccount.Namespace, serviceAccount.Name)
		return
	}

	klog.Infof("[%v] Purging ServiceAccount", remoteClusterId)

	labelSelector := metav1.LabelSelector{
		MatchLabels: map[string]string{
			discovery.LiqoManagedLabel: "true",
			discovery.ClusterIdLabel:   remoteClusterId,
		},
	}

	// delete ClusterRoleBindings
	if err := retry.OnError(retry.DefaultRetry, func(err error) bool {
		return true
	}, func() error {
		return client.RbacV1().ClusterRoleBindings().DeleteCollection(context.TODO(), metav1.DeleteOptions{}, metav1.ListOptions{
			LabelSelector: labels.Set(labelSelector.MatchLabels).String(),
		})
	}); err != nil {
		klog.Error(err)
		return
	}

	// delete ClusterRoles
	if err := retry.OnError(retry.DefaultRetry, func(err error) bool {
		return true
	}, func() error {
		return client.RbacV1().ClusterRoles().DeleteCollection(context.TODO(), metav1.DeleteOptions{}, metav1.ListOptions{
			LabelSelector: labels.Set(labelSelector.MatchLabels).String(),
		})
	}); err != nil {
		klog.Error(err)
		return
	}

	// remove the finalizer
	controllerutil.RemoveFinalizer(serviceAccount, discovery.GarbageCollection)
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var err error
		serviceAccount, err = client.CoreV1().ServiceAccounts(serviceAccount.Namespace).Update(context.TODO(), serviceAccount, metav1.UpdateOptions{})
		return err
	}); err != nil {
		klog.Error(err)
		return
	}

	klog.Infof("[%v] ServiceAccount successfully purged", remoteClusterId)
}
