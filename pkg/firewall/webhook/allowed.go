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

package webhook

import (
	"k8s.io/klog/v2"

	firewallapi "github.com/liqotech/liqo/apis/networking/v1alpha1/firewall"
)

// refer to https://wiki.nftables.org/wiki-nftables/index.php/Netfilter_hooks
func allowedTableFamilyChainTypeHook(familiy firewallapi.TableFamily, chainType firewallapi.ChainType, hook firewallapi.ChainHook) bool {
	switch familiy {
	case firewallapi.TableFamilyINet, firewallapi.TableFamilyIPv4, firewallapi.TableFamilyIPv6:
		switch chainType {
		case firewallapi.ChainTypeFilter:
			switch hook {
			case firewallapi.ChainHookIngress:
				if familiy == firewallapi.TableFamilyINet {
					return true
				}
				return false
			default:
				return true
			}
		case firewallapi.ChainTypeNAT:
			switch hook {
			case firewallapi.ChainHookPrerouting:
				return true
			case firewallapi.ChainHookInput:
				return true
			case firewallapi.ChainHookOutput:
				return true
			case firewallapi.ChainHookPostrouting:
				return true
			default:
				return false
			}
		case firewallapi.ChainTypeRoute:
			switch hook {
			case firewallapi.ChainHookOutput:
				return true
			default:
				return false
			}
		default:
			klog.Warningf("unknown chain type %v", chainType)
			return false
		}
	case firewallapi.TableFamilyARP:
		switch chainType {
		case firewallapi.ChainTypeFilter:
			switch hook {
			case firewallapi.ChainHookInput:
				return true
			case firewallapi.ChainHookOutput:
				return true
			default:
				return false
			}
		case firewallapi.ChainTypeNAT:
			return false
		case firewallapi.ChainTypeRoute:
			return false
		default:
			klog.Warningf("unknown chain type %v", chainType)
			return false
		}
	case firewallapi.TableFamilyBridge:
		switch chainType {
		case firewallapi.ChainTypeFilter:
			switch hook {
			case firewallapi.ChainHookIngress:
				return false
			default:
				return true
			}
		case firewallapi.ChainTypeNAT:
			return false
		case firewallapi.ChainTypeRoute:
			return false
		default:
			klog.Warningf("unknown chain type %v", chainType)
			return false
		}
	case firewallapi.TableFamilyNetdev:
		switch chainType {
		case firewallapi.ChainTypeFilter:
			switch hook {
			case firewallapi.ChainHookIngress:
				return true
			default:
				return false
			}
		case firewallapi.ChainTypeNAT:
			return false
		case firewallapi.ChainTypeRoute:
			return false
		default:
			klog.Warningf("unknown chain type %v", chainType)
			return false
		}
	default:
		klog.Warningf("unknown table family %v", familiy)
		return false
	}
}
