// Copyright 2019-2021 The Liqo Authors
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
	"fmt"
	"strings"

	capsulev1beta1 "github.com/clastix/capsule/api/v1beta1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	sharingv1alpha1 "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
)

const (
	tenantFinalizer        = "liqo.io/tenant"
	passthroughClusterRole = "liqo-passthrough-ClusterRole"
	liqoPublicNS           = "liqo-public"
)

func (r *ResourceRequestReconciler) ensureTenant(ctx context.Context,
	resourceRequest *discoveryv1alpha1.ResourceRequest) (requireUpdate bool, err error) {
	remoteClusterID := resourceRequest.Spec.ClusterIdentity.ClusterID
	tenant := &capsulev1beta1.Tenant{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("tenant-%v", remoteClusterID),
		},
	}
	_, err = controllerutil.CreateOrUpdate(ctx, r.Client, tenant, func() error {
		tenant.Spec = capsulev1beta1.TenantSpec{
			NamespaceOptions: &capsulev1beta1.NamespaceOptions{
				AdditionalMetadata: &capsulev1beta1.AdditionalMetadataSpec{
					Annotations: map[string]string{
						liqoconst.RemoteNamespaceAnnotationKey: resourceRequest.Spec.ClusterIdentity.ClusterID,
					},
				},
			},
			Owners: []capsulev1beta1.OwnerSpec{
				{
					Name: remoteClusterID,
					Kind: rbacv1.UserKind,
				},
			},
			AdditionalRoleBindings: []capsulev1beta1.AdditionalRoleBindingsSpec{
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

func (r *ResourceRequestReconciler) ensurePassthroughClusterRole(ctx context.Context, clusterID string) error {
	var roleBinding rbacv1.RoleBinding
	err := r.Get(ctx, types.NamespacedName{Namespace: liqoPublicNS, Name: fmt.Sprintf("passthrough-tenant-%s", clusterID)}, &roleBinding)
	if client.IgnoreNotFound(err) != nil {
		return err
	}
	if err != nil {
		// err is notfound, create binding
		clusterRole, err := r.getPassthroughClusterRole(ctx)
		if err != nil {
			return err
		}
		roleBinding := forgePassthroughRoleBinding(clusterID, clusterRole)
		if err := r.Create(ctx, roleBinding); err != nil {
			return err
		}
		klog.Infof("Passthrough ClusterRole bind to cluster %s", clusterID)
	}
	return nil
}

func (r *ResourceRequestReconciler) getPassthroughClusterRole(ctx context.Context) (*rbacv1.ClusterRole, error) {
	var cr rbacv1.ClusterRole
	if err := r.Get(ctx, types.NamespacedName{Namespace: liqoPublicNS, Name: passthroughClusterRole}, &cr); err != nil {
		return nil, err
	}
	return &cr, nil
}

func forgePassthroughRoleBinding(clusterID string, cr *rbacv1.ClusterRole) *rbacv1.RoleBinding {
	ownerRef := metav1.OwnerReference{
		APIVersion: rbacv1.SchemeGroupVersion.String(),
		Kind:       "ClusterRole",
		Name:       passthroughClusterRole,
		UID:        cr.GetUID(),
	}
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("passthrough-tenant-%s", clusterID),
			Namespace: liqoPublicNS,
			OwnerReferences: []metav1.OwnerReference{
				ownerRef,
			},
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:     rbacv1.UserKind,
				APIGroup: rbacv1.GroupName,
				Name:     clusterID,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     passthroughClusterRole,
		},
	}
}

func (r *ResourceRequestReconciler) ensureTenantDeletion(ctx context.Context,
	resourceRequest *discoveryv1alpha1.ResourceRequest) (requireUpdate bool, err error) {
	remoteClusterID := resourceRequest.Spec.ClusterIdentity.ClusterID

	tenant := &capsulev1beta1.Tenant{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("tenant-%v", remoteClusterID),
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
	name := strings.Join([]string{offerPrefix, r.ClusterID}, "")

	var resourceOffer sharingv1alpha1.ResourceOffer
	err := r.Client.Get(ctx, types.NamespacedName{
		Name:      name,
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
