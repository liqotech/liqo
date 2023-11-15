// Copyright 2019-2023 The Liqo Authors
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

	"sigs.k8s.io/controller-runtime/pkg/client"

	networkingv1alpha1 "github.com/liqotech/liqo/apis/networking/v1alpha1"
	firewallapi "github.com/liqotech/liqo/apis/networking/v1alpha1/firewall"
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

func checkUniqueTableName(ctx context.Context, cl client.Client, currentFwcfg *networkingv1alpha1.FirewallConfiguration) error {
	if currentFwcfg == nil {
		return fmt.Errorf("firewallconfiguration is nil")
	}
	if currentFwcfg.Spec.Table.Name == nil {
		return fmt.Errorf("table name is nil")
	}
	currentTableName := currentFwcfg.Spec.Table.Name

	fwcfglist := networkingv1alpha1.FirewallConfigurationList{}
	if err := cl.List(ctx, &fwcfglist); err != nil {
		return err
	}
	for i := range fwcfglist.Items {
		if fwcfglist.Items[i].UID == currentFwcfg.UID {
			continue
		}
		fwcfg := fwcfglist.Items[i]
		tableName := fwcfg.Spec.Table.Name
		if tableName == nil {
			return fmt.Errorf("table name is nil")
		}
		if *tableName == *currentTableName {
			return fmt.Errorf("table name %v is duplicated", *tableName)
		}
	}
	return nil
}
