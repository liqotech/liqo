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

package namespacemapctrl

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	mapsv1alpha1 "github.com/liqotech/liqo/apis/virtualkubelet/v1alpha1"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
	foreigncluster "github.com/liqotech/liqo/pkg/utils/foreignCluster"
)

// This function creates a remote Namespace inside the remote cluster, if it doesn't exist yet.
// The right client to use is chosen by means of NamespaceMap's cluster-id.
func (r *NamespaceMapReconciler) createRemoteNamespace(ctx context.Context,
	remoteCluster discoveryv1alpha1.ClusterIdentity, remoteNamespaceName string) error {
	if err := r.checkRemoteClientPresence(remoteCluster); err != nil {
		return err
	}

	// Todo: at the moment the capsule controller removes this annotation, so the Tenant will create directly
	//       this annotation on its namespaces.
	// This annotation is used to recognize the remote namespaces that have been created by this controller.
	remoteNamespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: remoteNamespaceName,
			Annotations: map[string]string{
				liqoconst.RemoteNamespaceAnnotationKey: r.LocalCluster.ClusterID,
			},
		},
	}

	var err error
	// Trying to create the remote namespace.
	if _, err = r.RemoteClients[remoteCluster.ClusterID].CoreV1().Namespaces().Create(ctx,
		remoteNamespace, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
		klog.Errorf("%s -> unable to create the remote namespace '%s' inside the remote cluster '%s'",
			err, remoteNamespaceName, remoteCluster.ClusterName)
		return err
	}

	// Whether the remote namespace is created successfully or already exists, is necessary to:
	// 1 - Try to get it
	if remoteNamespace, err = r.RemoteClients[remoteCluster.ClusterID].CoreV1().Namespaces().Get(
		ctx, remoteNamespaceName, metav1.GetOptions{}); err != nil {
		klog.Errorf("%s -> unable to get the remote namespace '%s'", err, remoteNamespaceName)
		return err
	}
	// 2 - Check if the remote namespace was created by the NamespaceMap controller.
	if value, ok := remoteNamespace.Annotations[liqoconst.RemoteNamespaceAnnotationKey]; !ok || value != r.LocalCluster.ClusterID {
		err := apierrors.NewAlreadyExists(schema.GroupResource{},
			fmt.Sprintf("the remote namespace '%s', inside the remote cluster '%s', does not have the annotation '%s'",
				remoteNamespaceName, remoteCluster.ClusterName, liqoconst.RemoteNamespaceAnnotationKey))
		klog.Error(err.Error())
		return err
	}
	// 3 - Check if the virtual kubelet will have the right privileges on the remote namespace.
	if err = checkRemoteNamespaceRoleBindings(ctx, r.RemoteClients[remoteCluster.ClusterID], remoteNamespaceName, r.LocalCluster); err != nil {
		return err
	}

	klog.Infof("The namespace '%s' is correctly inside the remote cluster: '%s'", remoteNamespaceName, remoteCluster.ClusterName)
	return nil
}

// For every entry of DesiredMapping create remote Namespace if it has not already being created.
// ensureRemoteNamespacesExistence tries to create all the remote namespaces requested in DesiredMapping (NamespaceMap->Spec->DesiredMapping).
func (r *NamespaceMapReconciler) ensureRemoteNamespacesExistence(
	ctx context.Context, nm *mapsv1alpha1.NamespaceMap) error {
	var err error
	for localName, remoteName := range nm.Spec.DesiredMapping {
		var remoteCluster *discoveryv1alpha1.ForeignCluster
		remoteCluster, err = foreigncluster.GetForeignClusterByID(ctx, r.Client, nm.Labels[liqoconst.RemoteClusterID])
		if err != nil {
			continue
		}
		if err = r.createRemoteNamespace(ctx, remoteCluster.Spec.ClusterIdentity, remoteName); err != nil {
			nm.Status.CurrentMapping[localName] = mapsv1alpha1.RemoteNamespaceStatus{
				RemoteNamespace: remoteName,
				Phase:           mapsv1alpha1.MappingCreationLoopBackOff,
			}
			if apierrors.IsInvalid(err) || apierrors.IsAlreadyExists(err) {
				// Do not report this error upstream
				err = nil
			}
			continue
		}
		nm.Status.CurrentMapping[localName] = mapsv1alpha1.RemoteNamespaceStatus{
			RemoteNamespace: remoteName,
			Phase:           mapsv1alpha1.MappingAccepted,
		}
	}
	return err
}

