// Copyright 2019-2026 The Liqo Authors
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

package setup

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/liqotech/liqo/pkg/liqoctl/test/network/client"
	"github.com/liqotech/liqo/pkg/liqoctl/test/network/flags"
)

// forgeIPv4SingleStack sets the service IP family fields to IPv4 single-stack.
// The Liqo network fabric is IPv4 only: without this, the services would be IPv6 single-stack
// on dual-stack clusters whose primary IP family is IPv6.
func forgeIPv4SingleStack(spec *corev1.ServiceSpec) {
	spec.IPFamilyPolicy = ptr.To(corev1.IPFamilyPolicySingleStack)
	spec.IPFamilies = []corev1.IPFamily{corev1.IPv4Protocol}
}

// CreateService creates a service resource.
func CreateService(ctx context.Context, cl *client.Client, opts *flags.Options) error {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      DeploymentName,
			Namespace: NamespaceName,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{PodLabelApp: DeploymentName},
			Ports:    []corev1.ServicePort{{Port: 80, TargetPort: intstr.IntOrString{IntVal: 80}}},
		},
	}
	forgeIPv4SingleStack(&svc.Spec)
	if err := cl.Consumer.Create(ctx, svc); err != nil && ctrlclient.IgnoreAlreadyExists(err) != nil {
		return err
	}

	svcnp := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      DeploymentName + "np",
			Namespace: NamespaceName,
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeNodePort,
			Selector: map[string]string{PodLabelApp: DeploymentName},
			Ports:    []corev1.ServicePort{{Port: 80, TargetPort: intstr.IntOrString{IntVal: 80}}},
		},
	}
	forgeIPv4SingleStack(&svcnp.Spec)
	if err := cl.Consumer.Create(ctx, svcnp); err != nil && ctrlclient.IgnoreAlreadyExists(err) != nil {
		return err
	}

	if opts.LoadBalancer {
		svclb := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      DeploymentName + "lb",
				Namespace: NamespaceName,
			},
			Spec: corev1.ServiceSpec{
				Type:     corev1.ServiceTypeLoadBalancer,
				Selector: map[string]string{PodLabelApp: DeploymentName},
				Ports:    []corev1.ServicePort{{Port: 80, TargetPort: intstr.IntOrString{IntVal: 80}}},
			},
		}
		forgeIPv4SingleStack(&svclb.Spec)
		if err := cl.Consumer.Create(ctx, svclb); err != nil && ctrlclient.IgnoreAlreadyExists(err) != nil {
			return err
		}
	}
	return nil
}
