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

// SecretOption is a function that modifies a Secret.
type SecretOption func(*corev1.Secret)

// EnforceSecret creates or updates a Secret with the given name in the given namespace.
func EnforceSecret(ctx context.Context, cl client.Client, namespace, name string, options ...SecretOption) error {
	sec := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		StringData: map[string]string{
			"test": "abcde",
		},
		Type: corev1.SecretTypeOpaque,
	}

	return Second(resource.CreateOrUpdate(ctx, cl, sec, func() error {
		if sec.Data == nil {
			sec.Data = make(map[string][]byte)
		}
		if sec.StringData == nil {
			sec.StringData = make(map[string]string)
		}

		for _, opt := range options {
			opt(sec)
		}

		return nil
	}))
}
