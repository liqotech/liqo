// Copyright 2019-2022 The Liqo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package resourcerequestoperator

import (
	"context"

	capsulev1beta1 "github.com/clastix/capsule/api/v1beta1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	sharingv1alpha1 "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
	foreigncluster "github.com/liqotech/liqo/pkg/utils/foreignCluster"
)

const tenantFinalizer = "liqo.io/tenant"

func (r *ResourceRequestReconciler) ensureTenant(ctx context.Context,
	resourceRequest *discoveryv1alpha1.ResourceRequest) (requireUpdate bool, err error) {
	// We don't use resourceRequest.Spec.ClusterIdentity directly because we might use a different ClusterName locally
	remoteCluster, err := foreigncluster.GetForeignClusterByID(ctx, r.Client, resourceRequest.Spec.ClusterIdentity.ClusterID)
	if err != nil {
		klog.Error(err)
		return false, err
	}
	remoteClusterIdentity := remoteCluster.Spec.ClusterIdentity
	klog.Infof("%s -> creating Tenant %s",
		remoteClusterIdentity.ClusterName, GetTenantName(remoteClusterIdentity))

	tenant := &capsulev1beta1.Tenant{
		ObjectMeta: metav1.ObjectMeta{
			Name: GetTenantName(remoteClusterIdentity),
		},
	}
	_, err = controllerutil.CreateOrUpdate(ctx, r.Client, tenant, func() error {
		tenant.Spec = capsulev1beta1.TenantSpec{
			NamespaceOptions: &capsulev1beta1.NamespaceOptions{
				AdditionalMetadata: &capsulev1beta1.AdditionalMetadataSpec{
					Annotations: map[string]string{
						liqoconst.RemoteNamespaceAnnotationKey: remoteClusterIdentity.ClusterID,
					},
				},
			},
			Owners: []capsulev1beta1.OwnerSpec{
				{
					Name: remoteClusterIdentity.ClusterID,
					Kind: rbacv1.UserKind,
				},
			},
			AdditionalRoleBindings: []capsulev1beta1.AdditionalRoleBindingsSpec{
				{
					ClusterRoleName: "liqo-virtual-kubelet-remote",
					Subjects: []rbacv1.Subject{
						{
							Kind: rbacv1.UserKind,
							Name: remoteClusterIdentity.ClusterID,
						},
					},
				},
			},
		}
		return nil
	})
	if err != nil {
		return false, err
	}

	if !controllerutil.ContainsFinalizer(resourceRequest, tenantFinalizer) {
		klog.Infof("%s -> adding %s finalizer", remoteClusterIdentity.ClusterName, tenantFinalizer)
		controllerutil.AddFinalizer(resourceRequest, tenantFinalizer)
		return true, nil
	}

	return false, nil
}

func (r *ResourceRequestReconciler) ensureTenantDeletion(ctx context.Context,
	resourceRequest *discoveryv1alpha1.ResourceRequest) (requireUpdate bool, err error) {
	// We don't use resourceRequest.Spec.ClusterIdentity directly because we might use a different ClusterName locally
	remoteCluster, err := foreigncluster.GetForeignClusterByID(ctx, r.Client, resourceRequest.Spec.ClusterIdentity.ClusterID)
	if err != nil {
		klog.Error(err)
		return false, err
	}
	remoteClusterIdentity := remoteCluster.Spec.ClusterIdentity

	tenant := &capsulev1beta1.Tenant{
		ObjectMeta: metav1.ObjectMeta{
			Name: GetTenantName(remoteClusterIdentity),
		},
	}
	err = r.Client.Delete(ctx, tenant)
	if apierrors.IsNotFound(err) {
		// ignore not found
		return false, nil
	}
	if err != nil {
		klog.Error(err)
		return false, err
	}

	controllerutil.RemoveFinalizer(resourceRequest, tenantFinalizer)
	return true, nil
}

func (r *ResourceRequestReconciler) checkOfferState(ctx context.Context,
	resourceRequest *discoveryv1alpha1.ResourceRequest) error {

	var resourceOffer sharingv1alpha1.ResourceOffer
	err := r.Client.Get(ctx, types.NamespacedName{
		Name:      getOfferName(r.HomeCluster),
		Namespace: resourceRequest.GetNamespace(),
	}, &resourceOffer)
	if err != nil && !apierrors.IsNotFound(err) {
		klog.Error(err)
		return err
	}

	if apierrors.IsNotFound(err) {
		resourceRequest.Status.OfferState = discoveryv1alpha1.OfferStateNone
	} else {
		resourceRequest.Status.OfferState = discoveryv1alpha1.OfferStateCreated
	}

	return nil
}

// getOfferName returns the name of the ResourceOffer coming from the given cluster.
func getOfferName(cluster discoveryv1alpha1.ClusterIdentity) string {
	return cluster.ClusterName
}

// GetTenantName returns the name of the Tenant for the given cluster.
func GetTenantName(cluster discoveryv1alpha1.ClusterIdentity) string {
	return cluster.ClusterName
}
