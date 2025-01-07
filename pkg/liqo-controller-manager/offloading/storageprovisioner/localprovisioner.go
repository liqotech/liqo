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

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/sig-storage-lib-external-provisioner/v7/controller"

	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/utils"
	"github.com/liqotech/liqo/pkg/utils/resource"
)

func (p *liqoLocalStorageProvisioner) provisionLocalPVC(ctx context.Context,
	options controller.ProvisionOptions) (*v1.PersistentVolume, controller.ProvisioningState, error) {
	virtualPvc := options.PVC
	realPvcName := virtualPvc.GetUID()
	realPvc := v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      string(realPvcName),
			Namespace: p.storageNamespace,
		},
	}

	if operation, err := resource.CreateOrUpdate(ctx, p.client, &realPvc, func() error {
		return p.mutateLocalRealPVC(virtualPvc, &realPvc, options.SelectedNode)
	}); err != nil {
		return nil, controller.ProvisioningInBackground, err
	} else if operation != controllerutil.OperationResultNone {
		return nil, controller.ProvisioningInBackground, fmt.Errorf("provisioning real PVC")
	}

	if realPvc.Spec.VolumeName == "" {
		return nil, controller.ProvisioningInBackground, fmt.Errorf("real PV not provided yet")
	}

	var realPv v1.PersistentVolume
	if err := p.client.Get(ctx, types.NamespacedName{
		Name: realPvc.Spec.VolumeName,
	}, &realPv); err != nil {
		return nil, controller.ProvisioningInBackground, err
	}

	pv := &v1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: options.PVName,
		},
		Spec: v1.PersistentVolumeSpec{
			StorageClassName: options.StorageClass.Name,
			AccessModes:      options.PVC.Spec.AccessModes,
			Capacity: v1.ResourceList{
				v1.ResourceStorage: options.PVC.Spec.Resources.Requests[v1.ResourceStorage],
			},
			PersistentVolumeSource: realPv.Spec.PersistentVolumeSource,
			NodeAffinity: &v1.VolumeNodeAffinity{
				Required: &v1.NodeSelector{
					NodeSelectorTerms: []v1.NodeSelectorTerm{
						{
							MatchExpressions: []v1.NodeSelectorRequirement{
								{
									Key:      consts.TypeLabel,
									Operator: v1.NodeSelectorOpDoesNotExist,
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

	pv.Spec.NodeAffinity = mergeAffinities(&pv.Spec, &realPv.Spec)

	return pv, controller.ProvisioningFinished, nil
}

// mutateLocalRealPVC enforces the local realPVC fields merging its old values (if any)
// with the ones coming from the virtualPVC.
// i.e. the PVC spec is the copy of the virtual one, but the storage class is the one set in the
// storage provisioner or the one previously set in the PVC (since it is a read-only field). The
// real volumeName is preserved too.
func (p *liqoLocalStorageProvisioner) mutateLocalRealPVC(virtualPvc, realPvc *v1.PersistentVolumeClaim,
	selectedNode *v1.Node) error {
	if realPvc.ObjectMeta.Annotations == nil {
		realPvc.ObjectMeta.Annotations = map[string]string{}
	}
	// required if the real storage class is wait for first consumer
	realPvc.ObjectMeta.Annotations["volume.kubernetes.io/selected-node"] = selectedNode.Name
	realPvc.ObjectMeta.Annotations["volume.alpha.kubernetes.io/selected-node"] = selectedNode.Name

	if realPvc.ObjectMeta.Labels == nil {
		realPvc.ObjectMeta.Labels = map[string]string{}
	}
	realPvc.ObjectMeta.Labels[consts.VirtualPvcNamespaceLabel] = virtualPvc.GetNamespace()
	realPvc.ObjectMeta.Labels[consts.VirtualPvcNameLabel] = virtualPvc.GetName()

	storageClassName := realPvc.Spec.StorageClassName
	if p.localRealStorageClass != "" {
		storageClassName = &p.localRealStorageClass
	}

	realPvName := realPvc.Spec.VolumeName
	realPvc.Spec = *virtualPvc.Spec.DeepCopy()
	realPvc.Spec.VolumeName = realPvName
	realPvc.Spec.StorageClassName = storageClassName

	return nil
}

func mergeAffinities(vol1, vol2 *v1.PersistentVolumeSpec) *v1.VolumeNodeAffinity {
	if emptyVolumeNodeAffinity(vol1) {
		return vol2.NodeAffinity.DeepCopy()
	}
	if emptyVolumeNodeAffinity(vol2) {
		return vol1.NodeAffinity.DeepCopy()
	}

	selector := utils.MergeNodeSelector(vol1.NodeAffinity.Required, vol2.NodeAffinity.Required)
	return &v1.VolumeNodeAffinity{
		Required: &selector,
	}
}

func emptyVolumeNodeAffinity(vol *v1.PersistentVolumeSpec) bool {
	return vol.NodeAffinity == nil ||
		vol.NodeAffinity.Required.NodeSelectorTerms == nil ||
		len(vol.NodeAffinity.Required.NodeSelectorTerms) == 0
}
