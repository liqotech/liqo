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

package firewallconfiguration

import (
	"context"
	"fmt"
	"maps"

	"sigs.k8s.io/controller-runtime/pkg/client"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	firewallapi "github.com/liqotech/liqo/apis/networking/v1beta1/firewall"
)

func checkUniqueChainName(chains []firewallapi.Chain) error {
	names := map[string]interface{}{}
	for i := range chains {
		name := chains[i].Name
		if name == nil {
			return fmt.Errorf("chain name is nil")
		}
		if _, ok := names[*name]; ok {
			return fmt.Errorf("chain name %v is duplicated", *name)
		}
		names[*name] = nil
	}
	return nil
}

// checkImmutableTableName checks if the table name is immutable.
func checkImmutableTableName(fwcfg, oldFwcfg *networkingv1beta1.FirewallConfiguration) error {
	if fwcfg.Spec.Table.Name == nil || oldFwcfg.Spec.Table.Name == nil {
		return fmt.Errorf("table name is nil")
	}
	if *oldFwcfg.Spec.Table.Name != *fwcfg.Spec.Table.Name {
		return fmt.Errorf("table name is immutable")
	}
	return nil
}

func checkUniqueTableName(ctx context.Context, cl client.Client, currentFwcfg *networkingv1beta1.FirewallConfiguration) error {
	if currentFwcfg == nil {
		return fmt.Errorf("firewallconfiguration is nil")
	}
	if currentFwcfg.Spec.Table.Name == nil {
		return fmt.Errorf("table name is nil")
	}
	currentTableName := currentFwcfg.Spec.Table.Name

	fwcfglist := networkingv1beta1.FirewallConfigurationList{}
	if err := cl.List(ctx, &fwcfglist); err != nil {
		return err
	}

	for i := range fwcfglist.Items {
		if fwcfglist.Items[i].UID == currentFwcfg.UID {
			continue
		}
		if fwcfglist.Items[i].Spec.Table.Name == nil || currentFwcfg.Spec.Table.Name == nil {
			return fmt.Errorf("table name is nil")
		}
		if *fwcfglist.Items[i].Spec.Table.Name == *currentFwcfg.Spec.Table.Name &&
			maps.Equal(currentFwcfg.GetLabels(), fwcfglist.Items[i].GetLabels()) {
			return fmt.Errorf("table name %s with labels %s already used",
				*currentTableName, currentFwcfg.GetLabels())
		}
	}
	return nil
}
