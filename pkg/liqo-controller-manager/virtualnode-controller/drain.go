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

package virtualnodectrl

import (
	"context"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrl "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	virtualkubeletv1alpha1 "github.com/liqotech/liqo/apis/virtualkubelet/v1alpha1"
	"github.com/liqotech/liqo/pkg/virtualKubelet"
)

const waitForPodTerminationCheckPeriod = 10 * time.Second

// drainNode drains the controlled node using the Eviction API. All the
// PodDisruptionBudget policies set in the home cluster will be respected.
// The implementation is inspired (even if very simplified) by the kubectl
// implementation (https://github.com/kubernetes/kubectl/blob/v0.21.2/pkg/drain/drain.go).
func drainNode(ctx context.Context, cl client.Client, vn *virtualkubeletv1alpha1.VirtualNode) error {
	podsToEvict, err := getPodsForDeletion(ctx, cl, vn)
	if err != nil {
		klog.Error(err)
		return err
	}

	if err = evictPods(ctx, cl, podsToEvict); err != nil {
		klog.Error(err)
		return err
	}

	klog.Infof("node %v successfully drained", virtualKubelet.VirtualNodeName(vn))
	return nil
}

// getPodsForDeletion lists the pods that are running on our virtual node.
func getPodsForDeletion(ctx context.Context, cl client.Client, vn *virtualkubeletv1alpha1.VirtualNode) (*corev1.PodList, error) {
	podList := &corev1.PodList{}
	err := cl.List(ctx, podList, &client.ListOptions{
		FieldSelector: fields.SelectorFromSet(fields.Set{
			"spec.nodeName": virtualKubelet.VirtualNodeName(vn),
		}),
	})
	if err != nil {
		return nil, err
	}
	return podList, nil
}

// evictPods performs the eviction of the provided list of pods in parallel, waiting for their deletion.
func evictPods(ctx context.Context, cl client.Client, podList *corev1.PodList) error {
	var wg sync.WaitGroup
	errors := make(chan error, len(podList.Items))
	defer close(errors)

	for i := range podList.Items {
		wg.Add(1)
		go evictPod(ctx, cl, &podList.Items[i], &wg, errors)
	}

	wg.Wait()

	// if some of the evictions returned an error print it
	select {
	case err := <-errors:
		klog.Error(err)
		return err
	default:
		return nil
	}
}

// evictPod evicts the provided pod and waits for its deletion.
func evictPod(ctx context.Context, cl client.Client, pod *corev1.Pod, wg *sync.WaitGroup, errors chan error) {
	defer wg.Done()

	eviction := &policyv1beta1.Eviction{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pod.Name,
			Namespace: pod.Namespace,
		},
		DeleteOptions: &metav1.DeleteOptions{},
	}

	_, err := ctrl.CreateOrUpdate(ctx, cl, eviction, func() error { return nil })
	if err != nil {
		klog.Error(err)
		errors <- err
		return
	}

	if err := waitForDelete(ctx, cl, pod); err != nil {
		klog.Error(err)
		errors <- err
		return
	}
}

// waitForDelete waits for the pod deletion.
func waitForDelete(ctx context.Context, cl client.Client, pod *corev1.Pod) error {
	return wait.PollImmediateInfinite(waitForPodTerminationCheckPeriod, func() (bool, error) {
		updatedPod := &corev1.Pod{}
		err := cl.Get(ctx, client.ObjectKey{Namespace: pod.Namespace, Name: pod.Name}, updatedPod)
		if kerrors.IsNotFound(err) || (updatedPod != nil &&
			updatedPod.ObjectMeta.UID != updatedPod.ObjectMeta.UID) {
			return true, nil
		}
		if err != nil {
			klog.Error(err)
			return false, err
		}
		return false, nil
	})
}
