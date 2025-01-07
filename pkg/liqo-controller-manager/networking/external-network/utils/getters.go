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

package utils

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/ptr"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
)

// ParseEndpoint parses an endpoint from a map.
func ParseEndpoint(endpoint map[string]interface{}) *networkingv1beta1.EndpointStatus {
	res := &networkingv1beta1.EndpointStatus{}
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

// ParseInternalEndpoint parses an internal endpoint from a map.
func ParseInternalEndpoint(internalEndpoint map[string]interface{}) *networkingv1beta1.InternalGatewayEndpoint {
	res := &networkingv1beta1.InternalGatewayEndpoint{}
	if value, ok := internalEndpoint["ip"]; ok {
		res.IP = ptr.To(networkingv1beta1.IP(value.(string)))
	}
	if value, ok := internalEndpoint["node"]; ok {
		res.Node = ptr.To(value.(string))
	}
	return res
}

// ParseRef parses an ObjectReference from a map.
func ParseRef(ref map[string]interface{}) *corev1.ObjectReference {
	res := &corev1.ObjectReference{}
	if value, ok := ref["apiVersion"]; ok {
		res.APIVersion = value.(string)
	}
	if value, ok := ref["kind"]; ok {
		res.Kind = value.(string)
	}
	if value, ok := ref["name"]; ok {
		res.Name = value.(string)
	}
	if value, ok := ref["namespace"]; ok {
		res.Namespace = value.(string)
	}
	if value, ok := ref["uid"]; ok {
		res.UID = value.(types.UID)
	}
	return res
}

// GetIfExists returns the value of a key in a map casting its type, or nil if the key is not present
// or the type is wrong.
func GetIfExists[T any](m map[string]interface{}, key string) (*T, bool) {
	if value, ok := m[key]; ok {
		v, ok := value.(T)
		return &v, ok
	}
	return nil, false
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
	plural, _ := meta.UnsafeGuessKindToResource(schema.GroupVersionKind{
		Kind: kind,
	})
	// lowercased and pluralized
	return plural.Resource
}

// ResourceToKind returns the kind name for a given resource.
func ResourceToKind(gvr schema.GroupVersionResource, kubeClient kubernetes.Interface) (string, error) {
	res, err := kubeClient.Discovery().ServerResourcesForGroupVersion(gvr.GroupVersion().String())
	if err != nil {
		return "", err
	}

	for i := range res.APIResources {
		if plural, singular := meta.UnsafeGuessKindToResource(schema.GroupVersionKind{
			Group:   gvr.Group,
			Version: gvr.Version,
			Kind:    res.APIResources[i].Kind,
		}); plural.Resource == gvr.Resource || singular.Resource == gvr.Resource {
			return res.APIResources[i].Kind, nil
		}
	}

	return "", fmt.Errorf("unable to find Kind name associated to resources %q", gvr.Resource)
}
