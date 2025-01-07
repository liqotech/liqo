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

package virtualnodectrl

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/utils/indexer"
)

const waitForPodTerminationCheckPeriod = 10 * time.Second

// drainNode drains the controlled node using the Eviction API. All the
// PodDisruptionBudget policies set in the home cluster will be respected.
// The implementation is inspired (even if very simplified) by the kubectl
// implementation (https://github.com/kubernetes/kubectl/blob/v0.21.2/pkg/drain/drain.go).
func drainNode(ctx context.Context, cl client.Client, vn *offloadingv1beta1.VirtualNode) error {
	podsToEvict, err := getPodsForDeletion(ctx, cl, vn)
	if err != nil {
		klog.Error(err)
		return err
	}

	if err = evictPods(ctx, cl, podsToEvict); err != nil {
		klog.Error(err)
		return err
	}

	return nil
}

// getPodsForDeletion lists the pods that are running on our virtual node.
func getPodsForDeletion(ctx context.Context, cl client.Client, vn *offloadingv1beta1.VirtualNode) (*corev1.PodList, error) {
	podList := &corev1.PodList{}
	err := cl.List(ctx, podList, &client.ListOptions{
		FieldSelector: client.MatchingFieldsSelector{
			Selector: fields.OneTermEqualSelector(indexer.FieldNodeNameFromPod, vn.Name),
		},
		LabelSelector: client.MatchingLabelsSelector{
			Selector: labels.SelectorFromSet(map[string]string{
				consts.LocalPodLabelKey: consts.LocalPodLabelValue,
			}),
		},
	})
	klog.Infof("Drain node %s -> %d pods found", vn.Name, len(podList.Items))
	if err != nil {
		return nil, err
	}
	for i := range podList.Items {
		klog.V(4).Infof("Drain node %s -> pod %v/%v found", podList.Items[i].Spec.NodeName, podList.Items[i].Namespace, podList.Items[i].Name)
	}
	return podList, nil
}

// evictPods performs the eviction of the provided list of pods in parallel, waiting for their deletion.
func evictPods(ctx context.Context, cl client.Client, podList *corev1.PodList) error {
	for i := range podList.Items {
		if err := evictPod(ctx, cl, &podList.Items[i]); err != nil {
			return err
		}
	}

	for i := range podList.Items {
		if err := waitPodForDelete(ctx, cl, &podList.Items[i]); err != nil {
			return err
		}
	}

	return nil
}

// evictPod evicts the provided pod and waits for its deletion.
func evictPod(ctx context.Context, cl client.Client, pod *corev1.Pod) error {
	eviction := &policyv1.Eviction{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pod.Name,
			Namespace: pod.Namespace,
		},
		DeleteOptions: &metav1.DeleteOptions{},
	}

	if err := cl.SubResource("eviction").Create(ctx, pod, eviction); err != nil {
		return err
	}

	klog.V(4).Infof("Drain node %s -> pod %v/%v eviction started", pod.Spec.NodeName, pod.Namespace, pod.Name)

	return nil
}

// waitForDelete waits for the pod deletion.
func waitPodForDelete(ctx context.Context, cl client.Client, pod *corev1.Pod) error {
	//nolint:staticcheck // Waiting for PollWithContextCancel implementation.
	return wait.PollImmediateInfinite(waitForPodTerminationCheckPeriod, func() (bool, error) {
		klog.Infof("Drain node %s -> pod %v/%v waiting for deletion", pod.Spec.NodeName, pod.Namespace, pod.Name)
		updatedPod := &corev1.Pod{}
		err := cl.Get(ctx, client.ObjectKey{Namespace: pod.Namespace, Name: pod.Name}, updatedPod)
		if kerrors.IsNotFound(err) || pod.ObjectMeta.UID != updatedPod.ObjectMeta.UID {
			klog.Infof("Drain node %s -> pod %v/%v successfully deleted", pod.Spec.NodeName, pod.Namespace, pod.Name)
			return true, nil
		}
		if err != nil {
			klog.Error(err)
			return false, err
		}
		return false, nil
	})
}
