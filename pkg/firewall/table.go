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

package firewall

import (
	"github.com/google/nftables"

	firewallapi "github.com/liqotech/liqo/apis/networking/v1alpha1/firewall"
)

func addTable(nftconn *nftables.Conn, table *firewallapi.Table) {
	nftTable := &nftables.Table{}
	setTableName(nftTable, *table.Name)
	setTableFamily(nftTable, *table.Family)
	nftconn.AddTable(nftTable)
}

func delTable(nftconn *nftables.Conn, table *firewallapi.Table) {
	nftTable := &nftables.Table{}
	setTableName(nftTable, *table.Name)
	nftconn.DelTable(nftTable)
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
