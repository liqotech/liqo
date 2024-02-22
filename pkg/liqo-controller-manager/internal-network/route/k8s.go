// Copyright 2019-2024 The Liqo Authors
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

	networkingv1alpha1 "github.com/liqotech/liqo/apis/networking/v1alpha1"
	gwfabric "github.com/liqotech/liqo/pkg/gateway/fabric"
	"github.com/liqotech/liqo/pkg/gateway/tunnel"
)

// GenerateRouteConfigurationName generates the name of the route configuration for the given node.
func GenerateRouteConfigurationName(nodeName string) string {
	return fmt.Sprintf("%s-gw-node", nodeName)
}

func enforeRoutePodPresence(ctx context.Context, cl client.Client, scheme *runtime.Scheme,
	opts *Options, pod *corev1.Pod) (controllerutil.OperationResult, error) {
	if pod.Spec.NodeName == "" {
		return "", nil
	}

	if pod.Status.PodIP == "" {
		return "", nil
	}

	if pod.Spec.HostNetwork {
		return "", nil
	}

	internalnode := &networkingv1alpha1.InternalNode{}
	if err := cl.Get(ctx, client.ObjectKey{Name: pod.Spec.NodeName}, internalnode); err != nil {
		return "", err
	}

	routecfg := &networkingv1alpha1.RouteConfiguration{
		ObjectMeta: metav1.ObjectMeta{Name: GenerateRouteConfigurationName(pod.Spec.NodeName), Namespace: opts.Namespace},
	}

	op, err := controllerutil.CreateOrUpdate(ctx, cl, routecfg, forgeRoutePodUpdateFunction(internalnode, routecfg, pod, scheme))

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
	routecfg := networkingv1alpha1.RouteConfiguration{}
	if err := cl.Get(ctx, client.ObjectKey{Name: GenerateRouteConfigurationName(nodeName), Namespace: opts.Namespace}, &routecfg); err != nil {
		return err
	}

	if _, err := controllerutil.CreateOrUpdate(ctx, cl, &routecfg, forgeRoutePodDeleteFunction(pod, &routecfg)); err != nil {
		return err
	}

	DeletePodKeyFromMap(client.ObjectKeyFromObject(pod))

	return nil
}

func forgeRoutePodUpdateFunction(internalnode *networkingv1alpha1.InternalNode, routecfg *networkingv1alpha1.RouteConfiguration,
	pod *corev1.Pod, scheme *runtime.Scheme) controllerutil.MutateFn {
	return func() error {
		if err := controllerutil.SetOwnerReference(internalnode, routecfg, scheme); err != nil {
			return err
		}

		routecfg.SetLabels(gwfabric.ForgeRouteInternalTargetLabels())

		routecfg.Spec.Table.Name = pod.Spec.NodeName

		if routecfg.Spec.Table.Rules == nil || len(routecfg.Spec.Table.Rules) != 2 {
			routecfg.Spec.Table.Rules = make([]networkingv1alpha1.Rule, 2)
			routecfg.Spec.Table.Rules[0].Dst = ptr.To(networkingv1alpha1.CIDR(
				fmt.Sprintf("%s/32", internalnode.Spec.Interface.Node.IP),
			))
			routecfg.Spec.Table.Rules[1].Iif = ptr.To(tunnel.TunnelInterfaceName)
		}

		if exists := routeContainsNode(internalnode, &routecfg.Spec.Table.Rules[0]); !exists {
			addNodeToRoute(internalnode, &routecfg.Spec.Table.Rules[0])
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
func forgeRoutePodDeleteFunction(pod *corev1.Pod, routecfg *networkingv1alpha1.RouteConfiguration) controllerutil.MutateFn {
	return func() error {
		if routecfg.Spec.Table.Rules == nil || len(routecfg.Spec.Table.Rules) == 0 {
			return nil
		}

		if existingroute, exists := routeContainsPod(pod, &routecfg.Spec.Table.Rules[1]); exists {
			routecfg.Spec.Table.Rules[1].Routes = slices.DeleteFunc(routecfg.Spec.Table.Rules[1].Routes, func(r networkingv1alpha1.Route) bool {
				return r.Dst == existingroute.Dst
			})
		}

		return nil
	}
}

func routeContainsPod(pod *corev1.Pod, rule *networkingv1alpha1.Rule) (*networkingv1alpha1.Route, bool) {
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

func routeContainsNode(internalnode *networkingv1alpha1.InternalNode, rule *networkingv1alpha1.Rule) bool {
	for i := range rule.Routes {
		if rule.Routes[i].Dst.String() == fmt.Sprintf("%s/32", internalnode.Spec.Interface.Node.IP) {
			return true
		}
	}
	return false
}

func addPodToRoute(pod *corev1.Pod, internalnode *networkingv1alpha1.InternalNode, rule *networkingv1alpha1.Rule) {
	route := networkingv1alpha1.Route{
		Dst: ptr.To(networkingv1alpha1.CIDR(fmt.Sprintf("%s/32", pod.Status.PodIP))),
		Gw:  ptr.To(internalnode.Spec.Interface.Node.IP),
		TargetRef: &corev1.ObjectReference{
			Kind:      pod.GetObjectKind().GroupVersionKind().Kind,
			Name:      pod.GetName(),
			Namespace: pod.GetNamespace(),
			UID:       pod.GetUID(),
		},
	}
	rule.Routes = append(rule.Routes, route)
}

func addNodeToRoute(internlnode *networkingv1alpha1.InternalNode, rule *networkingv1alpha1.Rule) {
	rule.Routes = []networkingv1alpha1.Route{
		{
			Dst:   ptr.To(networkingv1alpha1.CIDR(fmt.Sprintf("%s/32", internlnode.Spec.Interface.Node.IP))),
			Dev:   &internlnode.Spec.Interface.Gateway.Name,
			Scope: ptr.To(networkingv1alpha1.LinkScope),
		},
	}
}

func updatePodToRoute(pod *corev1.Pod, internalnode *networkingv1alpha1.InternalNode, route *networkingv1alpha1.Route) {
	route.Dst = ptr.To(networkingv1alpha1.CIDR(fmt.Sprintf("%s/32", pod.Status.PodIP)))
	route.Gw = ptr.To(internalnode.Spec.Interface.Node.IP)
}
