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

package move

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/liqotech/liqo/pkg/utils"
)

func isLocalVolume(ctx context.Context, cl client.Client, pvc *corev1.PersistentVolumeClaim) (bool, *corev1.Node, error) {
	if pvc.Annotations == nil {
		return false, nil, fmt.Errorf("pvc %s/%s has no annotations, cannot determine on which cluster is stored", pvc.Namespace, pvc.Name)
	}
	nodeName, found := pvc.Annotations["volume.kubernetes.io/selected-node"]
	if !found {
		return false, nil, fmt.Errorf("pvc %s/%s has no selected-node annotation, cannot determine on which cluster is stored", pvc.Namespace, pvc.Name)
	}

	node := corev1.Node{}
	if err := cl.Get(ctx, client.ObjectKey{Name: nodeName}, &node); err != nil {
		return false, nil, err
	}

	return !utils.IsVirtualNode(&node), &node, nil
}

func checkNoMounter(ctx context.Context, cl client.Client, pvc *corev1.PersistentVolumeClaim) error {
	var podList corev1.PodList
	if err := cl.List(ctx, &podList, client.InNamespace(pvc.Namespace)); err != nil {
		return err
	}

	for i := range podList.Items {
		pod := &podList.Items[i]
		for j := range pod.Spec.Volumes {
			volume := &pod.Spec.Volumes[j]
			if volume.PersistentVolumeClaim != nil && volume.PersistentVolumeClaim.ClaimName == pvc.Name {
				return fmt.Errorf("the volume (%s/%s) must not to be mounted by any pod, but found mounter pod %s/%s",
					pvc.Namespace, pvc.Name, pod.Namespace, pod.Name)
			}
		}
	}

	return nil
}

func recreatePvc(ctx context.Context, cl client.Client, oldPvc *corev1.PersistentVolumeClaim) (*corev1.PersistentVolumeClaim, error) {
	newPvc := corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      oldPvc.Name,
			Namespace: oldPvc.Namespace,
			Labels:    oldPvc.Labels,
		},
		Spec: *oldPvc.Spec.DeepCopy(),
	}
	newPvc.Spec.VolumeName = ""

	if err := cl.Delete(ctx, oldPvc); err != nil {
		return nil, err
	}

	if err := retry.OnError(
		wait.Backoff{
			Duration: 500 * time.Millisecond,
			Factor:   1.1,
			Steps:    100,
		},
		apierrors.IsAlreadyExists,
		func() error {
			return cl.Create(ctx, &newPvc)
		}); err != nil {
		return nil, err
	}

	return &newPvc, nil
}
