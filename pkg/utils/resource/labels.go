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

package resource

import (
	"maps"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	// globalLabels stores the global labels that should be added to all resources.
	globalLabels = make(map[string]string)
)

// SetGlobalLabels sets the global labels that should be added to all resources.
func SetGlobalLabels(labels map[string]string) {
	if labels == nil {
		globalLabels = make(map[string]string)
		return
	}
	globalLabels = labels
}

// GetGlobalLabels returns a copy of the current global labels.
func GetGlobalLabels() map[string]string {
	return maps.Clone(globalLabels)
}

// AddGlobalLabels adds the global labels to the given object's labels.
// If the object's labels is nil, it will be initialized.
func AddGlobalLabels(obj metav1.Object) {
	if obj.GetLabels() == nil {
		obj.SetLabels(make(map[string]string))
	}
	maps.Copy(obj.GetLabels(), globalLabels)
}
