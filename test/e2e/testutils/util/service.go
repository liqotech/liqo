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
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/liqotech/liqo/pkg/utils/resource"
)

// ServiceOption is a function that modifies a Service.
type ServiceOption func(*corev1.Service)

// EnforceService creates or updates a Service with the given name in the given namespace.
func EnforceService(ctx context.Context, cl client.Client, namespace, name string, options ...ServiceOption) error {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}

	return Second(resource.CreateOrUpdate(ctx, cl, svc, func() error {
		svc.Spec.Ports = []corev1.ServicePort{
			{
				Name:       "http",
				Protocol:   corev1.ProtocolTCP,
				Port:       80,
				TargetPort: intstr.FromInt(80),
			},
		}
		svc.Spec.Selector = map[string]string{"app": name}
		svc.Spec.Type = corev1.ServiceTypeClusterIP

		for _, opt := range options {
			opt(svc)
		}

		return nil
	}))
}

// WithNodePort sets the Service type to NodePort.
func WithNodePort() ServiceOption {
	return func(svc *corev1.Service) {
		svc.Spec.Type = corev1.ServiceTypeNodePort
	}
}
