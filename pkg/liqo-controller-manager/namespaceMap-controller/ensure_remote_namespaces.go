package namespacemapctrl

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"

	mapsv1alpha1 "github.com/liqotech/liqo/apis/virtualKubelet/v1alpha1"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
	liqoutils "github.com/liqotech/liqo/pkg/utils"
)

// This constants are used to compose the annotation that is inserted on remote Namespace at creation time.
const (
	liqoSuffix                      = "liqo.io"
	remoteNamespaceAnnotationPrefix = "remote-namespace"
	remoteNamespaceAnnotationValue  = "This Namespace has been created through Liqo offloading mechanism"
)

// This function gets the clusted-id of the local cluster.
func (r *NamespaceMapReconciler) checkLocalClusterID() error {
	if r.LocalClusterID == "" {
		clusterID, err := liqoutils.GetClusterID(r.Client)
		if err != nil || clusterID == "" {
			return err
		}
		r.LocalClusterID = clusterID
	}
	return nil
}

// This function creates a remote Namespace inside the remote cluster, if it doesn't exist yet.
// The right client to use is chosen by means of NamespaceMap's cluster-id.
func (r *NamespaceMapReconciler) createRemoteNamespace(clusterID, remoteName string) error {
	if err := r.checkRemoteClientPresence(clusterID); err != nil {
		return err
	}
	if err := r.checkLocalClusterID(); err != nil {
		return err
	}

	// This annotation is used to recognize the remote namespaces that have been created by this controller.
	annotationKey := fmt.Sprintf("%s-%s/%s", remoteNamespaceAnnotationPrefix, r.LocalClusterID, liqoSuffix)
	remoteNamespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: remoteName,
			Annotations: map[string]string{
				annotationKey: remoteNamespaceAnnotationValue,
			},
		},
	}

	if err := r.RemoteClients[clusterID].Create(context.TODO(), remoteNamespace); err != nil {
		if apierrors.IsAlreadyExists(err) {
			if errGet := r.RemoteClients[clusterID].Get(context.TODO(), types.NamespacedName{Name: remoteName}, remoteNamespace); errGet == nil {
				if value, ok := remoteNamespace.Annotations[annotationKey]; ok && value == remoteNamespaceAnnotationValue {
					klog.Infof("Namespace '%s' already created inside remote cluster: '%s'", remoteNamespace.Name, clusterID)
					return nil
				}
			}
		}
		klog.Error(err)
		return err
	}

	klog.Infof("Namespace '%s' created inside remote cluster: '%s'", remoteNamespace.Name, clusterID)
	return nil
}

// For every entry of DesiredMapping create remote Namespace if it has not already being created.
// This function tries to create all the remote namespaces requested in DesiredMapping (NamespaceMap->Spec->DesiredMapping).
func (r *NamespaceMapReconciler) ensureRemoteNamespacesExistence(nm *mapsv1alpha1.NamespaceMap) bool {
	errorCondition := false
	for localName, remoteName := range nm.Spec.DesiredMapping {
		if err := r.createRemoteNamespace(nm.Labels[liqoconst.RemoteClusterID], remoteName); err != nil {
			nm.Status.CurrentMapping[localName] = mapsv1alpha1.RemoteNamespaceStatus{
				RemoteNamespace: remoteName,
				Phase:           mapsv1alpha1.MappingCreationLoopBackOff,
			}
			errorCondition = true
			klog.Errorf("%s -> Namespace '%s' cannot be created inside cluster '%s'", err,
				localName, nm.Labels[liqoconst.RemoteClusterID])
			continue
		}
		nm.Status.CurrentMapping[localName] = mapsv1alpha1.RemoteNamespaceStatus{
			RemoteNamespace: remoteName,
			Phase:           mapsv1alpha1.MappingAccepted,
		}
	}
	return errorCondition
}

