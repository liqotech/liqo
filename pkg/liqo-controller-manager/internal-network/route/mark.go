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
	"math"
	"strconv"
	"sync"

	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	networkingv1alpha1 "github.com/liqotech/liqo/apis/networking/v1alpha1"
	"github.com/liqotech/liqo/apis/networking/v1alpha1/firewall"
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
		routecfglist, err := getters.ListRouteConfigurationsByLabel(ctx, cl, options.Namespace,
			labels.SelectorFromSet(gateway.ForgeRouteInternalTargetLabels()),
		)
		utilruntime.Must(err)

		firewallcfglist, err := getters.ListFirewallConfigurationsByLabel(ctx, cl, options.Namespace,
			labels.SelectorFromSet(gateway.ForgeFirewallInternalTargetLabels()),
		)
		utilruntime.Must(err)

		RegisterMarksFromRouteconfigurations(routecfglist.Items)

		RegisterMarksFromFirewallconfigurations(firewallcfglist.Items)
	})
}

// RegisterMarksFromRouteconfigurations registers the marks used in the routeconfigurations.
// It fullfills the marks map with the marks used in the routeconfigurations
// and the marksreverse map with the nodename and the mark used.
func RegisterMarksFromRouteconfigurations(routecfgs []networkingv1alpha1.RouteConfiguration) {
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
func RegisterMarksFromFirewallconfigurations(fwcfgs []networkingv1alpha1.FirewallConfiguration) {
	for i := range fwcfgs {
		chains := fwcfgs[i].Spec.Table.Chains
		for j := range chains {
			filterrules := chains[j].Rules.FilterRules
			for k := range filterrules {
				filterrule := filterrules[k]
				if filterrule.Action == firewall.ActionCtMark {
					mark, err := strconv.Atoi(*filterrule.Value)
					utilruntime.Must(err)
					marks[mark] = struct{}{}
					marksreverse[*filterrule.Name] = mark
				}
			}
		}
	}
}

// StartMarkTransaction returns a new mark to be used in the firewall configuration.
func StartMarkTransaction(nodename string) int {
	m.Lock()
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
	return nextmark
}

// EndMarkTransaction marks the transaction as successful or not.
func EndMarkTransaction(nodename string, err error) {
	defer m.Unlock()
	if err == nil {
		marks[nextmark] = struct{}{}
		marksreverse[nodename] = nextmark
	}
}

// FreeMarkTransaction frees the mark used in the firewall configuration.
func FreeMarkTransaction(nodename string) {
	defer m.Unlock()
	delete(marks, marksreverse[nodename])
	delete(marksreverse, nodename)
}
