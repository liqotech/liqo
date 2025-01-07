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
	"slices"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	"github.com/liqotech/liqo/apis/networking/v1beta1/firewall"
	"github.com/liqotech/liqo/pkg/fabric"
	"github.com/liqotech/liqo/pkg/utils/resource"
)

// generateGatewayMasqueradeBypassFirewallConfigurationName generates the name of the firewall configuration for the given node.
func generateFirewallConfigurationName(nodeName string) string {
	return fmt.Sprintf("%s-gw-masquerade-bypass", nodeName)
}

func getNodeFromFirewallConfigurationName(name string) string {
	return name[:len(name)-len("-gw-masquerade-bypass")]
}

func enforceFirewallPodPresence(ctx context.Context, cl client.Client, scheme *runtime.Scheme,
	opts *Options, pod *corev1.Pod) (controllerutil.OperationResult, error) {
	if pod.Status.PodIP == "" {
		return "", nil
	}

	internalnode := &networkingv1beta1.InternalNode{}
	if err := cl.Get(ctx, client.ObjectKey{Name: pod.Spec.NodeName}, internalnode); err != nil {
		return "", err
	}

	fwcfg := &networkingv1beta1.FirewallConfiguration{
		ObjectMeta: metav1.ObjectMeta{Name: generateFirewallConfigurationName(pod.Spec.NodeName), Namespace: opts.Namespace},
	}

	op, err := resource.CreateOrUpdate(ctx, cl, fwcfg, forgeFirewallPodUpdateFunction(internalnode, fwcfg, pod, scheme, opts.GenevePort))

	return op, err
}

func enforceFirewallPodAbsence(ctx context.Context, cl client.Client, opts *Options, pod *corev1.Pod) error {
	nodeName, err := GetPodNodeFromMap(client.ObjectKeyFromObject(pod))
	if err != nil {
		return err
	}
	if nodeName == "" {
		return fmt.Errorf("unable to get node name from pod %s/%s", pod.GetNamespace(), pod.GetName())
	}
	fwcfg := networkingv1beta1.FirewallConfiguration{ObjectMeta: metav1.ObjectMeta{
		Name: generateFirewallConfigurationName(nodeName), Namespace: opts.Namespace,
	}}
	if err := cl.Get(ctx, client.ObjectKeyFromObject(&fwcfg), &fwcfg); err != nil {
		return fmt.Errorf("unable to get firewall configuration %s: %w", fwcfg.GetName(), err)
	}

	if _, err := resource.CreateOrUpdate(ctx, cl, &fwcfg, forgeFirewallPodDeleteFunction(pod, &fwcfg)); err != nil {
		return fmt.Errorf("unable to update firewall configuration %s: %w", fwcfg.GetName(), err)
	}

	klog.Infof("Removed gw-masquerade-bypass for pod %s/%s from firewallconfiguration %s", pod.GetNamespace(), pod.GetName(), fwcfg.GetName())

	DeletePodKeyFromMap(client.ObjectKeyFromObject(pod))

	if len(fwcfg.Spec.Table.Chains[0].Rules.NatRules) == 0 {
		if err := cl.Delete(ctx, &fwcfg); err != nil {
			return err
		}
	}

	return nil
}

func forgeFirewallPodUpdateFunction(internalnode *networkingv1beta1.InternalNode,
	fwcfg *networkingv1beta1.FirewallConfiguration, pod *corev1.Pod, scheme *runtime.Scheme, genevePort uint16) controllerutil.MutateFn {
	return func() error {
		if err := controllerutil.SetOwnerReference(internalnode, fwcfg, scheme); err != nil {
			return err
		}

		fwcfg.SetLabels(fabric.ForgeFirewallTargetLabelsSingleNode(internalnode.GetName()))
		fwcfg.Labels[GatewayMasqueradeBypassLabel] = GatewayMasqueradeBypassLabelValue

		fwcfg.Spec.Table.Name = ptr.To(generateFirewallConfigurationName(internalnode.Name))
		fwcfg.Spec.Table.Family = ptr.To(firewall.TableFamilyIPv4)

		if fwcfg.Spec.Table.Chains == nil || len(fwcfg.Spec.Table.Chains) == 0 {
			fwcfg.Spec.Table.Chains = []firewall.Chain{{
				Rules: firewall.RulesSet{
					NatRules: []firewall.NatRule{},
				},
			}}
		}
		setFirewallPodChain(&fwcfg.Spec.Table.Chains[0])

		rules := &fwcfg.Spec.Table.Chains[0].Rules.NatRules

		if rule, exists := rulesContainsPod(pod, *rules); exists {
			updatePodToFw(pod, rule, genevePort)
		} else {
			addPodToFw(pod, rules, genevePort)
		}
		return nil
	}
}

