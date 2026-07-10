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
		firewall.FirewallCategoryTargetKey: firewall.FirewallCategoryTargetValueGw,
		firewall.FirewallUniqueTargetKey:   "cluster-x",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ForgeFirewallTargetLabels(cluster-x) = %v, want %v", got, want)
	}
}

func TestForgeFirewallTargetLabelsIPMappingGw(t *testing.T) {
	got := ForgeFirewallTargetLabelsIPMappingGw()
	want := map[string]string{
		firewall.FirewallCategoryTargetKey:    firewall.FirewallCategoryTargetValueGw,
		firewall.FirewallSubCategoryTargetKey: firewall.FirewallSubCategoryTargetValueIPMapping,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ForgeFirewallTargetLabelsIPMappingGw() = %v, want %v", got, want)
	}
}

func TestForgeFirewallTargetLabelsIPMappingFabric(t *testing.T) {
	got := ForgeFirewallTargetLabelsIPMappingFabric()
	want := map[string]string{
		firewall.FirewallCategoryTargetKey:    firewall.FirewallCategoryTargetValueFabric,
		firewall.FirewallSubCategoryTargetKey: firewall.FirewallSubCategoryTargetValueIPMapping,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ForgeFirewallTargetLabelsIPMappingFabric() = %v, want %v", got, want)
	}
}
