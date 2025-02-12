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

package route

import (
	"context"
	"fmt"
	"slices"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	"github.com/liqotech/liqo/pkg/gateway"
	"github.com/liqotech/liqo/pkg/gateway/tunnel"
	"github.com/liqotech/liqo/pkg/utils/resource"
)

// generatePodRouteConfigurationName generates the name of the route configuration for the given node.
func generatePodRouteConfigurationName(nodeName string) string {
	return fmt.Sprintf("%s-gw-node", nodeName)
}

func enforceRoutePodPresence(ctx context.Context, cl client.Client, scheme *runtime.Scheme,
	opts *Options, pod *corev1.Pod) (controllerutil.OperationResult, error) {
	if pod.Spec.NodeName == "" {
		return "", nil
	}

	if pod.Status.PodIP == "" {
		return "", nil
	}

	internalnode := &networkingv1beta1.InternalNode{}
	if err := cl.Get(ctx, client.ObjectKey{Name: pod.Spec.NodeName}, internalnode); err != nil {
		return "", err
	}

	routecfg := &networkingv1beta1.RouteConfiguration{
		ObjectMeta: metav1.ObjectMeta{Name: generatePodRouteConfigurationName(pod.Spec.NodeName), Namespace: opts.Namespace},
	}

	op, err := resource.CreateOrUpdate(ctx, cl, routecfg, forgeRoutePodUpdateFunction(internalnode, routecfg, pod, scheme))

	return op, err
}

func enforceRoutePodAbsence(ctx context.Context, cl client.Client, opts *Options, pod *corev1.Pod) error {
	nodeName, err := GetPodNodeFromMap(client.ObjectKeyFromObject(pod))
	if err != nil {
		return err
	}
	if nodeName == "" {
		return fmt.Errorf("unable to get node name from pod %s/%s", pod.GetNamespace(), pod.GetName())
	}
	routecfg := networkingv1beta1.RouteConfiguration{}
	if err := cl.Get(ctx, client.ObjectKey{Name: generatePodRouteConfigurationName(nodeName), Namespace: opts.Namespace}, &routecfg); err != nil {
		return err
	}

	if _, err := resource.CreateOrUpdate(ctx, cl, &routecfg, forgeRoutePodDeleteFunction(pod, &routecfg)); err != nil {
		return err
	}

	DeletePodKeyFromMap(client.ObjectKeyFromObject(pod))

	return nil
}

func forgeRoutePodUpdateFunction(internalnode *networkingv1beta1.InternalNode, routecfg *networkingv1beta1.RouteConfiguration,
	pod *corev1.Pod, scheme *runtime.Scheme) controllerutil.MutateFn {
	return func() error {
		if err := controllerutil.SetOwnerReference(internalnode, routecfg, scheme); err != nil {
			return err
		}

		routecfg.SetLabels(gateway.ForgeRouteInternalTargetLabels())

		routecfg.Spec.Table.Name = pod.Spec.NodeName

		if routecfg.Spec.Table.Rules == nil || len(routecfg.Spec.Table.Rules) < 1 {
			routecfg.Spec.Table.Rules = make([]networkingv1beta1.Rule, 1)
			routecfg.Spec.Table.Rules[0].Dst = ptr.To(networkingv1beta1.CIDR(
				fmt.Sprintf("%s/32", internalnode.Spec.Interface.Node.IP),
			))
		}

		if exists := routeContainsNode(internalnode, &routecfg.Spec.Table.Rules[0]); !exists {
			addNodeToRoute(internalnode, &routecfg.Spec.Table.Rules[0])
		}

		// We don't need to add routes for pods running on the host network.
		// Anyways, it's important to add the route for the node itself.
		if pod.Spec.HostNetwork {
			return nil
		}

		if routecfg.Spec.Table.Rules == nil || len(routecfg.Spec.Table.Rules) < 2 {
			routecfg.Spec.Table.Rules = append(routecfg.Spec.Table.Rules, networkingv1beta1.Rule{})
			routecfg.Spec.Table.Rules[1].Iif = ptr.To(tunnel.TunnelInterfaceName)
		}

		if existingroute, exists := routeContainsPod(pod, &routecfg.Spec.Table.Rules[1]); exists {
			updatePodToRoute(pod, internalnode, existingroute)
		} else {
			addPodToRoute(pod, internalnode, &routecfg.Spec.Table.Rules[1])
		}

		return nil
	}
}

