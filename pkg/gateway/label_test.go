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
