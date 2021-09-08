// Copyright 2019-2021 The Liqo Authors
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

package util

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	"github.com/liqotech/liqo/pkg/utils/pod"
	"github.com/liqotech/liqo/pkg/virtualKubelet"
)

// IsPodUp waits for a specific namespace/podName to be ready. It returns true if the pod within the timeout, false otherwise.
func IsPodUp(ctx context.Context, client kubernetes.Interface, namespace, podName string, isHomePod bool) bool {
	var podToCheck *corev1.Pod
	var err error
	var labelSelector = map[string]string{
		virtualKubelet.ReflectedpodKey: podName,
	}
	if isHomePod {
		klog.Infof("checking if local pod %s/%s is ready", namespace, podName)
		podToCheck, err = client.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			klog.Errorf("an error occurred while getting pod %s/%s: %v", namespace, podName, err)
			return false
		}
	} else {
		klog.Infof("checking if remote pod is ready")
		pods, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
			LabelSelector: labels.SelectorFromSet(labelSelector).String(),
		})
		if err != nil {
			klog.Errorf("an error occurred while getting remote pod: %v", err)
			return false
		}
		if len(pods.Items) == 0 {
			klog.Error("an error occurred: remote pod not found")
			return false
		}
		podToCheck = &pods.Items[0]
	}
	state := pod.IsPodReady(podToCheck)
	if isHomePod {
		klog.Infof("local pod %s/%s is ready", podToCheck.Namespace, podToCheck.Name)
	} else {
		klog.Infof("remote pod %s/%s is ready", podToCheck.Namespace, podToCheck.Name)
	}
	return state
}

// ArePodsUp check if all the pods of a specific namespace are ready. It returns a list of ready pods, a list of unready
// pods and occurred errors.
func ArePodsUp(ctx context.Context, clientset kubernetes.Interface, namespace string) (ready, notReady []string, retErr error) {
	pods, retErr := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if retErr != nil {
		klog.Error(retErr)
		return nil, nil, retErr
	}
	for index := range pods.Items {
		if !pod.IsPodReady(&pods.Items[index]) {
			notReady = append(notReady, pods.Items[index].Name)
		}
		ready = append(ready, pods.Items[index].Name)
	}
	return ready, notReady, nil
}
