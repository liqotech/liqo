package namespacemapctrl

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"

	mapsv1alpha1 "github.com/liqotech/liqo/apis/virtualKubelet/v1alpha1"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
)

// This constants are used to compose the annotation that is inserted on remote Namespace at creation time.
const (
	liqoSuffix                      = "liqo.io"
	remoteNamespaceAnnotationPrefix = "remote-namespace"
	remoteNamespaceAnnotationValue  = "This Namespace has been created through Liqo offloading mechanism"
)

// This function creates a remote Namespace inside the remote cluster, if it doesn't exist yet.
// The right client to use is chosen by means of NamespaceMap's cluster-id.
func (r *NamespaceMapReconciler) createRemoteNamespace(ctx context.Context, remoteClusterID, remoteNamespaceName string) error {
	if err := r.checkRemoteClientPresence(ctx, remoteClusterID); err != nil {
		return err
	}

	// This annotation is used to recognize the remote namespaces that have been created by this controller.
	annotationKey := fmt.Sprintf("%s-%s/%s", remoteNamespaceAnnotationPrefix, r.LocalClusterID, liqoSuffix)
	remoteNamespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: remoteNamespaceName,
			Annotations: map[string]string{
				annotationKey: remoteNamespaceAnnotationValue,
			},
		},
	}

	// Trying to create the remote namespace.
	if err := r.RemoteClients[remoteClusterID].Create(ctx, remoteNamespace); err != nil && !apierrors.IsAlreadyExists(err) {
		klog.Errorf("%s -> unable to create the remote namespace '%s' inside the remote cluster '%s'",
			err, remoteNamespaceName, remoteClusterID)
		return err
	}

	// Whether the remote namespace is created successfully or already exists, is necessary to:
	// 1 - Try to get it
	if err := r.RemoteClients[remoteClusterID].Get(ctx, types.NamespacedName{Name: remoteNamespaceName}, remoteNamespace); err != nil {
		klog.Errorf("%s -> unable to get the remote namespace '%s'", err, remoteNamespaceName)
		return err
	}
	// 2 - Check if the remote namespace was created by the NamespaceMap controller.
	if value, ok := remoteNamespace.Annotations[annotationKey]; !ok || value != remoteNamespaceAnnotationValue {
		err := fmt.Errorf("a namespace with the name '%s' already exists inside the remote cluster '%s'",
			remoteNamespaceName, remoteClusterID)
		klog.Error(err)
		return err
	}
	// 3 - Check if the virtual kubelet will have the right privileges on the remote namespace.
	if err := checkRemoteNamespacePrivileges(ctx, r.RemoteClients[remoteClusterID], remoteNamespaceName, r.LocalClusterID); err != nil {
		return err
	}

	klog.Infof("The namespace '%s' is correctly inside the remote cluster: '%s'", remoteNamespaceName, remoteClusterID)
	return nil
}

// For every entry of DesiredMapping create remote Namespace if it has not already being created.
// ensureRemoteNamespacesExistence tries to create all the remote namespaces requested in DesiredMapping (NamespaceMap->Spec->DesiredMapping).
func (r *NamespaceMapReconciler) ensureRemoteNamespacesExistence(ctx context.Context, nm *mapsv1alpha1.NamespaceMap) bool {
	errorCondition := false
	for localName, remoteName := range nm.Spec.DesiredMapping {
		if err := r.createRemoteNamespace(ctx, nm.Labels[liqoconst.RemoteClusterID], remoteName); err != nil {
			nm.Status.CurrentMapping[localName] = mapsv1alpha1.RemoteNamespaceStatus{
				RemoteNamespace: remoteName,
				Phase:           mapsv1alpha1.MappingCreationLoopBackOff,
			}
			errorCondition = true
			continue
		}
		nm.Status.CurrentMapping[localName] = mapsv1alpha1.RemoteNamespaceStatus{
			RemoteNamespace: remoteName,
			Phase:           mapsv1alpha1.MappingAccepted,
		}
	}
	return errorCondition
}

// deleteRemoteNamespace deletes a remote Namespace from the remote cluster, the right client to use is chosen
// by NamespaceMap's cluster-id. This function return nil (success) only when the remote Namespace is really deleted,
// so the get Api returns a "NotFound".
func (r *NamespaceMapReconciler) deleteRemoteNamespace(ctx context.Context, remoteClusterID, remoteName string) error {
	if err := r.checkRemoteClientPresence(ctx, remoteClusterID); err != nil {
		return err
	}

	remoteNamespace := &corev1.Namespace{}
	if err := r.RemoteClients[remoteClusterID].Get(ctx, types.NamespacedName{Name: remoteName}, remoteNamespace); err != nil {
		if apierrors.IsNotFound(err) {
			klog.Infof("The namespace '%s' is correctly deleted from the remote cluster: '%s'", remoteName, remoteClusterID)
			return nil
		}
		klog.Errorf("unable to get the remote namespace '%s'", remoteName)
		return err
	}

	// Check if it is a namespace created by the NamespaceMap controller.
	annotationKey := fmt.Sprintf("%s-%s/%s", remoteNamespaceAnnotationPrefix, r.LocalClusterID, liqoSuffix)
	if value, ok := remoteNamespace.Annotations[annotationKey]; !ok || value != remoteNamespaceAnnotationValue {
		klog.Infof("No remote namespaces with name '%s' created through Liqo mechanism inside the remote cluster: '%s'",
			remoteName, remoteClusterID)
		return nil
	}

	if remoteNamespace.DeletionTimestamp.IsZero() {
		if err := r.RemoteClients[remoteClusterID].Delete(ctx, remoteNamespace); err != nil {
			klog.Errorf("unable to delete the namespace '%s' from the remote cluster: '%s'", remoteName, remoteClusterID)
			return err
		}
		klog.Infof("The deletion timestamp is correctly set on the namespace '%s'", remoteName)
	}
	return fmt.Errorf("the remote Namespace '%s' inside the cluster '%s' is undergoing graceful termination", remoteName, remoteClusterID)
}

