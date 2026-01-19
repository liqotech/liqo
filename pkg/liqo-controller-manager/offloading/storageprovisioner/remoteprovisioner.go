// Copyright 2019-2026 The Liqo Authors
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
	"strings"

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

	if v, ok := virtualPvc.Annotations[consts.RemotePVCStorageClassAnnotKey]; ok && v != "" {
		remoteStorageClass = v
	}

	remoteAccessModes := virtualPvc.Spec.AccessModes
	if v, ok := virtualPvc.Annotations[consts.RemotePVCAccessModeAnnotKey]; ok && v != "" {
		var err error
		remoteAccessModes, err = parseAccessModes(v)
		if err != nil {
			return nil, controller.ProvisioningNoChange, err
		}
	}

	mutation := remotePersistentVolumeClaim(virtualPvc, remoteStorageClass, remoteNamespace, forgingOpts, remoteAccessModes)

	_, err = remotePvcClient.Apply(ctx, mutation, forge.ApplyOptions())
	if err != nil {
		return nil, controller.ProvisioningInBackground, err
	}

	nodeAffinity, err := buildPvNodeAffinity(options)
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
			NodeAffinity: nodeAffinity,
		},
	}

	if options.StorageClass.ReclaimPolicy != nil {
		pv.Spec.PersistentVolumeReclaimPolicy = *options.StorageClass.ReclaimPolicy
	}

	return pv, controller.ProvisioningFinished, nil
}

func buildPvNodeAffinity(options controller.ProvisionOptions) (*corev1.VolumeNodeAffinity, error) {
	nodeAffinitySelectorKey := consts.EdgeLocationName
	labels := options.SelectedNode.GetLabels()
	if labels == nil {
		return nil, fmt.Errorf("no labels found for node %s", options.SelectedNode.GetName())
	}
	nodeSelectorValue, ok := labels[nodeAffinitySelectorKey]
	if !ok {
		return nil,
			fmt.Errorf("liqo node selector %q not found on node %s", nodeAffinitySelectorKey, options.SelectedNode.GetName())
	}

	nodeAffinityOperator := corev1.NodeSelectorOpIn
	var nodeAffinityValues []string

	// Check whether pvc should be provisione on all the edges, in that case just filter by virtual node.
	shouldProvisionOnAllEdges := options.PVC.Annotations != nil &&
		options.PVC.Annotations[consts.ProvisionPVCOnAllEdgesAnnotationKey] == consts.ProvisionPVCOnAllEdgesAnnotationValue

	if shouldProvisionOnAllEdges {
		nodeAffinityOperator = corev1.NodeSelectorOpExists
	} else {
		nodeAffinityValues = []string{nodeSelectorValue}
	}

	res := &corev1.VolumeNodeAffinity{
		Required: &corev1.NodeSelector{
			NodeSelectorTerms: []corev1.NodeSelectorTerm{
				{
					MatchExpressions: []corev1.NodeSelectorRequirement{
						{
							Key:      nodeAffinitySelectorKey,
							Operator: nodeAffinityOperator,
							Values:   nodeAffinityValues,
						},
					},
				},
			},
		},
	}
	return res, nil
}

// remotePersistentVolumeClaim forges the apply patch for the reflected PersistentVolumeClaim, given the local one.
func remotePersistentVolumeClaim(virtualPvc *corev1.PersistentVolumeClaim,
	storageClass, namespace string, forgingOpts *forge.ForgingOpts,
	accessModes []corev1.PersistentVolumeAccessMode) *v1apply.PersistentVolumeClaimApplyConfiguration {
	return v1apply.PersistentVolumeClaim(virtualPvc.Name, namespace).
		WithLabels(forge.FilterNotReflected(virtualPvc.GetLabels(), forgingOpts.LabelsNotReflected)).
		WithLabels(forge.ReflectionLabels()).
		WithAnnotations(forge.FilterNotReflected(filterAnnotations(virtualPvc.GetAnnotations()), forgingOpts.AnnotationsNotReflected)).
		WithSpec(remotePersistentVolumeClaimSpec(virtualPvc, storageClass, accessModes))
}

func remotePersistentVolumeClaimSpec(virtualPvc *corev1.PersistentVolumeClaim,
	storageClass string, accessModes []corev1.PersistentVolumeAccessMode) *v1apply.PersistentVolumeClaimSpecApplyConfiguration {
	res := v1apply.PersistentVolumeClaimSpec().
		WithAccessModes(accessModes...).
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

var knownAccessModes = map[corev1.PersistentVolumeAccessMode]struct{}{
	corev1.ReadWriteOnce:    {},
	corev1.ReadOnlyMany:     {},
	corev1.ReadWriteMany:    {},
	corev1.ReadWriteOncePod: {},
}

// parseAccessModes parses a comma-separated list of Kubernetes access mode strings.
// Returns an error if any value is not a known PersistentVolumeAccessMode.
func parseAccessModes(s string) ([]corev1.PersistentVolumeAccessMode, error) {
	parts := strings.Split(s, ",")
	modes := make([]corev1.PersistentVolumeAccessMode, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		mode := corev1.PersistentVolumeAccessMode(p)
		if _, ok := knownAccessModes[mode]; !ok {
			return nil, fmt.Errorf("unknown PersistentVolumeAccessMode %q", p)
		}
		modes = append(modes, mode)
	}
	return modes, nil
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
