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

package gwmasqbypass

import (
	"context"
	"fmt"

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
	"github.com/liqotech/liqo/apis/networking/v1beta1/firewall"
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

// CheckLeftoverRules lists all currently existing firewallconfigurations and adds their
// pod to the queue if its pod does not exist anymore.
// This will detect rules that exist with no
// corresponding pod; these rules need to be deleted. We only need to
// do this once on startup, because in steady-state these are detected (but
// some stragglers could have been left behind if this controller
// reboots).
// It also populates podKeyToNode map with existing pods nodename.
func (r *PodReconciler) CheckLeftoverRules(ctx context.Context) error {
	fwcfglist, err := getters.ListFirewallConfigurationsByLabel(ctx, r.Client, labels.SelectorFromSet(labels.Set{
		GatewayMasqueradeBypassLabel: GatewayMasqueradeBypassLabelValue,
	}))
	if err != nil {
		return err
	}
	return r.processFirewallConfiguration(ctx, fwcfglist)
}

func (r *PodReconciler) processFirewallConfiguration(ctx context.Context, fwcfglist *networkingv1beta1.FirewallConfigurationList) error {
	if fwcfglist == nil {
		return fmt.Errorf("firewall configuration list is nil")
	}

	for i := range fwcfglist.Items {
		if len(fwcfglist.Items[i].Spec.Table.Chains) != 1 {
			return fmt.Errorf("firewall configuration table should contain only one chain")
		}

		chain := fwcfglist.Items[i].Spec.Table.Chains[0]

		if chain.Type == nil || *chain.Type != firewall.ChainTypeNAT {
			return fmt.Errorf("firewall configuration table chain should be of type NAT, not %s", *chain.Type)
		}

		if err := r.processRules(ctx, &chain, getNodeFromFirewallConfigurationName(fwcfglist.Items[i].Name)); err != nil {
			return fmt.Errorf("unable to process rules: %w", err)
		}
	}

	return nil
}

func (r *PodReconciler) processRules(ctx context.Context, chain *firewall.Chain, nodename string) error {
	for i := range chain.Rules.NatRules {
		gwPodName := chain.Rules.NatRules[i].TargetRef.Name
		gwPodNamespace := chain.Rules.NatRules[i].TargetRef.Namespace
		pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: gwPodName, Namespace: gwPodNamespace}}
		if err := r.Get(ctx, client.ObjectKeyFromObject(pod), pod); err != nil {
			if !apierrors.IsNotFound(err) {
				return err
			}
			klog.Infof("pod %s not found, adding to queue", chain.Rules.NatRules[i].TargetRef.String())
			pod.Spec.NodeName = nodename
			PopulatePodKeyToNodeMap(pod)
			r.GenericEvents <- event.GenericEvent{Object: pod}
		} else {
			PopulatePodKeyToNodeMap(pod)
		}
	}
	return nil
}
