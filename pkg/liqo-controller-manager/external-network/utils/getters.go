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
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	networkingv1alpha1 "github.com/liqotech/liqo/apis/networking/v1alpha1"
)

// ParseEndpoint parses an endpoint from a map.
func ParseEndpoint(endpoint map[string]interface{}) *networkingv1alpha1.EndpointStatus {
	res := &networkingv1alpha1.EndpointStatus{}
	if value, ok := endpoint["addresses"]; ok {
		res.Addresses = interfaceListToList[string](value.([]interface{}))
	}
	if value, ok := endpoint["port"]; ok {
		res.Port = int32(value.(int64))
	}
	if value, ok := endpoint["protocol"]; ok {
		tmp := corev1.Protocol(value.(string))
		res.Protocol = &tmp
	}
	return res
}

func interfaceListToList[T any](list []interface{}) []T {
	res := make([]T, len(list))
	for i, v := range list {
		res[i] = v.(T)
	}
	return res
}

// ParseGroupVersionResource parses a GroupVersionResource from a string in the form group/version/resource.
func ParseGroupVersionResource(gvr string) (schema.GroupVersionResource, error) {
	tmp := strings.Split(gvr, "/")
	if len(tmp) != 3 {
		return schema.GroupVersionResource{}, fmt.Errorf("invalid resource %q", gvr)
	}
	return schema.GroupVersionResource{
		Group:    tmp[0],
		Version:  tmp[1],
		Resource: tmp[2],
	}, nil
}

// GetValueOrDefault returns the value of a key in a map, or a default value if the key is not present.
func GetValueOrDefault(m map[string]interface{}, key, defaultValue string) string {
	if value, ok := m[key]; ok {
		return value.(string)
	}
	return defaultValue
}

// TranslateMap translates a map[string]interface{} to a map[string]string.
func TranslateMap(obj interface{}) map[string]string {
	if obj == nil {
		return nil
	}

	m, ok := obj.(map[string]interface{})
	if !ok {
		return nil
	}

	res := make(map[string]string)
	for k, v := range m {
		res[k] = v.(string)
	}
	return res
}

// KindToResource returns the resource name for a given kind.
func KindToResource(kind string) string {
	// lowercased and pluralized
	return strings.ToLower(kind) + "s"
}
