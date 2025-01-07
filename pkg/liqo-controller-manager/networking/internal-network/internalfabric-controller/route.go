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

package internalfabriccontroller

import (
	"context"
	"fmt"
	"sort"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	"github.com/liqotech/liqo/pkg/fabric"
	"github.com/liqotech/liqo/pkg/utils/resource"
)

func (r *InternalFabricReconciler) ensureRouteConfiguration(ctx context.Context, internalFabric *networkingv1beta1.InternalFabric) error {
	if internalFabric.Spec.Interface.Node.Name == "" {
		return fmt.Errorf("internal fabric %q has node interface name empty", client.ObjectKeyFromObject(internalFabric))
	}

	route := &networkingv1beta1.RouteConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GenerateRouteConfigurationName(internalFabric),
			Namespace: internalFabric.Namespace,
		},
	}
	_, err := resource.CreateOrUpdate(ctx, r.Client, route, func() error {
		// Forge metadata
		if route.Labels == nil {
			route.Labels = make(labels.Set)
		}
		route.SetLabels(labels.Merge(route.Labels, fabric.ForgeRouteTargetLabels()))

		// Add route rule for every remote CIDR
		var rules []networkingv1beta1.Rule

		rules = append(rules, networkingv1beta1.Rule{
			Dst: ptr.To(networkingv1beta1.CIDR(fmt.Sprintf("%s/32", internalFabric.Spec.Interface.Gateway.IP))),
			Routes: []networkingv1beta1.Route{
				{
					Dst:   ptr.To(networkingv1beta1.CIDR(fmt.Sprintf("%s/32", internalFabric.Spec.Interface.Gateway.IP))),
					Dev:   ptr.To(internalFabric.Spec.Interface.Node.Name),
					Scope: ptr.To(networkingv1beta1.LinkScope),
				},
			},
		})

		remoteCIDRs := internalFabric.Spec.RemoteCIDRs
		// sort slice to prevent useless updates if CIDRs are in different order
		sort.Slice(remoteCIDRs, func(i, j int) bool {
			return remoteCIDRs[i] < remoteCIDRs[j]
		})
		for _, remoteCIDR := range remoteCIDRs {
			rule := networkingv1beta1.Rule{
				Routes: []networkingv1beta1.Route{
					{
						Dst: ptr.To(remoteCIDR),
						Gw:  ptr.To(internalFabric.Spec.Interface.Gateway.IP),
					},
				},
				Dst: ptr.To(remoteCIDR),
			}
			rules = append(rules, rule)
		}

		route.Spec = networkingv1beta1.RouteConfigurationSpec{
			Table: networkingv1beta1.Table{
				Name:  route.Name,
				Rules: rules,
			},
		}

		return controllerutil.SetControllerReference(internalFabric, route, r.Scheme)
	})
	if err != nil {
		klog.Errorf("Unable to create or update RouteConfiguration %q: %s", route.Name, err)
		return err
	}

	return nil
}

// GenerateRouteConfigurationName returns the name of the RouteConfiguration associated to the InternalFabric.
func GenerateRouteConfigurationName(internalFabric *networkingv1beta1.InternalFabric) string {
	return fmt.Sprintf("%s-node-gw", internalFabric.Name)
}
