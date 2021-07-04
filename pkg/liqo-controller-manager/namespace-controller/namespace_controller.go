package namespacectrl

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	offv1alpha1 "github.com/liqotech/liqo/apis/offloading/v1alpha1"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
)

// NamespaceReconciler covers the case in which the user adds the enabling liqo label to his namespace, and the
// NamespaceOffloading resource associated with that namespace is created, if it is not already there.
type NamespaceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

const (
	nsCtrlAnnotationKey   = "liqo.io/resource-controlled-by"
	nsCtrlAnnotationValue = "This resource is created by the Namespace Controller"
)

// cluster-role
// +kubebuilder:rbac:groups=core,resources=namespaces,verbs=get;watch;list
// +kubebuilder:rbac:groups=offloading.liqo.io,resources=namespaceoffloadings,verbs=get;watch;list;create;delete

// Reconcile covers the case in which the user adds the enabling liqo label to his namespace, and the
// NamespaceOffloading resource associated with that namespace is created, if it is not already there.
func (r *NamespaceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	namespace := &corev1.Namespace{}
	if err := r.Get(ctx, req.NamespacedName, namespace); err != nil {
		if apierrors.IsNotFound(err) {
			klog.Infof("There is no namespace called '%s' in the cluster", req.Name)
			return ctrl.Result{}, nil
		}
		klog.Errorf("%s --> Unable to get namespace '%s'", err, req.Name)
		return ctrl.Result{}, err
	}

	namespaceOffloading := &offv1alpha1.NamespaceOffloading{}
	var namespaceOffloadingIsPresent bool
	checkPresenceErr := r.Get(ctx, types.NamespacedName{
		Namespace: namespace.Name,
		Name:      liqoconst.DefaultNamespaceOffloadingName,
	}, namespaceOffloading)

	// Check if a NamespaceOffloading resource called "offloading" is present in this Namespace.
	switch {
	case checkPresenceErr == nil:
		namespaceOffloadingIsPresent = true
	case apierrors.IsNotFound(checkPresenceErr):
		namespaceOffloadingIsPresent = false
	default:
		klog.Errorf("%s --> Unable to get NamespaceOffloading for the namespace '%s'",
			checkPresenceErr, req.Name)
		return ctrl.Result{}, checkPresenceErr
	}

	// Check if enabling Liqo Label is added, if there is no NamespaceOffloading
	// resource called "offloading", create it.
	if isLiqoEnabledLabelPresent(namespace.Labels) && !namespaceOffloadingIsPresent {
		if err := r.CreateNamespaceOffloading(ctx, namespace); err != nil {
			return ctrl.Result{}, err
		}
	}

	// If enabling Liqo label is removed, and there is a NamespaceOffloading owned by the controller, delete it
	if !isLiqoEnabledLabelPresent(namespace.Labels) && namespaceOffloadingIsPresent {
		if err := r.DeleteNamespaceOffloadingIfOwned(ctx, namespaceOffloading); err != nil {
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

// SetupWithManager reconciles only when a Namespace is involved in Liqo logic.
func (r *NamespaceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Namespace{}).
		WithEventFilter(manageLabelPredicate()).
		Complete(r)
}
