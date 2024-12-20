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

package info

import (
	"context"
	"fmt"

	"github.com/pterm/pterm"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	"github.com/liqotech/liqo/pkg/liqoctl/test/network/client"
	cidrutils "github.com/liqotech/liqo/pkg/utils/cidr"
)

// Info prints the configurations of the clusters.
func Info(ctx context.Context, cl *client.Client, table *pterm.TablePrinter) error {
	pterm.Println("")
	pterm.NewStyle(pterm.FgMagenta, pterm.Bold).Printfln("Cluster %q configurations", cl.ConsumerName)
	if err := PrintConfigurations(ctx, cl.Consumer, table); err != nil {
		return fmt.Errorf("error printing configurations for consumer: %w", err)
	}
	for k := range cl.Providers {
		pterm.Println("")
		pterm.NewStyle(pterm.FgMagenta, pterm.Bold).Printfln("Cluster %q configurations", k)
		if err := PrintConfigurations(ctx, cl.Providers[k], table); err != nil {
			return fmt.Errorf("error printing configuration for provider %q: %w", k, err)
		}
	}
	pterm.Println("")
	return nil
}

// ForgeTableData creates a table data.
func ForgeTableData() pterm.TableData {
	return pterm.TableData{
		{"Cluster", "Pod CIDR", "Pod CIDR remap", "External CIDR", "External CIDR remap"},
	}
}

// PrintConfigurations prints the configurations of the clusters.
func PrintConfigurations(ctx context.Context, cl ctrlclient.Client, table *pterm.TablePrinter) error {
	td := ForgeTableData()
	cfglist := networkingv1beta1.ConfigurationList{}
	if err := cl.List(ctx, &cfglist); err != nil {
		return err
	}
	td = AppendLocalConfigurationTableData(&cfglist.Items[0], td)
	for i := range cfglist.Items {
		td = AppendRemoteConfigurationTableData(&cfglist.Items[i], td)
	}
	if err := table.WithData(td).Render(); err != nil {
		return err
	}

	return nil
}

// AppendLocalConfigurationTableData appends the local configuration to the table data.
func AppendLocalConfigurationTableData(cfg *networkingv1beta1.Configuration, td pterm.TableData) pterm.TableData {
	return append(td, []string{
		"local",
		cidrutils.GetPrimary(cfg.Spec.Local.CIDR.Pod).String(), "N/R",
		cidrutils.GetPrimary(cfg.Spec.Local.CIDR.External).String(), "N/R",
	})
}

// AppendRemoteConfigurationTableData appends the remote configuration to the table data.
func AppendRemoteConfigurationTableData(cfg *networkingv1beta1.Configuration, td pterm.TableData) pterm.TableData {
	return append(td, []string{
		cfg.Name,
		cidrutils.GetPrimary(cfg.Spec.Remote.CIDR.Pod).String(), cidrutils.GetPrimary(cfg.Status.Remote.CIDR.Pod).String(),
		cidrutils.GetPrimary(cfg.Spec.Remote.CIDR.External).String(), cidrutils.GetPrimary(cfg.Status.Remote.CIDR.External).String(),
	})
}
