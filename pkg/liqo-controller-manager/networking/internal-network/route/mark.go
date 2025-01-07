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
	"math"
	"strconv"
	"sync"

	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	"github.com/liqotech/liqo/apis/networking/v1beta1/firewall"
	"github.com/liqotech/liqo/pkg/gateway"
	"github.com/liqotech/liqo/pkg/utils/getters"
)

var (
	nextmark     = 1
	marks        = make(map[int]interface{})
	marksreverse = make(map[string]int)
	m            sync.Mutex
	once         sync.Once
)

// +kubebuilder:rbac:groups=networking.liqo.io,resources=routeconfigurations,verbs=get;list;watch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=firewallconfigurations,verbs=get;list;watch

// InitMark initializes the marks map with the marks already used in the cluster.
func InitMark(ctx context.Context, cl client.Client, options *Options) {
	once.Do(func() {
		routecfglist, err := getters.ListRouteConfigurationsInNamespaceByLabel(ctx, cl, options.Namespace,
			labels.SelectorFromSet(gateway.ForgeRouteInternalTargetLabels()),
		)
		utilruntime.Must(err)

		firewallcfglist, err := getters.ListFirewallConfigurationsInNamespaceByLabel(ctx, cl, options.Namespace,
			labels.SelectorFromSet(gateway.ForgeFirewallInternalTargetLabels()),
		)
		utilruntime.Must(err)

		RegisterMarksFromRouteconfigurations(routecfglist.Items)

		utilruntime.Must(RegisterMarksFromFirewallconfigurations(firewallcfglist.Items))

		klog.Infof("Initialized marks: %v", marks)
	})
}

// RegisterMarksFromRouteconfigurations registers the marks used in the routeconfigurations.
// It fullfills the marks map with the marks used in the routeconfigurations
// and the marksreverse map with the nodename and the mark used.
func RegisterMarksFromRouteconfigurations(routecfgs []networkingv1beta1.RouteConfiguration) {
	for i := range routecfgs {
		rules := routecfgs[i].Spec.Table.Rules
		for j := range rules {
			rule := rules[j]
			if rule.FwMark != nil {
				marks[*rule.FwMark] = struct{}{}
				marksreverse[rule.TargetRef.Name] = *rule.FwMark
			}
		}
	}
}

// RegisterMarksFromFirewallconfigurations registers the marks used in the firewallconfigurations.
// It fullfills the marks map with the marks used in the firewallconfigurations
// and the marksreverse map with the nodename and the mark used.
func RegisterMarksFromFirewallconfigurations(fwcfgs []networkingv1beta1.FirewallConfiguration) error {
	for i := range fwcfgs {
		chains := fwcfgs[i].Spec.Table.Chains
		for j := range chains {
			filterrules := chains[j].Rules.FilterRules
			for k := range filterrules {
				filterrule := filterrules[k]
				if filterrule.Action == firewall.ActionCtMark {
					mark, err := strconv.Atoi(*filterrule.Value)
					if err != nil {
						return err
					}
					marks[mark] = struct{}{}
					marksreverse[*filterrule.Name] = mark
				}
			}
		}
	}
	return nil
}

// StartMarkTransaction starts a transaction to assign a mark to a node.
func StartMarkTransaction() {
	m.Lock()
}

// AssignMark assigns a mark to a node.
func AssignMark(nodename string) int {
	found := false
	if mark, ok := marksreverse[nodename]; ok {
		return mark
	}

	for !found {
		if _, ok := marks[nextmark]; !ok {
			found = true
		} else {
			if nextmark == math.MaxInt {
				nextmark = 1
			} else {
				nextmark++
			}
		}
	}

	marks[nextmark] = struct{}{}
	marksreverse[nodename] = nextmark

	klog.Infof("Mark %d assigned to node %s", nextmark, nodename)
	return nextmark
}

// EndMarkTransaction marks the transaction.
func EndMarkTransaction() {
	m.Unlock()
}

// FreeMark frees the mark used in the firewall configuration.
func FreeMark(nodename string) {
	delete(marks, marksreverse[nodename])
	delete(marksreverse, nodename)
}
