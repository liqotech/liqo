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

package flags

import (
	"github.com/spf13/pflag"
)

// FlagNames defines the names of the flags used by the network tests.
type FlagNames string

const (
	// FlagNamesProvidersKubeconfigs is the flag name for the kubeconfigs of the remote clusters.
	FlagNamesProvidersKubeconfigs FlagNames = "remote-kubeconfigs"
	// FlagNamesInfo is the flag name for the information output.
	FlagNamesInfo FlagNames = "info"
	// FlagNamesRemoveNamespace is the flag name for the namespace removal.
	FlagNamesRemoveNamespace FlagNames = "rm"
	// FlagNamesNodeportExternal is the flag that enables curl from external to nodeport service.
	FlagNamesNodeportExternal FlagNames = "np-ext"
	// FlagNamesNodeportNodes is the flag that selects nodes type for NodePort tests.
	FlagNamesNodeportNodes FlagNames = "np-nodes"
	// FlagNamesLoadbalancer is the flag that enables curl from external to loadbalancer service.
	FlagNamesLoadbalancer FlagNames = "lb"
	// FlagNamesBasic is the flag that runs only pod-to-pod checks.
	FlagNamesBasic FlagNames = "basic"
	// FlagNamesPodNodeport is the flag that enables curl from pod to nodeport service.
	FlagNamesPodNodeport FlagNames = "pod-np"
	// FlagNamesIP is the flag that enables IP remapping for the tests.
	FlagNamesIP FlagNames = "ip"
)

// AddFlags adds the flags used by the network tests to the given flag set.
func (o *Options) AddFlags(fs *pflag.FlagSet) {
	fs.StringSliceVarP(&o.RemoteKubeconfigs, string(FlagNamesProvidersKubeconfigs), "p", []string{},
		"A list of kubeconfigs for remote provider clusters")
	fs.BoolVar(&o.RemoveNamespace, string(FlagNamesRemoveNamespace), false, "Remove namespace after the test")
	fs.BoolVar(&o.Info, string(FlagNamesInfo), false, "Print information about the network configurations of the clusters")
	fs.BoolVar(&o.NodePortExt, string(FlagNamesNodeportExternal), false, "Enable curl from external to nodeport service")
	fs.Var(&o.NodePortNodes, string(FlagNamesNodeportNodes), "Select nodes type for NodePort tests. Possible values: all, workers, control-planes")
	fs.BoolVar(&o.LoadBalancer, string(FlagNamesLoadbalancer), false, "Enable curl from external to loadbalancer service")
	fs.BoolVar(&o.Basic, string(FlagNamesBasic), false, "Run only pod-to-pod checks")
	fs.BoolVar(&o.PodToNodePort, string(FlagNamesPodNodeport), false, "Enable curl from pod to nodeport service")
	fs.BoolVar(&o.IPRemapping, string(FlagNamesIP), false, "Enable IP remapping for the tests")
}
