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

package forge

import (
	"slices"
	"strings"

	"k8s.io/apimachinery/pkg/api/meta"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
)

// networkConfigurationArgKeys contains the arg keys managed by SetNetworkConfigurationArgs.
var networkConfigurationArgKeys = []string{
	string(RemotePodCIDR),
	string(RemotePodCIDRRemap),
	string(RemoteExternalCIDR),
	string(RemoteExternalCIDRRemap),
}

// SetNetworkConfigurationArgs updates the VK container args with the CIDRs from the provided Configuration.
// If the Configuration is nil or not ready, all network configuration args are removed.
func SetNetworkConfigurationArgs(args []string, cfg *networkingv1beta1.Configuration) []string {
	args = removeNetworkConfigurationArgs(args)
	if !isNetworkConfigurationConfigured(cfg) {
		return args
	}

	args = appendCIDRArgs(args, RemotePodCIDR, cfg.Spec.Remote.CIDR.Pod)
	args = appendCIDRArgs(args, RemotePodCIDRRemap, cfg.Status.Remote.CIDR.Pod)
	args = appendCIDRArgs(args, RemoteExternalCIDR, cfg.Spec.Remote.CIDR.External)
	args = appendCIDRArgs(args, RemoteExternalCIDRRemap, cfg.Status.Remote.CIDR.External)
	return args
}

func removeNetworkConfigurationArgs(args []string) []string {
	return slices.DeleteFunc(args, func(arg string) bool {
		// Keep args that are not key=value pairs (e.g. boolean flags).
		if !strings.Contains(arg, "=") {
			return false
		}
		key, _ := DestringifyArgument(arg)
		// Remove if the key is in networkConfigurationArgKeys.
		return containsString(networkConfigurationArgKeys, key)
	})
}

func appendCIDRArgs(args []string, flag VirtualKubeletOptsFlag, cidrs []networkingv1beta1.CIDR) []string {
	for i := range cidrs {
		args = append(args, StringifyArgument(string(flag), string(cidrs[i])))
	}
	return args
}

func isNetworkConfigurationConfigured(cfg *networkingv1beta1.Configuration) bool {
	return cfg != nil && cfg.Status.Remote != nil &&
		meta.IsStatusConditionTrue(cfg.Status.Conditions, networkingv1beta1.ConfigurationConditionNetworkCIDRsConfigured)
}

func containsString(slice []string, value string) bool {
	for i := range slice {
		if slice[i] == value {
			return true
		}
	}
	return false
}
