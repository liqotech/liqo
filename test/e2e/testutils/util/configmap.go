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

package util

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/liqotech/liqo/pkg/utils/resource"
)

// ConfigMapOption is a function that modifies a ConfigMap.
type ConfigMapOption func(*corev1.ConfigMap)

// EnforceConfigMap creates or updates a ConfigMap with the given name in the given namespace.
func EnforceConfigMap(ctx context.Context, cl client.Client, namespace, name string, options ...ConfigMapOption) error {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: map[string]string{
			"test": "true",
		},
	}

	return Second(resource.CreateOrUpdate(ctx, cl, cm, func() error {
		if cm.Data == nil {
			cm.Data = make(map[string]string)
		}

		for _, opt := range options {
			opt(cm)
		}

		return nil
	}))
}