// deleteRemoteNamespace deletes a remote Namespace from the remote cluster, the right client to use is chosen
// by NamespaceMap's cluster-id. This function return nil (success) only when the remote Namespace is really deleted,
// so the get Api returns a "NotFound".
func (r *NamespaceMapReconciler) deleteRemoteNamespace(ctx context.Context,
	remoteCluster discoveryv1alpha1.ClusterIdentity, remoteNamespaceName string) error {
	if err := r.checkRemoteClientPresence(remoteCluster); err != nil {
		return err
	}

	var err error
	remoteNamespace := &corev1.Namespace{}
	if remoteNamespace, err = r.RemoteClients[remoteCluster.ClusterID].CoreV1().Namespaces().Get(
		ctx, remoteNamespaceName, metav1.GetOptions{}); err != nil {
		if apierrors.IsNotFound(err) || apierrors.IsForbidden(err) {
			klog.Infof("The namespace '%s' is correctly deleted from the remote cluster: '%s'", remoteNamespaceName, remoteCluster.ClusterName)
			return nil
		}
		klog.Errorf("%s -> unable to get the remote namespace '%s'", err, remoteNamespaceName)
		return err
	}

	// Check if it is a namespace created by the NamespaceMap controller.
	if value, ok := remoteNamespace.Annotations[liqoconst.RemoteNamespaceAnnotationKey]; !ok || value != r.LocalCluster.ClusterID {
		klog.Infof("No remote namespaces with name '%s' created through Liqo mechanism inside the remote cluster: '%s'",
			remoteNamespaceName, remoteCluster.ClusterName)
		return nil
	}

	if remoteNamespace.DeletionTimestamp.IsZero() {
		if err = r.RemoteClients[remoteCluster.ClusterID].CoreV1().Namespaces().Delete(ctx, remoteNamespaceName, metav1.DeleteOptions{}); err != nil {
			klog.Errorf("unable to delete the namespace '%s' from the remote cluster: '%s'", remoteNamespaceName, remoteCluster.ClusterName)
			return err
		}
		klog.Infof("The deletion timestamp is correctly set on the remote namespace '%s'", remoteNamespaceName)
	}
	return fmt.Errorf("the remote namespace '%s' inside the cluster '%s' is undergoing graceful termination",
		remoteNamespaceName, remoteCluster.ClusterName)
}

// If DesiredMapping field has less entries than CurrentMapping, is necessary to remove some remote namespaces.
// This function checks if remote namespaces requested to be deleted are really deleted.
func (r *NamespaceMapReconciler) ensureRemoteNamespacesDeletion(ctx context.Context, nm *mapsv1alpha1.NamespaceMap) error {
	var err error
	for localName, remoteStatus := range nm.Status.CurrentMapping {
		if _, ok := nm.Spec.DesiredMapping[localName]; ok && nm.DeletionTimestamp.IsZero() {
			continue
		}
		var remoteCluster *discoveryv1alpha1.ForeignCluster
		remoteCluster, err = foreigncluster.GetForeignClusterByID(ctx, r.Client, nm.Labels[liqoconst.RemoteClusterID])
		if err != nil {
			continue
		}
		if err = r.deleteRemoteNamespace(ctx, remoteCluster.Spec.ClusterIdentity, remoteStatus.RemoteNamespace); err != nil {
			nm.Status.CurrentMapping[localName] = mapsv1alpha1.RemoteNamespaceStatus{
				RemoteNamespace: remoteStatus.RemoteNamespace,
				Phase:           mapsv1alpha1.MappingTerminating,
			}
			continue
		}
		delete(nm.Status.CurrentMapping, localName)
	}
	return err
}

