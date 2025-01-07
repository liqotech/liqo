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

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	"github.com/liqotech/liqo/pkg/utils/getters"
)

// NewLeftoverPodsSource returns a new LeftoversPodSource.
func NewLeftoverPodsSource(src <-chan event.GenericEvent, eh handler.EventHandler) source.Source {
	return source.Channel(src, eh)
}

// NewLeftoverPodsEventHandler returns a new LeftoverPodsEventHandler.
func NewLeftoverPodsEventHandler() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(_ context.Context, o client.Object) []reconcile.Request {
		pod, ok := o.(*corev1.Pod)
		if !ok {
			klog.Errorf("unable to cast object %s to pod", o.GetName())
			return nil
		}
		return []reconcile.Request{
			{
				NamespacedName: client.ObjectKey{
					Name:      pod.GetName(),
					Namespace: pod.GetNamespace(),
				},
			},
		}
	})
}

// CheckLeftoverRoutes lists all currently existing routeconfigurations and adds their
// pod to the queue if its pod does not exist anymore.
// This will detect routes that exist with no
// corresponding pod; these routes need to be deleted. We only need to
// do this once on startup, because in steady-state these are detected (but
// some stragglers could have been left behind if this controller
// reboots).
// It also populates podKeyToNode map with existing pods nodename.
func (r *PodReconciler) CheckLeftoverRoutes(ctx context.Context) error {
	routecfglist, err := getters.ListRouteConfigurationsInNamespaceByLabel(ctx, r.Client, r.Options.Namespace, labels.Everything())
	if err != nil {
		return err
	}
	return r.processRouteConfigurations(ctx, routecfglist)
}

func (r *PodReconciler) processRouteConfigurations(ctx context.Context, routecfglist *networkingv1beta1.RouteConfigurationList) error {
	for i := range routecfglist.Items {
		if routecfglist.Items[i].Spec.Table.Rules == nil || len(routecfglist.Items[i].Spec.Table.Rules) == 0 {
			continue
		}
		if err := r.processRules(ctx, &routecfglist.Items[i]); err != nil {
			return err
		}
	}
	return nil
}

func (r *PodReconciler) processRules(ctx context.Context, routecfg *networkingv1beta1.RouteConfiguration) error {
	for j := range routecfg.Spec.Table.Rules {
		if err := r.processRoutes(ctx, &routecfg.Spec.Table.Rules[j], routecfg.Spec.Table.Name); err != nil {
			return err
		}
	}
	return nil
}

func (r *PodReconciler) processRoutes(ctx context.Context, rule *networkingv1beta1.Rule, nodeName string) error {
	for k := range rule.Routes {
		if rule.Routes[k].TargetRef == nil {
			continue
		}
		if err := r.processRoute(ctx, &rule.Routes[k], nodeName); err != nil {
			return err
		}
	}
	return nil
}

func (r *PodReconciler) processRoute(ctx context.Context, route *networkingv1beta1.Route, nodeName string) error {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      route.TargetRef.Name,
			Namespace: route.TargetRef.Namespace,
		},
	}
	if err := r.Get(ctx, client.ObjectKey{Name: route.TargetRef.Name, Namespace: route.TargetRef.Namespace}, pod); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		klog.Infof("pod %s not found, adding to queue", route.TargetRef.Name)
		pod.Spec.NodeName = nodeName
		PopulatePodKeyToNodeMap(pod)
		r.GenericEvents <- event.GenericEvent{Object: pod}
	} else {
		PopulatePodKeyToNodeMap(pod)
	}
	return nil
}
