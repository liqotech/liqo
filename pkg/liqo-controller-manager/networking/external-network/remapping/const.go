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

package remapping

var (
	// TablePodCIDRName is the name of the table for the pod CIDR.
	TablePodCIDRName = "remap-podcidr"
	// TableExternalCIDRName is the name of the table for the external CIDR.
	TableExternalCIDRName = "remap-externalcidr"
	// TableIPMappingGwName is the name of the table for the IP mapping.
	TableIPMappingGwName = "remap-ipmapping-gw"
	// TableIPMappingFabricName is the name of the table for the IP mapping.
	TableIPMappingFabricName = "remap-ipmapping-fabric"

	// DNATChainName is the name of the chain for the output traffic.
	DNATChainName = "outgoing"
	// SNATChainName is the name of the chain for the input traffic.
	SNATChainName = "incoming"

	// PreroutingChainName is the name of the chain for the IP mapping.
	PreroutingChainName = "prerouting"
	// PostroutingChainName is the name of the chain for the IP mapping.
	PostroutingChainName = "postrouting"
)
