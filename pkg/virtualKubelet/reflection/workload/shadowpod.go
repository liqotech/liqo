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

package workload

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"

	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
)

// RemoteShadowNamespacedKeyer returns a shadowpod keyer associated with the given namespace, retrieving the
// object name from its metadata.
func RemoteShadowNamespacedKeyer(namespace, nodename string) func(metadata metav1.Object) []types.NamespacedName {
	return func(metadata metav1.Object) []types.NamespacedName {
		label, ok := metadata.GetLabels()[forge.LiqoOriginClusterNodeName]
		klog.V(4).Infof("RemoteShadowNamespaceKeyer: Comparing %q with %q", label, nodename)
		if ok && label == nodename {
			return []types.NamespacedName{{Namespace: namespace, Name: metadata.GetName()}}
		}
		return []types.NamespacedName{}
	}
}
