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

package testutil

const (
	// APIAddress is the address of the API server used for testing.
	APIAddress = "1.0.0.1"
	// PodCIDR is the CIDR of the pod network used for testing.
	PodCIDR = "fake pod CIDR"
	// ServiceCIDR is the CIDR of the service network used for testing.
	ServiceCIDR = "fake service CIDR"
	// ExternalCIDR is the name of the cluster used for testing.
	ExternalCIDR = "fake external CIDR"
)

var (
	// ReservedSubnets is the list of reserved subnets used for testing.
	ReservedSubnets = []string{
		"reserved subnet 1",
		"reserved subnet 2",
		"reserved subnet 3",
		"reserved subnet 4",
	}
	// ClusterLabels is the map of labels used for testing.
	ClusterLabels = map[string]string{
		"liqo.io/testLabel": "fake label",
	}

	// OverrideAPIAddress is the overrided address of the API server used for testing.
	OverrideAPIAddress = "1.0.0.2:6443"
)