// This function deletes a remote Namespace from the remote cluster, the right client to use is chosen
// by NamespaceMap's cluster-id. This function return nil (success) only when the remote Namespace is really deleted,
// so the get Api returns a "NotFound".
func (r *NamespaceMapReconciler) deleteRemoteNamespace(clusterID, remoteName string) error {
	if err := r.checkRemoteClientPresence(clusterID); err != nil {
		return err
	}
	if err := r.checkLocalClusterID(); err != nil {
		return err
	}

	remoteNamespace := &corev1.Namespace{}
	if err := r.RemoteClients[clusterID].Get(context.TODO(), types.NamespacedName{Name: remoteName}, remoteNamespace); err != nil {
		if apierrors.IsNotFound(err) {
			klog.Infof("The namespace '%s' is correctly deleted from the remote cluster: '%s'", remoteName, clusterID)
			return nil
		}
		klog.Errorf("unable to get the remote namespace '%s'", remoteName)
		return err
	}

	// Check if it is a namespace created by the NamespaceMap controller.
	annotationKey := fmt.Sprintf("%s-%s/%s", remoteNamespaceAnnotationPrefix, r.LocalClusterID, liqoSuffix)
	if value, ok := remoteNamespace.Annotations[annotationKey]; !ok || value != remoteNamespaceAnnotationValue {
		klog.Infof("No remote namespaces with name '%s' created through Liqo mechanism inside the remote cluster: '%s'",
			remoteName, clusterID)
		return nil
	}

	if remoteNamespace.DeletionTimestamp.IsZero() {
		if err := r.RemoteClients[clusterID].Delete(context.TODO(), remoteNamespace); err != nil {
			klog.Errorf("unable to delete the namespace '%s' from the remote cluster: '%s'", remoteName, clusterID)
			return err
		}
		klog.Infof("The deletion timestamp is correctly set on the namespace '%s'", remoteName)
	}
	return fmt.Errorf("remote Namespace '%s' terminating", remoteName)
}

// If DesiredMapping field has less entries than CurrentMapping, is necessary to remove some remote namespaces.
// This function checks if remote namespaces requested to be deleted are really deleted.
func (r *NamespaceMapReconciler) ensureRemoteNamespacesDeletion(nm *mapsv1alpha1.NamespaceMap) bool {
	errorCondition := false
	for localName, remoteStatus := range nm.Status.CurrentMapping {
		if _, ok := nm.Spec.DesiredMapping[localName]; ok && nm.DeletionTimestamp.IsZero() {
			continue
		}
		if err := r.deleteRemoteNamespace(nm.Labels[liqoconst.RemoteClusterID], remoteStatus.RemoteNamespace); err != nil {
			nm.Status.CurrentMapping[localName] = mapsv1alpha1.RemoteNamespaceStatus{
				RemoteNamespace: remoteStatus.RemoteNamespace,
				Phase:           mapsv1alpha1.MappingTerminating,
			}
			errorCondition = true
			klog.Errorf("%s -> Namespace '%s' cannot be deleted from cluster '%s'", err,
				localName, nm.Labels[liqoconst.RemoteClusterID])
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
	patch := nm.DeepCopy()

	errorCondition = r.ensureRemoteNamespacesExistence(nm) || r.ensureRemoteNamespacesDeletion(nm)

	// MergeFrom used to avoid conflicts, the NamespaceMap controller has the ownership of NamespaceMap status
	if err := r.Patch(ctx, nm, client.MergeFrom(patch)); err != nil {
		klog.Errorf("%s -> unable to update NamespaceMap '%s' Status", err, nm.Name)
		return err
	}
	klog.Infof("Status of the NamespaceMap '%s' is correctly updated", nm.Name)

	if errorCondition {
		err := fmt.Errorf("something during remote namespaces management went wrong")
		klog.Error(err)
		return err
	}
	return nil
}

// The NamespaceMap is requested to be deleted so before removing the NamespaceMapControllerFinalizer finalizer
// is necessary to delete all remote namespaces associated with this NamespaceMap.
func (r *NamespaceMapReconciler) namespaceMapDeletionProcess(ctx context.Context,
	nm *mapsv1alpha1.NamespaceMap) error {
	patch := nm.DeepCopy()
	r.ensureRemoteNamespacesDeletion(nm)
	// If the NamespaceMap Status is empty, it is possible to remove the finalizer.
	if len(nm.Status.CurrentMapping) == 0 {
		if err := r.RemoveNamespaceMapControllerFinalizer(ctx, nm); err != nil {
			return err
		}
	}
	if err := r.Patch(ctx, nm, client.MergeFrom(patch)); err != nil {
		klog.Errorf("%s -> unable to patch the status of the NamespaceMap %s", err, nm.Name)
	}
	err := fmt.Errorf("remote namespaces deletion phase in progress")
	klog.Error(err)
	return err
}
