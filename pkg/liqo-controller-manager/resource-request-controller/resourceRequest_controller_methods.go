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
	"fmt"

	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	sharingv1alpha1 "github.com/liqotech/liqo/apis/sharing/v1alpha1"
)

func (r *ResourceRequestReconciler) ensureClusterRole(ctx context.Context,
	remoteClusterIdentity discoveryv1alpha1.ClusterIdentity) error {
	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("liqo-tenant-remote-%s", GetTenantName(remoteClusterIdentity)),
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, clusterRole, func() error {
		clusterRole.Rules = []rbacv1.PolicyRule{
			{
				APIGroups:     []string{"metrics.liqo.io"},
				Resources:     []string{"scrape", "scrape/metrics"},
				Verbs:         []string{"get"},
				ResourceNames: []string{remoteClusterIdentity.ClusterID},
			},
		}
		return nil
	})
	return err
}

func (r *ResourceRequestReconciler) ensureClusterRoleBinding(ctx context.Context,
	remoteClusterIdentity discoveryv1alpha1.ClusterIdentity) error {
	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("liqo-tenant-remote-%s", GetTenantName(remoteClusterIdentity)),
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, clusterRoleBinding, func() error {
		clusterRoleBinding.RoleRef = rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     fmt.Sprintf("liqo-tenant-remote-%s", GetTenantName(remoteClusterIdentity)),
		}
		clusterRoleBinding.Subjects = []rbacv1.Subject{
			{
				Kind: rbacv1.UserKind,
				Name: remoteClusterIdentity.ClusterID,
			},
		}
		return nil
	})
	return err
}

func (r *ResourceRequestReconciler) deleteClusterRole(ctx context.Context,
	remoteClusterIdentity discoveryv1alpha1.ClusterIdentity) error {
	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("liqo-tenant-remote-%s", GetTenantName(remoteClusterIdentity)),
		},
	}
	err := r.Client.Delete(ctx, clusterRole)
	if client.IgnoreNotFound(err) != nil {
		klog.Error(err)
		return err
	}
	return nil
}

func (r *ResourceRequestReconciler) deleteClusterRoleBinding(ctx context.Context,
	remoteClusterIdentity discoveryv1alpha1.ClusterIdentity) error {
	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("liqo-tenant-remote-%s", GetTenantName(remoteClusterIdentity)),
		},
	}
	err := r.Client.Delete(ctx, clusterRoleBinding)
	if client.IgnoreNotFound(err) != nil {
		klog.Error(err)
		return err
	}
	return nil
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
