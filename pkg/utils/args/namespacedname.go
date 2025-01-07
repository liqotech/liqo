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
)

// SplitNamespacedName splits a namespaced name string into its namespace and name components.
func SplitNamespacedName(nsName string) (splits []string, err error) {
	if nsName == "" {
		return nil, fmt.Errorf("empty namespaced name")
	}
	splits = strings.Split(nsName, "/")
	if len(splits) != 2 {
		return nil, fmt.Errorf("not exactly one '/' sepatator is present in namespaced name: %v", nsName)
	}
	if splits[0] == "" {
		return nil, fmt.Errorf("empty namespace in namespaced name: %v", nsName)
	}
	if splits[1] == "" {
		return nil, fmt.Errorf("empty name in namespaced name: %v", nsName)
	}
	return splits, nil
}

// GetObjectRefFromNamespacedName returns an ObjectReference from a namespaced name string.
func GetObjectRefFromNamespacedName(nsName string) (*corev1.ObjectReference, error) {
	splits, err := SplitNamespacedName(nsName)
	if err != nil {
		return nil, err
	}

	return &corev1.ObjectReference{
		Namespace: splits[0],
		Name:      splits[1],
	}, nil
}
