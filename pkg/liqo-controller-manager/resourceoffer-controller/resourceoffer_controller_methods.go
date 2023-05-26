// Copyright 2019-2023 The Liqo Authors
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

package resourceoffercontroller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	sharingv1alpha1 "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	virtualkubeletv1alpha1 "github.com/liqotech/liqo/apis/virtualkubelet/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/discovery"
	foreigncluster "github.com/liqotech/liqo/pkg/utils/foreignCluster"
)

// setControllerReference sets owner reference to the related ForeignCluster.
func (r *ResourceOfferReconciler) setControllerReference(
	ctx context.Context, resourceOffer *sharingv1alpha1.ResourceOffer) error {
	// get the foreign cluster by clusterID label
	foreignCluster, err := foreigncluster.GetForeignClusterByID(ctx, r.Client, resourceOffer.Spec.ClusterID)
	if err != nil {
		klog.Error(err)
		return err
	}

	// add controller reference, if it is not already set
	if err := controllerutil.SetControllerReference(foreignCluster, resourceOffer, r.Scheme); err != nil {
		klog.Error(err)
		return err
	}

	return nil
}

// setResourceOfferPhase checks if the resource request can be accepted and set its phase accordingly.
func (r *ResourceOfferReconciler) setResourceOfferPhase(resourceOffer *sharingv1alpha1.ResourceOffer) {
	// we want only to care about resource offers with a pending status
	if resourceOffer.Status.Phase != "" && resourceOffer.Status.Phase != sharingv1alpha1.ResourceOfferPending {
		return
	}

	if r.disableAutoAccept {
		resourceOffer.Status.Phase = sharingv1alpha1.ResourceOfferManualActionRequired
	} else {
		resourceOffer.Status.Phase = sharingv1alpha1.ResourceOfferAccepted
	}
}

// checkVirtualNode checks the existence of the VirtualNode related to the ResourceOffer
// and sets its status in the ResourceOffer accordingly.
func (r *ResourceOfferReconciler) checkVirtualNode(
	ctx context.Context, resourceOffer *sharingv1alpha1.ResourceOffer) error {
	virtualNodeStatus, err := r.getVirtualNodeStatus(ctx, resourceOffer)
	if err != nil {
		klog.Error(err)
		return err
	}

	if virtualNodeStatus == nil {
		resourceOffer.Status.VirtualKubeletStatus = sharingv1alpha1.VirtualKubeletStatusNone
	} else if resourceOffer.Status.VirtualKubeletStatus != sharingv1alpha1.VirtualKubeletStatusDeleting {
		// there is a virtual node and the phase is not deleting
		resourceOffer.Status.VirtualKubeletStatus = sharingv1alpha1.VirtualKubeletStatusCreated
	}
	return nil
}

func getVirtualNodeName(fc *discoveryv1alpha1.ForeignCluster,
	resourceOffer *sharingv1alpha1.ResourceOffer) string {
	switch {
	case resourceOffer.Spec.NodeName != "":
		return resourceOffer.Spec.NodeName
	case resourceOffer.Spec.NodeNamePrefix != "":
		return fmt.Sprintf("%s-%s", resourceOffer.Spec.NodeNamePrefix, fc.Spec.ClusterIdentity.ClusterName)
	default:
		return fmt.Sprintf("liqo-%s", fc.Spec.ClusterIdentity.ClusterName)
	}
}

func (r *ResourceOfferReconciler) getVirtualNodeMutator(fc *discoveryv1alpha1.ForeignCluster,
	resourceOffer *sharingv1alpha1.ResourceOffer,
	virtualNode *virtualkubeletv1alpha1.VirtualNode) controllerutil.MutateFn {
	remoteClusterIdentity := fc.Spec.ClusterIdentity
	return func() error {
		if virtualNode.ObjectMeta.Labels == nil {
			virtualNode.ObjectMeta.Labels = map[string]string{}
		}
		virtualNode.ObjectMeta.Labels[discovery.ClusterIDLabel] = resourceOffer.Spec.ClusterID
		virtualNode.ObjectMeta.Labels[consts.ResourceOfferNameLabel] = resourceOffer.Name

		kubeconfigSecretNamespacedName, err := r.identityReader.GetSecretNamespacedName(fc.Spec.ClusterIdentity, fc.Status.TenantNamespace.Local)
		if err != nil {
			return err
		}

		virtualNode.Spec.ClusterIdentity = &remoteClusterIdentity
		//virtualNode.Spec.CreateNode = true
		virtualNode.Spec.KubeconfigSecretRef = &corev1.LocalObjectReference{
			Name: kubeconfigSecretNamespacedName.Name,
			// TODO: set the namespace (?) or copy the secret in the local namespace (?)
		}
		virtualNode.Spec.Images = resourceOffer.Spec.Images
		virtualNode.Spec.ResourceQuota = resourceOffer.Spec.ResourceQuota
		virtualNode.Spec.Labels = resourceOffer.Spec.Labels
		virtualNode.Spec.StorageClasses = resourceOffer.Spec.StorageClasses
		return controllerutil.SetControllerReference(resourceOffer, virtualNode, r.Scheme)
	}
}

