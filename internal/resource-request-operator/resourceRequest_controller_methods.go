package resourcerequestoperator

import (
	"context"
	"fmt"

	capsulev1alpha1 "github.com/clastix/capsule/api/v1alpha1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
)

const tenantFinalizer = "liqo.io/tenant"

func requireTenantDeletion(resourceRequest *discoveryv1alpha1.ResourceRequest) bool {
	return !resourceRequest.GetDeletionTimestamp().IsZero() && controllerutil.ContainsFinalizer(resourceRequest, tenantFinalizer)
}

func (r *ResourceRequestReconciler) ensureTenant(ctx context.Context,
	resourceRequest *discoveryv1alpha1.ResourceRequest) (requireUpdate bool, err error) {
	remoteClusterID := resourceRequest.Spec.ClusterIdentity.ClusterID
	tenant := &capsulev1alpha1.Tenant{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("tenant-%v", remoteClusterID),
		},
	}
	_, err = controllerutil.CreateOrUpdate(ctx, r.Client, tenant, func() error {
		tenant.Spec = capsulev1alpha1.TenantSpec{
			NamespacesMetadata: capsulev1alpha1.AdditionalMetadata{
				AdditionalAnnotations: map[string]string{
					liqoconst.RemoteNamespaceAnnotationKey: resourceRequest.Spec.ClusterIdentity.ClusterID,
				},
			},
			Owner: capsulev1alpha1.OwnerSpec{
				Name: remoteClusterID,
				Kind: rbacv1.UserKind,
			},
			AdditionalRoleBindings: []capsulev1alpha1.AdditionalRoleBindings{
				{
					ClusterRoleName: "liqo-virtual-kubelet-remote",
					Subjects: []rbacv1.Subject{
						{
							Kind: rbacv1.UserKind,
							Name: remoteClusterID,
						},
					},
				},
			},
		}
		return nil
	})
	if err != nil {
		klog.Error(err)
		return false, err
	}

	if !controllerutil.ContainsFinalizer(resourceRequest, tenantFinalizer) {
		klog.Infof("%s -> adding %s finalizer", remoteClusterID, tenantFinalizer)
		controllerutil.AddFinalizer(resourceRequest, tenantFinalizer)
		return true, nil
	}

	return false, nil
}

func (r *ResourceRequestReconciler) ensureTenantDeletion(ctx context.Context,
	resourceRequest *discoveryv1alpha1.ResourceRequest) error {
	remoteClusterID := resourceRequest.Spec.ClusterIdentity.ClusterID

	tenant := &capsulev1alpha1.Tenant{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("tenant-%v", remoteClusterID),
		},
	}
	err := r.Client.Delete(ctx, tenant)
	if err != nil {
		klog.Error(err)
		return err
	}

	controllerutil.RemoveFinalizer(resourceRequest, tenantFinalizer)
	return nil
}
