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

package remapping

import (
	"reflect"
	"testing"

	"github.com/liqotech/liqo/pkg/firewall"
)

func TestForgeFirewallTargetLabels(t *testing.T) {
	got := ForgeFirewallTargetLabels("cluster-x")
	want := map[string]string{
		firewall.FirewallCategoryTargetKey: FirewallCategoryTargetValueGw,
		firewall.FirewallUniqueTargetKey:   "cluster-x",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ForgeFirewallTargetLabels(cluster-x) = %v, want %v", got, want)
	}
}

func TestForgeFirewallTargetLabelsIPMappingGw(t *testing.T) {
	got := ForgeFirewallTargetLabelsIPMappingGw()
	want := map[string]string{
		firewall.FirewallCategoryTargetKey:    FirewallCategoryTargetValueGw,
		firewall.FirewallSubCategoryTargetKey: FirewallSubCategoryTargetValueIPMapping,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ForgeFirewallTargetLabelsIPMappingGw() = %v, want %v", got, want)
	}
}

func TestForgeFirewallTargetLabelsIPMappingFabric(t *testing.T) {
	got := ForgeFirewallTargetLabelsIPMappingFabric()
	want := map[string]string{
		firewall.FirewallCategoryTargetKey:    FirewallCategoryTargetValueFabric,
		firewall.FirewallSubCategoryTargetKey: FirewallSubCategoryTargetValueIPMapping,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ForgeFirewallTargetLabelsIPMappingFabric() = %v, want %v", got, want)
	}
}

// ForgeFirewallBindingTargetLabels stores the remote cluster ID in the SubCategory key
// (alongside the gateway name in Unique). This is intentional and load-bearing for the
// BindingCreator's single-gateway path, so we explicitly assert it.
func TestForgeFirewallBindingTargetLabels_RemoteIDInSubCategory(t *testing.T) {
	got := ForgeFirewallBindingTargetLabels("remote-cluster-123", "gw-name")
	want := map[string]string{
		firewall.FirewallCategoryTargetKey:    FirewallCategoryTargetValueGw,
		firewall.FirewallSubCategoryTargetKey: "remote-cluster-123",
		firewall.FirewallUniqueTargetKey:      "gw-name",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ForgeFirewallBindingTargetLabels(remote-cluster-123, gw-name) = %v, want %v", got, want)
	}
}

func TestForgeFirewallBindingTargetLabelsIPMappingGw(t *testing.T) {
	got := ForgeFirewallBindingTargetLabelsIPMappingGw("gw-y")
	want := map[string]string{
		firewall.FirewallCategoryTargetKey:    FirewallCategoryTargetValueGw,
		firewall.FirewallSubCategoryTargetKey: FirewallSubCategoryTargetValueIPMapping,
		firewall.FirewallUniqueTargetKey:      "gw-y",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ForgeFirewallBindingTargetLabelsIPMappingGw(gw-y) = %v, want %v", got, want)
	}
}

func TestForgeFirewallBindingTargetLabelsIPMappingFabric(t *testing.T) {
	got := ForgeFirewallBindingTargetLabelsIPMappingFabric("node-z")
	want := map[string]string{
		firewall.FirewallCategoryTargetKey:    FirewallCategoryTargetValueFabric,
		firewall.FirewallSubCategoryTargetKey: FirewallSubCategoryTargetValueIPMapping,
		firewall.FirewallUniqueTargetKey:      "node-z",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ForgeFirewallBindingTargetLabelsIPMappingFabric(node-z) = %v, want %v", got, want)
	}
}
