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

package utils

import (
	"context"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// RestartDeployment restarts the provided deployment by evicting all its pods.
func RestartDeployment(ctx context.Context, cl client.Client, deploy *appsv1.Deployment) error {
	var podList corev1.PodList
	if err := cl.List(ctx, &podList, client.InNamespace(deploy.Namespace),
		client.MatchingLabels(deploy.Spec.Selector.MatchLabels)); err != nil {
		return err
	}

	return EvictPods(ctx, cl, &podList, 3*time.Second)
}

// EvictPods performs the eviction of the provided list of pods in parallel, waiting for their deletion.
func EvictPods(ctx context.Context, cl client.Client, podList *corev1.PodList, checkPeriod time.Duration) error {
	for i := range podList.Items {
		if err := EvictPod(ctx, cl, &podList.Items[i]); err != nil {
			return err
		}
	}

	for i := range podList.Items {
		if err := WaitPodForDelete(ctx, cl, &podList.Items[i], checkPeriod); err != nil {
			return err
		}
	}

	return nil
}

// EvictPod evicts the provided pod and waits for its deletion.
func EvictPod(ctx context.Context, cl client.Client, pod *corev1.Pod) error {
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

// WaitPodForDelete waits for the pod deletion.
func WaitPodForDelete(ctx context.Context, cl client.Client, pod *corev1.Pod, checkPeriod time.Duration) error {
	return wait.PollUntilContextCancel(ctx, checkPeriod, true, func(ctx context.Context) (bool, error) {
		klog.Infof("Drain node %s -> pod %v/%v waiting for deletion", pod.Spec.NodeName, pod.Namespace, pod.Name)
		updatedPod := &corev1.Pod{}
		err := cl.Get(ctx, client.ObjectKey{Namespace: pod.Namespace, Name: pod.Name}, updatedPod)
		if apierrors.IsNotFound(err) || (updatedPod != nil &&
			pod.ObjectMeta.UID != updatedPod.ObjectMeta.UID) {
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