func (r *ResourceOfferReconciler) createVirtualNode(ctx context.Context,
	resourceOffer *sharingv1alpha1.ResourceOffer) error {
	namespace := resourceOffer.Namespace
	remoteCluster, err := foreigncluster.GetForeignClusterByID(ctx, r.Client, resourceOffer.Spec.ClusterID)
	if err != nil {
		return err
	}

	virtualNode := virtualkubeletv1alpha1.VirtualNode{
		ObjectMeta: metav1.ObjectMeta{
			Name:      getVirtualNodeName(remoteCluster, resourceOffer),
			Namespace: namespace,
		},
	}
	_, err = controllerutil.CreateOrUpdate(ctx, r.Client,
		&virtualNode, r.getVirtualNodeMutator(remoteCluster, resourceOffer, &virtualNode))
	return err
}

func (r *ResourceOfferReconciler) deleteVirtualNode(ctx context.Context,
	resourceOffer *sharingv1alpha1.ResourceOffer) error {
	namespace := resourceOffer.Namespace
	remoteCluster, err := foreigncluster.GetForeignClusterByID(ctx, r.Client, resourceOffer.Spec.ClusterID)
	if err != nil {
		return err
	}

	virtualNode := virtualkubeletv1alpha1.VirtualNode{
		ObjectMeta: metav1.ObjectMeta{
			Name:      getVirtualNodeName(remoteCluster, resourceOffer),
			Namespace: namespace,
		},
	}
	return client.IgnoreNotFound(r.Client.Delete(ctx, &virtualNode))
}

// getVirtualNodeStatus returns the VirtualNode status given a ResourceOffer.
func (r *ResourceOfferReconciler) getVirtualNodeStatus(
	ctx context.Context, resourceOffer *sharingv1alpha1.ResourceOffer) (*virtualkubeletv1alpha1.VirtualNodeStatus, error) {
	var virtualNodeList virtualkubeletv1alpha1.VirtualNodeList
	labels := map[string]string{
		discovery.ClusterIDLabel:      resourceOffer.Spec.ClusterID,
		consts.ResourceOfferNameLabel: resourceOffer.Name,
	}
	if err := r.Client.List(ctx, &virtualNodeList, client.MatchingLabels(labels)); err != nil {
		klog.Error(err)
		return nil, err
	}

	switch len(virtualNodeList.Items) {
	case 1:
		return &virtualNodeList.Items[0].Status, nil
	case 0:
		klog.V(4).Infof("[%v] no VirtualNode found for ResourceOffer %s",
			resourceOffer.Spec.ClusterID, resourceOffer.Name)
		return nil, nil
	default:
		err := fmt.Errorf("[%v] more than one VirtualNode found for ResourceOffer %s",
			resourceOffer.Spec.ClusterID, resourceOffer.Name)
		klog.Error(err)
		return nil, err
	}
}

type kubeletDeletePhase string

const (
	kubeletDeletePhaseNone         kubeletDeletePhase = "None"
	kubeletDeletePhaseDrainingNode kubeletDeletePhase = "DrainingNode"
	kubeletDeletePhaseNodeDeleted  kubeletDeletePhase = "NodeDeleted"
)

// getDeleteVirtualKubeletPhase returns the delete phase for the VirtualKubelet created basing on the
// given ResourceOffer.
func getDeleteVirtualKubeletPhase(resourceOffer *sharingv1alpha1.ResourceOffer) kubeletDeletePhase {
	notAccepted := !isAccepted(resourceOffer)
	deleting := !resourceOffer.DeletionTimestamp.IsZero()
	desiredDelete := !resourceOffer.Spec.WithdrawalTimestamp.IsZero()
	nodeDrained := !controllerutil.ContainsFinalizer(resourceOffer, consts.NodeFinalizer)

	// if the ResourceRequest has not been accepted by the local cluster,
	// or it has a DeletionTimestamp not equal to zero (the resource has been deleted),
	// or it has a WithdrawalTimestamp not equal to zero (the remote cluster asked for its graceful deletion),
	// the VirtualKubelet is in a terminating phase, otherwise return the None phase.
	if notAccepted || deleting || desiredDelete {
		// if the liqo.io/node finalizer is not set, the remote cluster has been drained and the node has been deleted,
		// we can then proceed with the VirtualKubelet deletion.
		if nodeDrained {
			return kubeletDeletePhaseNodeDeleted
		}

		// if the finalizer is still present, the node draining has not completed yet, we have to wait before to
		// continue the unpeering process.
		return kubeletDeletePhaseDrainingNode
	}
	return kubeletDeletePhaseNone
}

// isAccepted checks if a ResourceOffer is in Accepted phase.
func isAccepted(resourceOffer *sharingv1alpha1.ResourceOffer) bool {
	return resourceOffer.Status.Phase == sharingv1alpha1.ResourceOfferAccepted
}
