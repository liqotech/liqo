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

package firewall

import (
	"github.com/google/nftables"
	"k8s.io/klog/v2"

	firewallapi "github.com/liqotech/liqo/apis/networking/v1beta1/firewall"
)

func addTable(nftconn *nftables.Conn, table *firewallapi.Table) *nftables.Table {
	nftTable := &nftables.Table{}
	setTableName(nftTable, *table.Name)
	setTableFamily(nftTable, *table.Family)
	nftconn.AddTable(nftTable)
	return nftTable
}

func delTable(nftconn *nftables.Conn, table *firewallapi.Table) {
	nftTable := &nftables.Table{}
	setTableName(nftTable, *table.Name)
	nftconn.DelTable(nftTable)
}

func getTableFamily(family firewallapi.TableFamily) nftables.TableFamily {
	switch family {
	case firewallapi.TableFamilyIPv4:
		return nftables.TableFamilyIPv4
	case firewallapi.TableFamilyIPv6:
		return nftables.TableFamilyIPv6
	case firewallapi.TableFamilyINet:
		return nftables.TableFamilyINet
	case firewallapi.TableFamilyARP:
		return nftables.TableFamilyARP
	case firewallapi.TableFamilyBridge:
		return nftables.TableFamilyBridge
	case firewallapi.TableFamilyNetdev:
		return nftables.TableFamilyNetdev
	default:
		return nftables.TableFamily(0)
	}
}

// cleanTable removes all the chains and rules that are not present in the firewall configuration or that have been modified.
func cleanTable(nftconn *nftables.Conn, table *firewallapi.Table) error {
	nftChains, err := nftconn.ListChainsOfTableFamily(getTableFamily(*table.Family))
	if err != nil {
		return err
	}
	// Cycle on chains that are applied on nftable.
	for i := range nftChains {
		// We need to filter the chains which are contained in the table.
		if nftChains[i].Table.Name != *table.Name {
			continue
		}
		// If the chain is outdated we need to delete it. Otherwise we get the index of the chain and check the rules inside it.
		outdated, chainIndex := isChainOutdated(nftChains[i], table.Chains)
		if outdated {
			klog.V(2).Infof("deleting chain %s", nftChains[i].Name)
			nftconn.DelChain(nftChains[i])
			continue
		}
		// If the chain is not outdated we need to check the rules inside it.
		if err := cleanChain(nftconn, &table.Chains[chainIndex], nftChains[i]); err != nil {
			return err
		}
	}
	return nil
}

func setTableName(table *nftables.Table, name string) {
	table.Name = name
}

func setTableFamily(table *nftables.Table, family firewallapi.TableFamily) {
	switch family {
	case firewallapi.TableFamilyIPv4:
		table.Family = nftables.TableFamilyIPv4
	case firewallapi.TableFamilyIPv6:
		table.Family = nftables.TableFamilyIPv6
	case firewallapi.TableFamilyINet:
		table.Family = nftables.TableFamilyINet
	case firewallapi.TableFamilyARP:
		table.Family = nftables.TableFamilyARP
	case firewallapi.TableFamilyBridge:
		table.Family = nftables.TableFamilyBridge
	case firewallapi.TableFamilyNetdev:
		table.Family = nftables.TableFamilyNetdev
	}
}
