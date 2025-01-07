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

package storage

import (
	"context"
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ref "k8s.io/client-go/tools/reference"
	"k8s.io/klog/v2"
	"k8s.io/kubectl/pkg/scheme"
	"sigs.k8s.io/sig-storage-lib-external-provisioner/v7/controller"
	"sigs.k8s.io/sig-storage-lib-external-provisioner/v7/util"
)

const (
	annStorageProvisioner     = "volume.beta.kubernetes.io/storage-provisioner"
	annSelectedNode           = "volume.kubernetes.io/selected-node"
	annAlphaSelectedNode      = "volume.alpha.kubernetes.io/selected-node"
	annDynamicallyProvisioned = "pv.kubernetes.io/provisioned-by"
)

//
// the methods in this file are largely taken (and simplified in the parts not required for our use-case) from
// https://github.com/kubernetes-sigs/sig-storage-lib-external-provisioner/blob/v7.0.1/controller/controller.go
//

// shouldProvision checks if the PVC reflector has to reflect this volume claim or not.
// It checks if:
// - the volume name is not set
// - the provisioner name (in the annotation) is the same of the current provisioner
// - the storage class exists and it is the same which we are provisioning
// - the volume binding mode is WaitForFirstConsumer
// - the selected node is the one the reflector is managing.
func (npvcr *NamespacedPersistentVolumeClaimReflector) shouldProvision(claim *corev1.PersistentVolumeClaim) (bool, error) {
	if claim.Spec.VolumeName != "" {
		return false, nil
	}

	if provisioner, found := claim.Annotations[annStorageProvisioner]; found {
		if npvcr.knownProvisioner(provisioner) {
			claimClass := util.GetPersistentVolumeClaimClass(claim)
			if claimClass != npvcr.virtualStorageClassName {
				return false, nil
			}

			class, err := npvcr.classes.Get(claimClass)
			if err != nil {
				return false, err
			}

			if class.VolumeBindingMode != nil && *class.VolumeBindingMode == storagev1.VolumeBindingWaitForFirstConsumer {
				// When claim is in delay binding mode, annSelectedNode is
				// required to provision volume.
				// Though PV controller set annStorageProvisioner only when
				// annSelectedNode is set, but provisioner may remove
				// annSelectedNode to notify scheduler to reschedule again.
				if selectedNode, ok := claim.Annotations[annSelectedNode]; ok && selectedNode != "" {
					return true, nil
				}
				return false, nil
			}
			// do not provision if the binding mode is not WaitForFirstConsumer
			return false, nil
		}
	}

	return false, nil
}

func (npvcr *NamespacedPersistentVolumeClaimReflector) knownProvisioner(provisioner string) bool {
	return provisioner == npvcr.provisionerName
}

type provisionFunc func(context.Context, controller.ProvisionOptions) (*corev1.PersistentVolume, controller.ProvisioningState, error)

// provisionClaimOperation attempts to provision a volume for the given claim.
// Returns nil error only when the volume was provisioned (in which case it also returns ProvisioningFinished),
// a normal error when the volume was not provisioned and provisioning should be retried (requeue the claim),
// or the special errStopProvision when provisioning was impossible and no further attempts to provision should be tried.
func (npvcr *NamespacedPersistentVolumeClaimReflector) provisionClaimOperation(ctx context.Context,
	claim *corev1.PersistentVolumeClaim, provision provisionFunc) (controller.ProvisioningState, error) {
	// Most code here is identical to that found in controller.go of kube's PV controller...
	claimClass := util.GetPersistentVolumeClaimClass(claim)

	//  A previous doProvisionClaim may just have finished while we were waiting for
	//  the locks. Check that PV (with deterministic name) hasn't been provisioned
	//  yet.
	pvName := "pvc-" + string(claim.UID)
	_, err := npvcr.volumes.Get(pvName)
	if err == nil {
		// Volume has been already provisioned, nothing to do.
		klog.V(4).Infof("Persistentvolume %q already exists, skipping", pvName)
		return controller.ProvisioningFinished, nil
	}

	// Prepare a claimRef to the claim early (to fail before a volume is
	// provisioned)
	claimRef, err := ref.GetReference(scheme.Scheme, claim)
	if err != nil {
		klog.Errorf("Unexpected error getting claim reference (local claim %q): %v", npvcr.LocalRef(claim.GetName()), err)
		return controller.ProvisioningNoChange, err
	}

	// For any issues getting fields from StorageClass (including reclaimPolicy & mountOptions),
	// retry the claim because the storageClass can be fixed/(re)created independently of the claim
	class, err := npvcr.classes.Get(claimClass)
	if err != nil {
		klog.Errorf("Error getting StorageClass field of claim %q: %v", npvcr.LocalRef(claim.GetName()), err)
		return controller.ProvisioningFinished, err
	}
	if !npvcr.knownProvisioner(class.Provisioner) {
		// class.Provisioner has either changed since shouldProvision() or
		// annDynamicallyProvisioned contains different provisioner than
		// class.Provisioner.
		klog.Errorf("Unknown provisioner %q requested in StorageClass of claim %q", class.Provisioner, npvcr.LocalRef(claim.GetName()))
		return controller.ProvisioningFinished, nil
	}

	var selectedNode *corev1.Node
	// Get SelectedNode
	if nodeName, ok := getString(claim.Annotations, annSelectedNode, annAlphaSelectedNode); ok {
		selectedNode, err = npvcr.nodes.Get(nodeName)
		if err != nil {
			err = fmt.Errorf("failed to get target node: %w", err)
			npvcr.Event(claim, corev1.EventTypeWarning, "ProvisioningFailed", err.Error())
			return controller.ProvisioningNoChange, err
		}
	}

	options := controller.ProvisionOptions{
		StorageClass: class,
		PVName:       pvName,
		PVC:          claim,
		SelectedNode: selectedNode,
	}

	npvcr.Event(claim, corev1.EventTypeNormal, "Provisioning",
		fmt.Sprintf("External provisioner is provisioning volume for claim %q", npvcr.LocalRef(claim.GetName())))

	volume, result, err := provision(ctx, options)
	if err != nil {
		var ierr *controller.IgnoredError
		if ok := errors.As(err, &ierr); ok {
			// Provision ignored, do nothing and hope another provisioner will provision it.
			klog.V(4).Infof("Volume provision ignored: %v", ierr)
			return controller.ProvisioningFinished, nil
		}
		err = fmt.Errorf("failed to provision volume with StorageClass %q: %w", claimClass, err)
		npvcr.Event(claim, corev1.EventTypeWarning, "ProvisioningFailed", err.Error())
		return result, err
	}

	klog.Infof("Volume %q provisioned (local claim %q)", volume.Name, npvcr.LocalRef(claim.GetName()))

	// Set ClaimRef and the PV controller will bind and set annBoundByController for us
	volume.Spec.ClaimRef = claimRef

	metav1.SetMetaDataAnnotation(&volume.ObjectMeta, annDynamicallyProvisioned, class.Provisioner)
	volume.Spec.StorageClassName = claimClass

	_, err = npvcr.localPersistentVolumesClient.Create(ctx, volume, metav1.CreateOptions{})
	return controller.ProvisioningFinished, err
}

// getString reads a string from a map, using the key value or the alternatives if the
// first one is not set.
func getString(m map[string]string, key string, alts ...string) (string, bool) {
	if m == nil {
		return "", false
	}
	keys := append([]string{key}, alts...)
	for _, k := range keys {
		if v, ok := m[k]; ok {
			return v, true
		}
	}
	return "", false
}
