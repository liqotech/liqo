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

package util

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	"github.com/liqotech/liqo/pkg/utils/pod"
)

// PodType -> defines the type of a pod (local/remote).
type PodType string

const (
	// PodLocal -> the pod is local.
	PodLocal = "local"
	// PodRemote -> the pod is remote.
	PodRemote = "remote"
)

// IsPodUp waits for a specific namespace/podName to be ready. It returns true if the pod within the timeout, false otherwise.
func IsPodUp(ctx context.Context, client kubernetes.Interface, namespace, podName string, podType PodType) bool {
	klog.Infof("checking if %s pod %s/%s is ready", podType, namespace, podName)
	podToCheck, err := client.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("an error occurred while getting %s pod %s/%s: %v", podType, namespace, podName, err)
		return false
	}

	ready, reason := pod.IsPodReady(podToCheck)
	message := "ready"
	if !ready {
		message = "NOT ready"
	}

	klog.Infof("%s pod %s/%s is %s (reason: %s)", podType, podToCheck.Namespace, podToCheck.Name, message, reason)
	return ready
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
		if ready, _ := pod.IsPodReady(&pods.Items[index]); !ready {
			notReady = append(notReady, pods.Items[index].Name)
		}
		ready = append(ready, pods.Items[index].Name)
	}
	return ready, notReady, nil
}
