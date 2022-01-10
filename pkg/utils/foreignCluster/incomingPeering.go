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
	"k8s.io/klog/v2"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
)

// AllowIncomingPeering returns the value set in the ForeignCluster spec if it has been set,
// it returns the value set through the command line flag if it is automatic.
func AllowIncomingPeering(foreignCluster *discoveryv1alpha1.ForeignCluster, defaultEnableIncomingPeering bool) bool {
	switch foreignCluster.Spec.IncomingPeeringEnabled {
	case discoveryv1alpha1.PeeringEnabledYes:
		return true
	case discoveryv1alpha1.PeeringEnabledNo:
		return false
	case discoveryv1alpha1.PeeringEnabledAuto:
		return defaultEnableIncomingPeering
	default:
		klog.Warningf("invalid value for incomingPeeringEnabled field: %v", foreignCluster.Spec.IncomingPeeringEnabled)
		return false
	}
}
