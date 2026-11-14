// Copyright 2019-2026 The Liqo Authors
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

//go:build linux

package firewall

import (
	"testing"

	"github.com/google/nftables"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/ptr"

	firewallapi "github.com/liqotech/liqo/apis/networking/v1beta1/firewall"
	firewallutils "github.com/liqotech/liqo/pkg/firewall/utils"
)

func TestGetSetDataType(t *testing.T) {
	tests := []struct {
		name         string
		in           *firewallapi.SetDataType
		wantType     nftables.SetDatatype
		wantInterval bool
		wantErr      bool
	}{
		{name: "nil", in: nil, wantType: nftables.SetDatatype{}},
		{name: "integer", in: ptr.To(firewallapi.SetDataTypeInteger), wantType: nftables.TypeInteger},
		{name: "ipv4_addr", in: ptr.To(firewallapi.SetDataTypeIPAddr), wantType: nftables.TypeIPAddr},
		{name: "ipv4_cidr", in: ptr.To(firewallapi.SetDataTypeIPCIDR), wantType: nftables.TypeIPAddr, wantInterval: true},
		{name: "unknown", in: ptr.To(firewallapi.SetDataType("foo")), wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, interval, err := getSetDataType(tt.in)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantType, got)
			assert.Equal(t, tt.wantInterval, interval)
		})
	}
}

func TestGenSetElements(t *testing.T) {
	t.Run("ipv4_addr keys without data", func(t *testing.T) {
		elems, err := genSetElements(&firewallapi.Set{
			Name:     "s",
			KeyType:  firewallapi.SetDataTypeIPAddr,
			Elements: []firewallapi.SetElement{{Key: "10.0.0.1"}, {Key: "10.0.0.2"}},
		})
		require.NoError(t, err)
		require.Len(t, elems, 2)
		assert.Equal(t, []byte{10, 0, 0, 1}, elems[0].Key)
		assert.Equal(t, []byte{10, 0, 0, 2}, elems[1].Key)
		assert.False(t, elems[0].IntervalEnd)
		assert.False(t, elems[1].IntervalEnd)
	})

	t.Run("ipv4_cidr keys generate start and exclusive end elements", func(t *testing.T) {
		elems, err := genSetElements(&firewallapi.Set{
			Name:     "s",
			KeyType:  firewallapi.SetDataTypeIPCIDR,
			Elements: []firewallapi.SetElement{{Key: "10.0.1.0/24"}, {Key: "10.0.5.7/32"}},
		})
		require.NoError(t, err)
		require.Len(t, elems, 4)
		assert.Equal(t, nftables.SetElement{Key: []byte{10, 0, 1, 0}, Val: []byte{}}, elems[0])
		assert.Equal(t, nftables.SetElement{Key: []byte{10, 0, 2, 0}, Val: []byte{}, IntervalEnd: true}, elems[1])
		assert.Equal(t, nftables.SetElement{Key: []byte{10, 0, 5, 7}, Val: []byte{}}, elems[2])
		assert.Equal(t, nftables.SetElement{Key: []byte{10, 0, 5, 8}, Val: []byte{}, IntervalEnd: true}, elems[3])
	})

	t.Run("ipv4_cidr reaching the end of the address space has no end element", func(t *testing.T) {
		elems, err := genSetElements(&firewallapi.Set{
			Name:     "s",
			KeyType:  firewallapi.SetDataTypeIPCIDR,
			Elements: []firewallapi.SetElement{{Key: "255.255.255.0/24"}},
		})
		require.NoError(t, err)
		require.Len(t, elems, 1)
		assert.Equal(t, []byte{255, 255, 255, 0}, elems[0].Key)
	})

	t.Run("map with data", func(t *testing.T) {
		elems, err := genSetElements(&firewallapi.Set{
			Name:     "m",
			KeyType:  firewallapi.SetDataTypeIPAddr,
			DataType: ptr.To(firewallapi.SetDataTypeIPAddr),
			Elements: []firewallapi.SetElement{{Key: "10.0.0.1", Data: ptr.To("192.168.0.1")}},
		})
		require.NoError(t, err)
		require.Len(t, elems, 1)
		assert.Equal(t, []byte{10, 0, 0, 1}, elems[0].Key)
		assert.Equal(t, []byte{192, 168, 0, 1}, elems[0].Val)
	})

	t.Run("invalid key", func(t *testing.T) {
		_, err := genSetElements(&firewallapi.Set{
			Name:     "s",
			KeyType:  firewallapi.SetDataTypeIPAddr,
			Elements: []firewallapi.SetElement{{Key: "not-an-ip"}},
		})
		assert.Error(t, err)
	})

	t.Run("data without data type", func(t *testing.T) {
		_, err := genSetElements(&firewallapi.Set{
			Name:     "s",
			KeyType:  firewallapi.SetDataTypeIPAddr,
			Elements: []firewallapi.SetElement{{Key: "10.0.0.1", Data: ptr.To("10.0.0.2")}},
		})
		assert.Error(t, err)
	})

	t.Run("no elements", func(t *testing.T) {
		elems, err := genSetElements(&firewallapi.Set{Name: "s", KeyType: firewallapi.SetDataTypeIPAddr})
		require.NoError(t, err)
		assert.Empty(t, elems)
	})
}

