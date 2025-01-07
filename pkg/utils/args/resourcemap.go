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

package args

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// ResourceMap implements the flag.Value interface and allows to parse stringified maps
// of resources in the form: "key1=4,key2=2Gi".
type ResourceMap struct {
	Map map[string]resource.Quantity
}

// String returns the stringified map.
func (rm ResourceMap) String() string {
	if rm.Map == nil {
		return ""
	}

	items := make([]string, len(rm.Map))
	i := 0
	for k, v := range rm.Map {
		items[i] = fmt.Sprintf("%s=%s", k, v.String())
		i++
	}
	return strings.Join(items, ",")
}

// Set parses the provided string into the map[string]resource.Quantity map.
func (rm *ResourceMap) Set(str string) error {
	if rm.Map == nil {
		rm.Map = map[string]resource.Quantity{}
	}
	if str == "" {
		return nil
	}
	chunks := strings.Split(str, ",")
	for i := range chunks {
		chunk := chunks[i]
		strs := strings.Split(chunk, "=")
		if len(strs) != 2 {
			return fmt.Errorf("invalid value %v", chunk)
		}
		qnt, err := resource.ParseQuantity(strs[1])
		if err != nil {
			return err
		}
		rm.Map[strs[0]] = qnt
	}
	return nil
}

// Type returns the resourceMap type.
func (rm ResourceMap) Type() string {
	return "resourceMap"
}

// ToResourceList converts the ResourceMap into a corev1.ResourceList.
func (rm *ResourceMap) ToResourceList() corev1.ResourceList {
	res := corev1.ResourceList{}
	for k, v := range rm.Map {
		res[corev1.ResourceName(k)] = v
	}
	return res
}
