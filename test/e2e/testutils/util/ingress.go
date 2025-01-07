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

	netv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/liqotech/liqo/pkg/utils/resource"
)

// IngressOption is a function that modifies a Ingress.
type IngressOption func(*netv1.Ingress)

// EnforceIngress creates or updates a Ingress with the given name in the given namespace.
func EnforceIngress(ctx context.Context, cl client.Client, namespace, name string, options ...IngressOption) error {
	ing := &netv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}

	return Second(resource.CreateOrUpdate(ctx, cl, ing, func() error {
		ing.Spec.DefaultBackend = &netv1.IngressBackend{
			Service: &netv1.IngressServiceBackend{
				Name: "default-backend",
				Port: netv1.ServiceBackendPort{
					Number: 80,
				},
			},
		}
		ing.Spec.Rules = []netv1.IngressRule{
			{
				Host: "example.com",
				IngressRuleValue: netv1.IngressRuleValue{
					HTTP: &netv1.HTTPIngressRuleValue{
						Paths: []netv1.HTTPIngressPath{
							{
								PathType: func() *netv1.PathType {
									pt := netv1.PathTypePrefix
									return &pt
								}(),
								Path: "/",
								Backend: netv1.IngressBackend{
									Service: &netv1.IngressServiceBackend{
										Name: "example-backend",
										Port: netv1.ServiceBackendPort{
											Number: 80,
										},
									},
								},
							},
						},
					},
				},
			},
		}

		for _, opt := range options {
			opt(ing)
		}

		return nil
	}))
}