func setFirewallPodChain(chain *firewall.Chain) {
	chain.Name = ptr.To(PrePostroutingChainName)
	chain.Type = ptr.To(firewall.ChainTypeNAT)
	chain.Hook = ptr.To(firewall.ChainHookPostrouting)
	chain.Policy = ptr.To(firewall.ChainPolicyAccept)
	chain.Priority = ptr.To(firewall.ChainPriorityNATSource - 1)
}

func forgeFirewallPodDeleteFunction(pod *corev1.Pod, fwcfg *networkingv1beta1.FirewallConfiguration) controllerutil.MutateFn {
	return func() error {
		if fwcfg.Spec.Table.Chains == nil || len(fwcfg.Spec.Table.Chains) != 1 {
			return fmt.Errorf("firewall configuration table should contain only one chain, it contains %d", len(fwcfg.Spec.Table.Chains))
		}
		if _, exists := rulesContainsPod(pod, fwcfg.Spec.Table.Chains[0].Rules.NatRules); exists {
			fwcfg.Spec.Table.Chains[0].Rules.NatRules = slices.DeleteFunc(fwcfg.Spec.Table.Chains[0].Rules.NatRules, func(r firewall.NatRule) bool {
				return ruleMatchPod(pod, &r)
			})
		}
		return nil
	}
}

func ruleMatchPod(pod *corev1.Pod, rule *firewall.NatRule) bool {
	return rule.TargetRef.Name == pod.Name &&
		rule.TargetRef.Namespace == pod.Namespace &&
		rule.TargetRef.Kind == "Pod"
}

func rulesContainsPod(pod *corev1.Pod, rules []firewall.NatRule) (*firewall.NatRule, bool) {
	for i := range rules {
		if ruleMatchPod(pod, &rules[i]) {
			return &rules[i], true
		}
	}
	return nil, false
}

func addPodToFw(pod *corev1.Pod, rules *[]firewall.NatRule, genevePort uint16) {
	*rules = append(*rules, firewall.NatRule{
		Name:    &pod.Name,
		NatType: firewall.NatTypeSource,
		To:      ptr.To(pod.Status.PodIP),
		Match: []firewall.Match{
			{
				Op: firewall.MatchOperationEq,
				IP: &firewall.MatchIP{
					Value:    pod.Status.PodIP,
					Position: firewall.MatchPositionSrc,
				},
				Port: &firewall.MatchPort{
					Position: firewall.MatchPositionDst,
					Value:    fmt.Sprintf("%d", genevePort),
				},
				Proto: &firewall.MatchProto{
					Value: firewall.L4ProtoUDP,
				},
			},
		},
		TargetRef: &corev1.ObjectReference{Name: pod.Name, Namespace: pod.Namespace, Kind: pod.Kind},
	})
}

func updatePodToFw(pod *corev1.Pod, rule *firewall.NatRule, genevePort uint16) {
	rule.Name = ptr.To(pod.Name)
	rule.NatType = firewall.NatTypeSource
	rule.To = ptr.To(pod.Status.PodIP)
	rule.Match = []firewall.Match{
		{
			Op: firewall.MatchOperationEq,
			IP: &firewall.MatchIP{
				Value:    pod.Status.PodIP,
				Position: firewall.MatchPositionSrc,
			},
			Port: &firewall.MatchPort{
				Position: firewall.MatchPositionDst,
				Value:    fmt.Sprintf("%d", genevePort),
			},
			Proto: &firewall.MatchProto{
				Value: firewall.L4ProtoUDP,
			},
		},
	}
	rule.TargetRef = &corev1.ObjectReference{Name: pod.Name, Namespace: pod.Namespace, Kind: pod.Kind}
}