func TestIsSetOutdated(t *testing.T) {
	a := nftables.SetElement{Key: []byte{10, 0, 0, 1}}
	b := nftables.SetElement{Key: []byte{10, 0, 0, 2}}
	c := nftables.SetElement{Key: []byte{10, 0, 0, 3}}
	withVal := func(e nftables.SetElement, v byte) nftables.SetElement {
		e.Val = []byte{v}
		return e
	}

	tests := []struct {
		name     string
		existing []nftables.SetElement
		wanted   []nftables.SetElement
		outdated bool
	}{
		{name: "both empty", outdated: false},
		{name: "same elements", existing: []nftables.SetElement{a, b}, wanted: []nftables.SetElement{a, b}, outdated: false},
		{name: "same elements different order", existing: []nftables.SetElement{b, a}, wanted: []nftables.SetElement{a, b}, outdated: false},
		{name: "element added", existing: []nftables.SetElement{a}, wanted: []nftables.SetElement{a, b}, outdated: true},
		{name: "element removed", existing: []nftables.SetElement{a, b}, wanted: []nftables.SetElement{a}, outdated: true},
		{name: "element replaced", existing: []nftables.SetElement{a, b}, wanted: []nftables.SetElement{a, c}, outdated: true},
		{name: "value changed", existing: []nftables.SetElement{withVal(a, 1)}, wanted: []nftables.SetElement{withVal(a, 2)}, outdated: true},
		{name: "interval end differs from interval start with same key",
			existing: []nftables.SetElement{{Key: a.Key, IntervalEnd: true}, b},
			wanted:   []nftables.SetElement{{Key: a.Key}, b}, outdated: true},
		{name: "adjacent intervals sharing a key",
			existing: []nftables.SetElement{{Key: b.Key, IntervalEnd: true}, {Key: a.Key}, {Key: c.Key, IntervalEnd: true}, {Key: b.Key}},
			wanted:   []nftables.SetElement{{Key: a.Key}, {Key: b.Key, IntervalEnd: true}, {Key: b.Key}, {Key: c.Key, IntervalEnd: true}},
			outdated: false},
		{name: "nil and empty value are equal", existing: []nftables.SetElement{{Key: a.Key, Val: nil}},
			wanted: []nftables.SetElement{{Key: a.Key, Val: []byte{}}}, outdated: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.outdated, isSetOutdated(tt.existing, tt.wanted))
		})
	}
}

// The tests below interact with the kernel nftables subsystem. They require CAP_NET_ADMIN
// (e.g., run them as root, in the privileged liqo-test container, or inside "unshare -rn").

const testTableName = "liqo-test-sets"

func newTestConn(t *testing.T) *nftables.Conn {
	t.Helper()
	conn, err := nftables.New()
	if err != nil {
		t.Skipf("nftables not available: %v", err)
	}
	if _, err := conn.ListTables(); err != nil {
		t.Skipf("nftables not available (missing CAP_NET_ADMIN?): %v", err)
	}
	t.Cleanup(func() {
		conn.DelTable(&nftables.Table{Name: testTableName, Family: nftables.TableFamilyIPv4})
		_ = conn.Flush()
	})
	return conn
}

// applyTable mimics the reconciliation flow of the FirewallConfigurationReconciler.
func applyTable(t *testing.T, conn *nftables.Conn, table *firewallapi.Table) {
	t.Helper()
	require.NoError(t, cleanTable(conn, table))
	require.NoError(t, conn.Flush())
	nftTable := addTable(conn, table)
	require.NoError(t, addSets(conn, table.Sets, nftTable))
	_, err := addChains(conn, table.Chains, nftTable)
	require.NoError(t, err)
	require.NoError(t, conn.Flush())
}

