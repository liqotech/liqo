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

package gateway

import (
	"reflect"
	"testing"

	"github.com/liqotech/liqo/pkg/firewall"
)

func TestForgeFirewallInternalTargetLabels(t *testing.T) {
	got := ForgeFirewallInternalTargetLabels()
	want := map[string]string{
		firewall.FirewallCategoryTargetKey:    FirewallCategoryGwTargetValue,
		firewall.FirewallSubCategoryTargetKey: FirewallSubCategoryFabricTargetValue,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ForgeFirewallInternalTargetLabels() = %v, want %v", got, want)
	}
}

func TestForgeFirewallAllGatewaysTargetLabels(t *testing.T) {
	got := ForgeFirewallAllGatewaysTargetLabels()
	want := map[string]string{
		firewall.FirewallCategoryTargetKey:    FirewallCategoryGwTargetValue,
		firewall.FirewallSubCategoryTargetKey: FirewallSubCategoryAllGatewaysTargetValue,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ForgeFirewallAllGatewaysTargetLabels() = %v, want %v", got, want)
	}
}

func TestForgeFirewallAttachAllGatewaysTargetLabels(t *testing.T) {
	got := ForgeFirewallAttachAllGatewaysTargetLabels("gw-1")
	want := map[string]string{
		firewall.FirewallCategoryTargetKey:    FirewallCategoryGwTargetValue,
		firewall.FirewallSubCategoryTargetKey: FirewallSubCategoryAllGatewaysTargetValue,
		firewall.FirewallUniqueTargetKey:      "gw-1",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ForgeFirewallAttachAllGatewaysTargetLabels(gw-1) = %v, want %v", got, want)
	}
}

func TestForgeFirewallAttachInternalTargetLabels(t *testing.T) {
	got := ForgeFirewallAttachInternalTargetLabels("gw-2")
	want := map[string]string{
		firewall.FirewallCategoryTargetKey:    FirewallCategoryGwTargetValue,
		firewall.FirewallSubCategoryTargetKey: FirewallSubCategoryFabricTargetValue,
		firewall.FirewallUniqueTargetKey:      "gw-2",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ForgeFirewallAttachInternalTargetLabels(gw-2) = %v, want %v", got, want)
	}
}

func TestForgeFirewallAttachLabels_AttachVariantsAddUniqueKey(t *testing.T) {
	// Base (non-attach) variants do not carry the unique key; attach variants do.
	if _, ok := ForgeFirewallInternalTargetLabels()[firewall.FirewallUniqueTargetKey]; ok {
		t.Errorf("base ForgeFirewallInternalTargetLabels unexpectedly contains the unique-target key")
	}
	if _, ok := ForgeFirewallAttachInternalTargetLabels("gw-x")[firewall.FirewallUniqueTargetKey]; !ok {
		t.Errorf("attach variant ForgeFirewallAttachInternalTargetLabels missing the unique-target key")
	}
}
