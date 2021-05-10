package garbage_collection

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog"

	"github.com/liqotech/liqo/pkg/discovery"
)

// delete ClusterRoles and ClusterRoleBindings related to a ServiceAccount. We cannot do it setting an OwnerReference
// on them and let the Kubernetes garbage collector to do that, due to the fact they (cluster scoped resources) need
// to be deleted after the deletion of a ServiceAccount (namespaced resource). See https://kubernetes.io/docs/concepts/workloads/controllers/garbage-collection/#owners-and-dependents
// to have more details on how OwnerReferences are handled by Kubernetes >= 1.20
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
		klog.V(4).Infof("%v, retrying...", err)
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
		klog.V(4).Infof("%v, retrying...", err)
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
	finalizerPatch := []byte(fmt.Sprintf(
		`[{"op":"remove","path":"/metadata/finalizers","value":["%s"]}]`,
		discovery.GarbageCollection))

	if _, err := client.CoreV1().ServiceAccounts(serviceAccount.Namespace).Patch(context.TODO(),
		serviceAccount.Name,
		types.JSONPatchType,
		finalizerPatch,
		metav1.PatchOptions{}); err != nil {
		klog.Error(err)
		return
	}

	klog.Infof("[%v] ServiceAccount successfully purged", remoteClusterId)
}
