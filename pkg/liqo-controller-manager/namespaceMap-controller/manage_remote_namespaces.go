package namespaceMap_controller

import (
	"context"
	"fmt"
	mapsv1alpha1 "github.com/liqotech/liqo/apis/virtualKubelet/v1alpha1"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"
)

// This function creates a new Namespace onto the remote cluster, the right client to use is chosen
// by NamespaceMap's cluster-id
func (r *NamespaceMapReconciler) createRemoteNamespace(clusterId string, remoteName string) error {

	if err := r.checkRemoteClientPresence(clusterId); err != nil {
		return err
	}

	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: remoteName,
		},
	}

	if err := r.RemoteClients[clusterId].Create(context.TODO(), namespace); err != nil {
		klog.Errorf("unable to create namespace '%s' onto remote cluster: '%s'", namespace.Name, clusterId)
		return err
	}

	klog.Infof("Namespace '%s' created onto remote cluster: '%s'", namespace.Name, clusterId)
	return nil
}

// This function  Namespace onto the remote cluster, the right client to use is chosen
// by NamespaceMap's cluster-id
func (r *NamespaceMapReconciler) deleteRemoteNamespace(clusterId string, remoteName string) error {

	if err := r.checkRemoteClientPresence(clusterId); err != nil {
		return err
	}

	remoteNamespace := &corev1.Namespace{}
	if err := r.RemoteClients[clusterId].Get(context.TODO(), types.NamespacedName{Name: remoteName}, remoteNamespace); err != nil {
		klog.Errorf("unable to get remote namespace '%s'", remoteName)
		return err
	}

	if err := r.RemoteClients[clusterId].Delete(context.TODO(), remoteNamespace); err != nil {
		klog.Errorf("unable to delete namespace '%s' onto remote cluster: '%s'", remoteName, clusterId)
		return err
	}

	klog.Infof("Namespace '%s' correctly deleted onto remote cluster: '%s'", remoteName, clusterId)
	return nil
}

// This function checks if there are Namespaces that have to be created or deleted, in accordance with DesiredMapping
// and CurrentMapping fields
func (r *NamespaceMapReconciler) manageRemoteNamespaces(nm *mapsv1alpha1.NamespaceMap) error {

	if nm.Status.CurrentMapping == nil {
		nm.Status.CurrentMapping = map[string]mapsv1alpha1.RemoteNamespaceStatus{}
	}

	// if DesiredMapping field has more entries than CurrentMapping, is necessary to create new remote namespaces
	for localName, remoteName := range nm.Spec.DesiredMapping {
		if remoteStatus, ok := nm.Status.CurrentMapping[localName]; !ok || (ok && remoteStatus.Phase != mapsv1alpha1.MappingAccepted) {
			namespacePhase := mapsv1alpha1.MappingAccepted
			if err := r.createRemoteNamespace(nm.Labels[liqoconst.VirtualNodeClusterId], remoteName); err != nil {
				namespacePhase = mapsv1alpha1.MappingRefused
			}
			nm.Status.CurrentMapping[localName] = mapsv1alpha1.RemoteNamespaceStatus{
				RemoteNamespace: remoteName,
				Phase:           namespacePhase,
			}
			if errUpdate := r.Update(context.TODO(), nm); errUpdate != nil {
				klog.Errorf("unable to update NamespaceMap '%s' Status", nm.Name)
				return errUpdate
			}
			klog.Infof("Status of Namespace '%s' is correctly updated", localName)
			if namespacePhase != mapsv1alpha1.MappingAccepted {
				return fmt.Errorf("namespace '%s' cannot be created on cluster '%s'", localName, nm.Labels[liqoconst.VirtualNodeClusterId])
			}
		}
	}

	// if DesiredMapping field has less entries than CurrentMapping, is necessary to remove some remote namespaces
	for localName, remoteStatus := range nm.Status.CurrentMapping {
		if _, ok := nm.Spec.DesiredMapping[localName]; !ok {
			if err := r.deleteRemoteNamespace(nm.Labels[liqoconst.VirtualNodeClusterId], remoteStatus.RemoteNamespace); err != nil {
				return err
			}
			// Update Map status
			delete(nm.Status.CurrentMapping, localName)
			if err := r.Update(context.TODO(), nm); err != nil {
				klog.Errorf("unable to update NamespaceMap '%s' Status", nm.Name)
				return err
			}
			klog.Infof("Status of NamespaceMap '%s' is correctly updated, delete remote Namespace '%s'", nm.Name, remoteStatus.RemoteNamespace)
		}
	}

	return nil
}