// This function checks if there are Namespaces that have to be created or deleted, in accordance with DesiredMapping
// field. It updates also NamespaceOffloading status in according to the remote Namespace Status.
func (r *NamespaceMapReconciler) ensureRemoteNamespaces(ctx context.Context, nm *mapsv1alpha1.NamespaceMap) error {
	if nm.Status.CurrentMapping == nil {
		nm.Status.CurrentMapping = map[string]mapsv1alpha1.RemoteNamespaceStatus{}
	}
	// Object used as base for client.MergeFrom
	original := nm.DeepCopy()

	errorCreationPhase := r.ensureRemoteNamespacesExistence(ctx, nm)
	errorDeletionPhase := r.ensureRemoteNamespacesDeletion(ctx, nm)

	// MergeFrom used to avoid conflicts, the NamespaceMap controller has the ownership of NamespaceMap status
	if err := r.Patch(ctx, nm, client.MergeFrom(original)); err != nil {
		klog.Errorf("%s -> unable to update the NamespaceMap '%s' Status", err, nm.Name)
		return err
	}

	if errorCreationPhase != nil {
		return fmt.Errorf("creating remote namespaces: %w", errorCreationPhase)
	}
	if errorDeletionPhase != nil {
		return fmt.Errorf("deleting remote namespaces: %w", errorDeletionPhase)
	}

	klog.Infof("the status of the NamespaceMap '%s' is correctly updated", nm.Name)
	return nil
}

// The NamespaceMap is requested to be deleted so before removing the NamespaceMapControllerFinalizer finalizer
// is necessary to delete all remote namespaces associated with this NamespaceMap.
func (r *NamespaceMapReconciler) namespaceMapDeletionProcess(ctx context.Context, nm *mapsv1alpha1.NamespaceMap) error {
	original := nm.DeepCopy()
	// If the NamespaceMap Status is empty, it is possible to remove the finalizer.
	if err := r.ensureRemoteNamespacesDeletion(ctx, nm); err == nil {
		return r.RemoveNamespaceMapControllerFinalizer(ctx, nm)
	}
	// This patch is used to update NamespaceMap status if some remote namespaces still exist.
	if err := r.Patch(ctx, nm, client.MergeFrom(original)); err != nil {
		klog.Errorf("%s -> unable to patch the status of the NamespaceMap %s", err, nm.Name)
		return err
	}
	return fmt.Errorf("remote namespaces deletion phase in progress")
}

// checkRemoteNamespaceRoleBindings checks that the right roleBindings are inside the remote namespace to understand if the
// virtual kubelet will have the right privileges on that namespace.
func checkRemoteNamespaceRoleBindings(ctx context.Context, cl kubernetes.Interface,
	remoteNamespaceName string, localCluster discoveryv1alpha1.ClusterIdentity) error {
	roleBindingLabelValue := fmt.Sprintf("%s-%s", liqoconst.RoleBindingLabelValuePrefix, localCluster.ClusterID)
	roleBindingList := &rbacv1.RoleBindingList{}
	labelSelector := metav1.LabelSelector{
		MatchLabels: map[string]string{
			liqoconst.RoleBindingLabelKey: roleBindingLabelValue,
		},
	}

	var err error
	if roleBindingList, err = cl.RbacV1().RoleBindings(remoteNamespaceName).List(ctx, metav1.ListOptions{
		LabelSelector: labels.Set(labelSelector.MatchLabels).String(),
	}); err != nil {
		klog.Errorf("%s -> unable to list roleBindings in the remote namespace '%s'", err, remoteNamespaceName)
		return err
	}
	if len(roleBindingList.Items) < 3 {
		err = fmt.Errorf("not enough roleBinding in the remote namespace '%s'. Virtual kubelet will not have "+
			"the necessary privileges", remoteNamespaceName)
		klog.Error(err)
		return err
	}
	return nil
}