func getTestSet(t *testing.T, conn *nftables.Conn, name string) (*nftables.Set, []nftables.SetElement) {
	t.Helper()
	set, err := conn.GetSetByName(&nftables.Table{Name: testTableName, Family: nftables.TableFamilyIPv4}, name)
	if err != nil {
		return nil, nil
	}
	elems, err := conn.GetSetElements(set)
	require.NoError(t, err)
	return set, elems
}

func listTestSetNames(t *testing.T, conn *nftables.Conn) []string {
	t.Helper()
	sets, err := conn.GetSets(&nftables.Table{Name: testTableName, Family: nftables.TableFamilyIPv4})
	require.NoError(t, err)
	names := make([]string, 0, len(sets))
	for _, s := range sets {
		names = append(names, s.Name)
	}
	return names
}

func elementKeys(elems []nftables.SetElement) [][]byte {
	keys := make([][]byte, 0, len(elems))
	for i := range elems {
		keys = append(keys, elems[i].Key)
	}
	return keys
}

func forgeTestTable(sets []firewallapi.Set, matchValue string, op firewallapi.MatchOperation) *firewallapi.Table {
	return &firewallapi.Table{
		Name:   ptr.To(testTableName),
		Family: ptr.To(firewallapi.TableFamilyIPv4),
		Sets:   sets,
		Chains: []firewallapi.Chain{{
			Name:     ptr.To("forward"),
			Type:     firewallapi.ChainTypeFilter,
			Hook:     ptr.To(firewallapi.ChainHookForward),
			Priority: ptr.To(firewallapi.ChainPriorityFilter),
			Policy:   ptr.To(firewallapi.ChainPolicyAccept),
			Rules: firewallapi.RulesSet{FilterRules: []firewallapi.FilterRule{{
				Name: ptr.To("drop-set"),
				Match: []firewallapi.Match{{
					Op: op,
					IP: &firewallapi.MatchIP{Value: matchValue, Position: firewallapi.MatchPositionSrc},
				}},
				Action: firewallapi.ActionDrop,
			}}},
		}},
	}
}

