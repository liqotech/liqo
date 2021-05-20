package namespacectrl

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"

	offv1alpha1 "github.com/liqotech/liqo/apis/offloading/v1alpha1"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
)

// CreateNamespaceOffloading creates a NamespaceOffloading resource with an annotation which represents the ownership
// of the controller on this resource (this annotation will be useful during deletion phase).
func (r *NamespaceReconciler) CreateNamespaceOffloading(ctx context.Context,
	namespace *corev1.Namespace) error {
	namespaceOffloading := &offv1alpha1.NamespaceOffloading{
		ObjectMeta: metav1.ObjectMeta{
			Name:      liqoconst.DefaultNamespaceOffloadingName,
			Namespace: namespace.Name,
			Annotations: map[string]string{
				nsCtrlAnnotationKey: nsCtrlAnnotationValue,
			},
		},
		Spec: offv1alpha1.NamespaceOffloadingSpec{
			NamespaceMappingStrategy: offv1alpha1.DefaultNameMappingStrategyType,
			PodOffloadingStrategy:    offv1alpha1.LocalAndRemotePodOffloadingStrategyType,
			ClusterSelector:          corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{}},
		},
	}
	if err := r.Create(ctx, namespaceOffloading); err != nil {
		klog.Errorf("%s --> unable to Create NamespaceOffloading in namespace '%s'", err, namespace.Name)
		return err
	}
	return nil
}

// DeleteNamespaceOffloadingIfOwned checks if the NamespaceOffloading resource is owned
// by the controller and if so, delete it.
func (r *NamespaceReconciler) DeleteNamespaceOffloadingIfOwned(ctx context.Context,
	namespaceOffloading *offv1alpha1.NamespaceOffloading) error {
	if value, ok := namespaceOffloading.Annotations[nsCtrlAnnotationKey]; ok && value == nsCtrlAnnotationValue {
		if err := r.Delete(ctx, namespaceOffloading); err != nil {
			klog.Errorf("%s --> Unable to remove NamespaceOffloading for the namespace '%s'",
				err, namespaceOffloading.Namespace)
			return err
		}
	}
	return nil
}
