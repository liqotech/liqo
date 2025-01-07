// Copyright 2019-2025 The Liqo Authors
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

package storageprovisioner

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1apply "k8s.io/client-go/applyconfigurations/core/v1"
	corev1clients "k8s.io/client-go/kubernetes/typed/core/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"sigs.k8s.io/sig-storage-lib-external-provisioner/v7/controller"

	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/utils/maps"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
)

// ProvisionRemotePVC ensures the existence of a remote PVC and returns a virtual PV for that remote storage device.
func ProvisionRemotePVC(ctx context.Context,
	options controller.ProvisionOptions,
	remoteNamespace, remoteRealStorageClass string,
	remotePvcLister corev1listers.PersistentVolumeClaimNamespaceLister,
	remotePvcClient corev1clients.PersistentVolumeClaimInterface,
	forgingOpts *forge.ForgingOpts) (*corev1.PersistentVolume, controller.ProvisioningState, error) {
	virtualPvc := options.PVC

	labels := options.SelectedNode.GetLabels()
	if labels == nil {
		return nil, controller.ProvisioningInBackground, fmt.Errorf("no labels found for node %s", options.SelectedNode.GetName())
	}
	remoteClusterID, ok := labels[consts.RemoteClusterID]
	if !ok {
		return nil, controller.ProvisioningInBackground, fmt.Errorf("no remote cluster ID found for node %s", options.SelectedNode.GetName())
	}

	// get the storage class for the remote PVC,
	// use its class if denied, otherwise use the default one
	var remoteStorageClass string
	remotePvc, err := remotePvcLister.Get(virtualPvc.Name)
	switch {
	case apierrors.IsNotFound(err):
		remoteStorageClass = remoteRealStorageClass
	case err != nil && !apierrors.IsNotFound(err):
		return nil, controller.ProvisioningInBackground, err
	case remotePvc.Spec.StorageClassName != nil:
		remoteStorageClass = *remotePvc.Spec.StorageClassName
	default:
	}

	mutation := remotePersistentVolumeClaim(virtualPvc, remoteStorageClass, remoteNamespace, forgingOpts)
	_, err = remotePvcClient.Apply(ctx, mutation, forge.ApplyOptions())
	if err != nil {
		return nil, controller.ProvisioningInBackground, err
	}

	pv := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: options.PVName,
		},
		Spec: corev1.PersistentVolumeSpec{
			StorageClassName: options.StorageClass.Name,
			AccessModes:      options.PVC.Spec.AccessModes,
			Capacity: corev1.ResourceList{
				corev1.ResourceStorage: options.PVC.Spec.Resources.Requests[corev1.ResourceStorage],
			},
			PersistentVolumeSource: corev1.PersistentVolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/tmp/liqo-placeholder",
				},
			},
			NodeAffinity: &corev1.VolumeNodeAffinity{
				Required: &corev1.NodeSelector{
					NodeSelectorTerms: []corev1.NodeSelectorTerm{
						{
							MatchExpressions: []corev1.NodeSelectorRequirement{
								{
									Key:      consts.RemoteClusterID,
									Operator: corev1.NodeSelectorOpIn,
									Values:   []string{remoteClusterID},
								},
							},
						},
					},
				},
			},
		},
	}

	if options.StorageClass.ReclaimPolicy != nil {
		pv.Spec.PersistentVolumeReclaimPolicy = *options.StorageClass.ReclaimPolicy
	}

	return pv, controller.ProvisioningFinished, nil
}

// remotePersistentVolumeClaim forges the apply patch for the reflected PersistentVolumeClaim, given the local one.
func remotePersistentVolumeClaim(virtualPvc *corev1.PersistentVolumeClaim,
	storageClass, namespace string, forgingOpts *forge.ForgingOpts) *v1apply.PersistentVolumeClaimApplyConfiguration {
	return v1apply.PersistentVolumeClaim(virtualPvc.Name, namespace).
		WithLabels(forge.FilterNotReflected(virtualPvc.GetLabels(), forgingOpts.LabelsNotReflected)).
		WithLabels(forge.ReflectionLabels()).
		WithAnnotations(forge.FilterNotReflected(filterAnnotations(virtualPvc.GetAnnotations()), forgingOpts.AnnotationsNotReflected)).
		WithSpec(remotePersistentVolumeClaimSpec(virtualPvc, storageClass))
}

func remotePersistentVolumeClaimSpec(virtualPvc *corev1.PersistentVolumeClaim,
	storageClass string) *v1apply.PersistentVolumeClaimSpecApplyConfiguration {
	res := v1apply.PersistentVolumeClaimSpec().
		WithAccessModes(virtualPvc.Spec.AccessModes...).
		WithVolumeMode(func() corev1.PersistentVolumeMode {
			if virtualPvc.Spec.VolumeMode != nil {
				return *virtualPvc.Spec.VolumeMode
			}
			return corev1.PersistentVolumeFilesystem
		}()).
		WithResources(persistentVolumeClaimResources(virtualPvc.Spec.Resources))

	if storageClass != "" {
		res.WithStorageClassName(storageClass)
	}

	return res
}

func persistentVolumeClaimResources(resources corev1.VolumeResourceRequirements) *v1apply.VolumeResourceRequirementsApplyConfiguration {
	return v1apply.VolumeResourceRequirements().
		WithLimits(resources.Limits).
		WithRequests(resources.Requests)
}

var controllerAnnotations = []string{
	"pv.kubernetes.io/bind-completed",
	"pv.kubernetes.io/bound-by-controller",
	"volume.beta.kubernetes.io/storage-provisioner",
	"volume.kubernetes.io/storage-provisioner",
	"volume.kubernetes.io/selected-node",
	corev1.BetaStorageClassAnnotation,
}

func filterAnnotations(annotations map[string]string) map[string]string {
	return maps.Filter(annotations, maps.FilterBlacklist(controllerAnnotations...))
}
