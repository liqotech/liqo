// Copyright 2019-2022 The Liqo Authors
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

package routing

import netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"

const (
	routingTableID = 18952
)

// Routing interface used to configure the routing rules for peering clusters.
type Routing interface {
	EnsureRoutesPerCluster(tep *netv1alpha1.TunnelEndpoint) (bool, error)
	RemoveRoutesPerCluster(tep *netv1alpha1.TunnelEndpoint) (bool, error)
	CleanRoutingTable() error
	CleanPolicyRules() error
}
