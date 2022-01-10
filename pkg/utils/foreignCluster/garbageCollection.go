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

package foreigncluster

import (
	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/pkg/discovery"
)

// HasToBeRemoved indicates if a ForeignCluster CR has to be removed.
func HasToBeRemoved(foreignCluster *discoveryv1alpha1.ForeignCluster) bool {
	isIncomingDiscovery := GetDiscoveryType(foreignCluster) == discovery.IncomingPeeringDiscovery
	hasPeering := IsIncomingEnabled(foreignCluster) || IsOutgoingEnabled(foreignCluster)
	return isIncomingDiscovery && !hasPeering
}