func TestSetsKernel(t *testing.T) {
	conn := newTestConn(t)

	cidrSet := firewallapi.Set{
		Name:    "blocked",
		KeyType: firewallapi.SetDataTypeIPCIDR,
		Elements: []firewallapi.SetElement{
			{Key: "10.0.1.0/24"},
			{Key: "10.0.5.7/32"},
		},
	}
	addrSet := firewallapi.Set{
		Name:     "hosts",
		KeyType:  firewallapi.SetDataTypeIPAddr,
		Elements: []firewallapi.SetElement{{Key: "192.168.0.1"}, {Key: "192.168.0.2"}},
	}

	t.Run("create sets and rule referencing a set", func(t *testing.T) {
		table := forgeTestTable([]firewallapi.Set{cidrSet, addrSet}, "@blocked", firewallapi.MatchOperationEq)
		applyTable(t, conn, table)

		assert.ElementsMatch(t, []string{"blocked", "hosts"}, listTestSetNames(t, conn))

		set, elems := getTestSet(t, conn, "blocked")
		require.NotNil(t, set)
		assert.True(t, set.Interval)
		assert.ElementsMatch(t, [][]byte{{10, 0, 1, 0}, {10, 0, 2, 0}, {10, 0, 5, 7}, {10, 0, 5, 8}}, elementKeys(elems))

		set, elems = getTestSet(t, conn, "hosts")
		require.NotNil(t, set)
		assert.False(t, set.Interval)
		assert.ElementsMatch(t, [][]byte{{192, 168, 0, 1}, {192, 168, 0, 2}}, elementKeys(elems))
	})

	t.Run("rule read back from kernel is equal to the desired one", func(t *testing.T) {
		table := forgeTestTable([]firewallapi.Set{cidrSet, addrSet}, "@blocked", firewallapi.MatchOperationEq)
		nftTable := &nftables.Table{Name: testTableName, Family: nftables.TableFamilyIPv4}
		rules, err := conn.GetRules(nftTable, &nftables.Chain{Name: "forward", Table: nftTable})
		require.NoError(t, err)
		require.Len(t, rules, 1)

		wrapper := &firewallutils.FilterRuleWrapper{FilterRule: &table.Chains[0].Rules.FilterRules[0]}
		assert.True(t, wrapper.Equal(rules[0]), "rule using a named set must not be considered outdated")

		// Re-applying the same configuration must be idempotent.
		applyTable(t, conn, table)
		rulesAfter, err := conn.GetRules(nftTable, &nftables.Chain{Name: "forward", Table: nftTable})
		require.NoError(t, err)
		require.Len(t, rulesAfter, 1)
		assert.Equal(t, rules[0].Handle, rulesAfter[0].Handle, "rule should not have been recreated")
	})

	t.Run("update set elements", func(t *testing.T) {
		updated := cidrSet.DeepCopy()
		updated.Elements = []firewallapi.SetElement{{Key: "172.16.0.0/16"}}
		table := forgeTestTable([]firewallapi.Set{*updated, addrSet}, "@blocked", firewallapi.MatchOperationEq)
		applyTable(t, conn, table)

		_, elems := getTestSet(t, conn, "blocked")
		assert.ElementsMatch(t, [][]byte{{172, 16, 0, 0}, {172, 17, 0, 0}}, elementKeys(elems))
	})

	t.Run("adjacent intervals are stable across reconciliations", func(t *testing.T) {
		adjacent := cidrSet.DeepCopy()
		adjacent.Elements = []firewallapi.SetElement{{Key: "10.1.0.0/24"}, {Key: "10.1.1.0/24"}}
		table := forgeTestTable([]firewallapi.Set{*adjacent, addrSet}, "@blocked", firewallapi.MatchOperationEq)
		applyTable(t, conn, table)

		nftSet, elems := getTestSet(t, conn, "blocked")
		require.NotNil(t, nftSet)
		wanted, err := genSetElements(adjacent)
		require.NoError(t, err)
		assert.False(t, isSetOutdated(elems, wanted), "existing elements: %+v", elems)

		// Re-applying must succeed and keep the same elements.
		applyTable(t, conn, table)
		_, elemsAfter := getTestSet(t, conn, "blocked")
		assert.False(t, isSetOutdated(elemsAfter, wanted))
	})

	t.Run("switch rule to another set with neq and remove unused set", func(t *testing.T) {
		table := forgeTestTable([]firewallapi.Set{addrSet}, "@hosts", firewallapi.MatchOperationNeq)
		applyTable(t, conn, table)

		assert.ElementsMatch(t, []string{"hosts"}, listTestSetNames(t, conn))

		nftTable := &nftables.Table{Name: testTableName, Family: nftables.TableFamilyIPv4}
		rules, err := conn.GetRules(nftTable, &nftables.Chain{Name: "forward", Table: nftTable})
		require.NoError(t, err)
		require.Len(t, rules, 1)
		wrapper := &firewallutils.FilterRuleWrapper{FilterRule: &table.Chains[0].Rules.FilterRules[0]}
		assert.True(t, wrapper.Equal(rules[0]))
	})

	t.Run("map with ipv4 data", func(t *testing.T) {
		mapSet := firewallapi.Set{
			Name:     "nat_map",
			KeyType:  firewallapi.SetDataTypeIPAddr,
			DataType: ptr.To(firewallapi.SetDataTypeIPAddr),
			Elements: []firewallapi.SetElement{{Key: "10.0.0.1", Data: ptr.To("192.168.0.1")}},
		}
		table := forgeTestTable([]firewallapi.Set{addrSet, mapSet}, "@hosts", firewallapi.MatchOperationNeq)
		applyTable(t, conn, table)

		set, elems := getTestSet(t, conn, "nat_map")
		require.NotNil(t, set)
		assert.True(t, set.IsMap)
		require.Len(t, elems, 1)
		assert.Equal(t, []byte{10, 0, 0, 1}, elems[0].Key)
		assert.Equal(t, []byte{192, 168, 0, 1}, elems[0].Val)
	})

	t.Run("cidr reaching the end of the address space", func(t *testing.T) {
		for _, cidr := range []string{"0.0.0.0/0", "255.255.255.0/24"} {
			last := firewallapi.Set{
				Name:     "last",
				KeyType:  firewallapi.SetDataTypeIPCIDR,
				Elements: []firewallapi.SetElement{{Key: cidr}},
			}
			table := forgeTestTable([]firewallapi.Set{last}, "@last", firewallapi.MatchOperationEq)
			applyTable(t, conn, table)

			_, elems := getTestSet(t, conn, "last")
			wanted, err := genSetElements(&last)
			require.NoError(t, err)
			assert.False(t, isSetOutdated(elems, wanted), "cidr %s, existing elements: %+v", cidr, elems)
		}
	})

	t.Run("remove all sets", func(t *testing.T) {
		table := forgeTestTable(nil, "10.0.0.1", firewallapi.MatchOperationEq)
		applyTable(t, conn, table)
		assert.Empty(t, listTestSetNames(t, conn))
	})
}