// forgeRoutePodDeleteFunction removes the pod entries from the route configuration.
func forgeRoutePodDeleteFunction(pod *corev1.Pod, routecfg *networkingv1beta1.RouteConfiguration) controllerutil.MutateFn {
	return func() error {
		if routecfg.Spec.Table.Rules == nil || len(routecfg.Spec.Table.Rules) <= 1 {
			return nil
		}

		// We allocate this array statically with length 2.
		// The rule we are managing is the second one.
		if existingroute, exists := routeContainsPod(pod, &routecfg.Spec.Table.Rules[1]); exists {
			routecfg.Spec.Table.Rules[1].Routes = slices.DeleteFunc(routecfg.Spec.Table.Rules[1].Routes, func(r networkingv1beta1.Route) bool {
				return r.Dst == existingroute.Dst
			})
		}

		return nil
	}
}

func routeContainsPod(pod *corev1.Pod, rule *networkingv1beta1.Rule) (*networkingv1beta1.Route, bool) {
	for i := range rule.Routes {
		if string(*rule.Routes[i].Dst) == fmt.Sprintf("%s/32", pod.Status.PodIP) {
			return &rule.Routes[i], true
		}
		// This is necessary to detect pods that are not present anymore in etcd but still have a route.
		if rule.Routes[i].TargetRef != nil &&
			rule.Routes[i].TargetRef.Name == pod.GetName() &&
			rule.Routes[i].TargetRef.Namespace == pod.GetNamespace() {
			return &rule.Routes[i], true
		}
	}
	return nil, false
}

func routeContainsNode(internalnode *networkingv1beta1.InternalNode, rule *networkingv1beta1.Rule) bool {
	for i := range rule.Routes {
		if rule.Routes[i].Dst.String() == fmt.Sprintf("%s/32", internalnode.Spec.Interface.Node.IP) {
			return true
		}
	}
	return false
}

func addPodToRoute(pod *corev1.Pod, internalnode *networkingv1beta1.InternalNode, rule *networkingv1beta1.Rule) {
	rule.Routes = append(rule.Routes, networkingv1beta1.Route{
		Dst: ptr.To(networkingv1beta1.CIDR(fmt.Sprintf("%s/32", pod.Status.PodIP))),
		Gw:  ptr.To(internalnode.Spec.Interface.Node.IP),
		TargetRef: &corev1.ObjectReference{
			Kind:      pod.GetObjectKind().GroupVersionKind().Kind,
			Name:      pod.GetName(),
			Namespace: pod.GetNamespace(),
			UID:       pod.GetUID(),
		},
	})
}

func addNodeToRoute(internalnode *networkingv1beta1.InternalNode, rule *networkingv1beta1.Rule) {
	rule.Routes = []networkingv1beta1.Route{
		{
			Dst:   ptr.To(networkingv1beta1.CIDR(fmt.Sprintf("%s/32", internalnode.Spec.Interface.Node.IP))),
			Dev:   &internalnode.Spec.Interface.Gateway.Name,
			Scope: ptr.To(networkingv1beta1.LinkScope),
		},
	}
}

func updatePodToRoute(pod *corev1.Pod, internalnode *networkingv1beta1.InternalNode, route *networkingv1beta1.Route) {
	route.Dst = ptr.To(networkingv1beta1.CIDR(fmt.Sprintf("%s/32", pod.Status.PodIP)))
	route.Gw = ptr.To(internalnode.Spec.Interface.Node.IP)
}
