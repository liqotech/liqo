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

package route

import (
	"fmt"
	"sync"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	// podKeyToNode is a map that associates a pod key to the node name where the pod is running.
	// We need it to get the correct routeconfiguration resource when a pod is deleted.
	podKeyToNode sync.Map
)

// PopulatePodKeyToNodeMap adds the given pod to the map.
func PopulatePodKeyToNodeMap(pod *corev1.Pod) {
	podKeyToNode.Store(client.ObjectKeyFromObject(pod).String(), pod.Spec.NodeName)
}

// GetPodNodeFromMap returns the node name of the pod with the given key.
func GetPodNodeFromMap(objKey client.ObjectKey) (string, error) {
	v, ok := podKeyToNode.Load(objKey.String())
	if !ok {
		return "", nil
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("cannot convert %v to string", v)
	}
	return s, nil
}

// DeletePodKeyFromMap deletes the given pod from the map.
func DeletePodKeyFromMap(objKey client.ObjectKey) {
	podKeyToNode.Delete(objKey.String())
}
