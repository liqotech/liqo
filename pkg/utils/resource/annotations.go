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
	// globalAnnotations stores the global annotations that should be added to all resources.
	globalAnnotations = make(map[string]string)
)

// SetGlobalAnnotations sets the global annotations that should be added to all resources.
func SetGlobalAnnotations(annotations map[string]string) {
	if annotations == nil {
		globalAnnotations = make(map[string]string)
		return
	}
	globalAnnotations = annotations
}

// GetGlobalAnnotations returns a copy of the current global annotations.
func GetGlobalAnnotations() map[string]string {
	return maps.Clone(globalAnnotations)
}

// AddGlobalAnnotations adds the global annotations to the given object's annotations.
// If the object's annotations is nil, it will be initialized.
func AddGlobalAnnotations(obj metav1.Object) {
	if obj.GetAnnotations() == nil {
		obj.SetAnnotations(make(map[string]string))
	}
	maps.Copy(obj.GetAnnotations(), globalAnnotations)
}