// If DesiredMapping field has less entries than CurrentMapping, is necessary to remove some remote namespaces.
// This function checks if remote namespaces requested to be deleted are really deleted.
func (r *NamespaceMapReconciler) ensureRemoteNamespacesDeletion(ctx context.Context, nm *mapsv1alpha1.NamespaceMap) bool {
	errorCondition := false
	for localName, remoteStatus := range nm.Status.CurrentMapping {
		if _, ok := nm.Spec.DesiredMapping[localName]; ok && nm.DeletionTimestamp.IsZero() {
			continue
		}
		if err := r.deleteRemoteNamespace(ctx, nm.Labels[liqoconst.RemoteClusterID], remoteStatus.RemoteNamespace); err != nil {
			nm.Status.CurrentMapping[localName] = mapsv1alpha1.RemoteNamespaceStatus{
				RemoteNamespace: remoteStatus.RemoteNamespace,
				Phase:           mapsv1alpha1.MappingTerminating,
			}
			errorCondition = true
			continue
		}
		delete(nm.Status.CurrentMapping, localName)
	}
	return errorCondition
}

// This function checks if there are Namespaces that have to be created or deleted, in accordance with DesiredMapping
// field. It updates also NamespaceOffloading status in according to the remote Namespace Status.
func (r *NamespaceMapReconciler) ensureRemoteNamespaces(ctx context.Context, nm *mapsv1alpha1.NamespaceMap) error {
	errorCondition := false
	if nm.Status.CurrentMapping == nil {
		nm.Status.CurrentMapping = map[string]mapsv1alpha1.RemoteNamespaceStatus{}
	}
	// Object used as base for client.MergeFrom
	original := nm.DeepCopy()

	errorCondition = r.ensureRemoteNamespacesExistence(ctx, nm) || r.ensureRemoteNamespacesDeletion(ctx, nm)

	// MergeFrom used to avoid conflicts, the NamespaceMap controller has the ownership of NamespaceMap status
	if err := r.Patch(ctx, nm, client.MergeFrom(original)); err != nil {
		klog.Errorf("%s -> unable to update NamespaceMap '%s' Status", err, nm.Name)
		return err
	}
	klog.Infof("the status of the NamespaceMap '%s' is correctly updated", nm.Name)

	if errorCondition {
		return fmt.Errorf("something during remote namespaces management went wrong")
	}
	return nil
}

// The NamespaceMap is requested to be deleted so before removing the NamespaceMapControllerFinalizer finalizer
// is necessary to delete all remote namespaces associated with this NamespaceMap.
func (r *NamespaceMapReconciler) namespaceMapDeletionProcess(ctx context.Context, nm *mapsv1alpha1.NamespaceMap) error {
	original := nm.DeepCopy()
	// If the NamespaceMap Status is empty, it is possible to remove the finalizer.
	if !r.ensureRemoteNamespacesDeletion(ctx, nm) {
		return r.RemoveNamespaceMapControllerFinalizer(ctx, nm)
	}
	// This patch is used to update NamespaceMap status if some remote namespaces still exist.
	if err := r.Patch(ctx, nm, client.MergeFrom(original)); err != nil {
		klog.Errorf("%s -> unable to patch the status of the NamespaceMap %s", err, nm.Name)
		return err
	}
	return fmt.Errorf("remote namespaces deletion phase in progress")
}

// checkRemoteNamespacePrivileges checks that the right roleBindings are inside the remote namespace to understand if the
// virtual kubelet will have the right privileges on that namespace.
func checkRemoteNamespacePrivileges(ctx context.Context, cl client.Client, remoteNamespaceName, localClusterID string) error {
	roleBindingLabelValue := fmt.Sprintf("%s-%s", liqoconst.RoleBindingLabelValuePrefix, localClusterID)
	roleBindingList := &rbacv1.RoleBindingList{}
	if err := cl.List(ctx, roleBindingList, client.InNamespace(remoteNamespaceName),
		client.MatchingLabels{liqoconst.RoleBindingLabelKey: roleBindingLabelValue}); err != nil {
		klog.Errorf("%s -> unable to list roleBindings in the remote namespace '%s'", err, remoteNamespaceName)
		return err
	}
	if len(roleBindingList.Items) < 2 {
		err := fmt.Errorf("virtual kubelet will not have the necessary privileges on the remote namespace '%s'",
			remoteNamespaceName)
		klog.Error(err)
		return err
	}
	return nil
}
