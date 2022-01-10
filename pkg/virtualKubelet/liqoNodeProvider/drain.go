// Copyright 2019-2022 The Liqo Authors
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

package liqonodeprovider

import (
	"context"
	"sync"
	"time"

	v1 "k8s.io/api/core/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
)

const waitForPodTerminationCheckPeriod = 10 * time.Second

// drainNode drains the controlled node using the Eviction API. All the
// PodDisruptionBudget policies set in the home cluster will be respected.
// The implementation is inspired (even if very simplified) by the kubectl
// implementation (https://github.com/kubernetes/kubectl/blob/v0.21.2/pkg/drain/drain.go).
func (p *LiqoNodeProvider) drainNode(ctx context.Context) error {
	podsToEvict, err := p.getPodsForDeletion(ctx)
	if err != nil {
		klog.Error(err)
		return err
	}

	if err = p.evictPods(ctx, podsToEvict); err != nil {
		klog.Error(err)
		return err
	}

	klog.Infof("node %v successfully drained", p.node.GetName())
	return nil
}

// getPodsForDeletion lists the pods that are running on our virtual node.
func (p *LiqoNodeProvider) getPodsForDeletion(ctx context.Context) (*v1.PodList, error) {
	podList, err := p.localClient.CoreV1().Pods(metav1.NamespaceAll).List(ctx, metav1.ListOptions{
		FieldSelector: fields.SelectorFromSet(fields.Set{
			"spec.nodeName": p.node.GetName(),
		}).String()})
	if err != nil {
		return nil, err
	}
	return podList, nil
}

// evictPods performs the eviction of the provided list of pods in parallel, waiting for their deletion.
func (p *LiqoNodeProvider) evictPods(ctx context.Context, podList *v1.PodList) error {
	var wg sync.WaitGroup
	errors := make(chan error, len(podList.Items))
	defer close(errors)

	for i := range podList.Items {
		wg.Add(1)
		go p.evictPod(ctx, &podList.Items[i], &wg, errors)
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
func (p *LiqoNodeProvider) evictPod(ctx context.Context, pod *v1.Pod, wg *sync.WaitGroup, errors chan error) {
	defer wg.Done()

	eviction := &policyv1beta1.Eviction{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pod.Name,
			Namespace: pod.Namespace,
		},
		DeleteOptions: &metav1.DeleteOptions{},
	}

	if err := p.localClient.PolicyV1beta1().Evictions(pod.Namespace).Evict(ctx, eviction); err != nil {
		klog.Error(err)
		errors <- err
		return
	}

	if err := p.waitForDelete(ctx, pod); err != nil {
		klog.Error(err)
		errors <- err
		return
	}
}

// waitForDelete waits for the pod deletion.
func (p *LiqoNodeProvider) waitForDelete(ctx context.Context, pod *v1.Pod) error {
	return wait.PollImmediateInfinite(waitForPodTerminationCheckPeriod, func() (bool, error) {
		retPod, err := p.localClient.CoreV1().Pods(pod.Namespace).Get(ctx, pod.Name, metav1.GetOptions{})
		if kerrors.IsNotFound(err) || (retPod != nil && retPod.ObjectMeta.UID != pod.ObjectMeta.UID) {
			return true, nil
		}
		if err != nil {
			klog.Error(err)
			return false, err
		}
		return false, nil
	})
}
