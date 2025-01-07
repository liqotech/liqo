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

package ipmapping

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	ipamv1alpha1 "github.com/liqotech/liqo/apis/ipam/v1alpha1"
	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	"github.com/liqotech/liqo/pkg/utils/resource"
)

// CreateOrUpdateIP creates or updates an IP resource for the given pod.
func CreateOrUpdateIP(ctx context.Context, cl client.Client, scheme *runtime.Scheme, pod *corev1.Pod) error {
	ip := &ipamv1alpha1.IP{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pod.Name,
			Namespace: pod.Namespace,
		},
	}
	if _, err := resource.CreateOrUpdate(ctx, cl, ip, mutateIP(ip, pod, scheme)); err != nil {
		return fmt.Errorf("unable to create or update IP %q: %w", ip.Name, err)
	}
	return nil
}

func mutateIP(ip *ipamv1alpha1.IP, pod *corev1.Pod, scheme *runtime.Scheme) controllerutil.MutateFn {
	return func() error {
		if err := controllerutil.SetOwnerReference(pod, ip, scheme); err != nil {
			return fmt.Errorf("unable to set owner reference for IP %q: %w", ip.Name, err)
		}
		ip.SetLabels(labels.Merge(ip.GetLabels(), forgeIPLabels(pod)))
		ip.Spec.IP = networkingv1beta1.IP(pod.Status.PodIP)
		return nil
	}
}
