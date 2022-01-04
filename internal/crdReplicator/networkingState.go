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

package crdreplicator

import (
	"k8s.io/klog/v2"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	"github.com/liqotech/liqo/internal/crdReplicator/resources"
)

// getNetworkingState returns the state of the networking for a cluster given its clusterID.
func (c *Controller) getNetworkingState(clusterID string) discoveryv1alpha1.NetworkingEnabledType {
	c.networkingStateMutex.RLock()
	defer c.networkingStateMutex.RUnlock()
	if state, ok := c.networkingStates[clusterID]; ok {
		return state
	}
	return discoveryv1alpha1.NetworkingEnabledNone
}

// setNetworkingState sets the networking state for a given clusterID.
func (c *Controller) setNetworkingState(clusterID string, state discoveryv1alpha1.NetworkingEnabledType) {
	c.networkingStateMutex.RLock()
	defer c.networkingStateMutex.RUnlock()
	if c.networkingStates == nil {
		c.networkingStates = map[string]discoveryv1alpha1.NetworkingEnabledType{}
	}
	c.networkingStates[clusterID] = state
}

// isNetworkingEnabled indicates if the replication for the networkconfigs has to be enabled based on the state
// of the networking.
func isNetworkingEnabled(networkingState discoveryv1alpha1.NetworkingEnabledType, resource *resources.Resource) bool {
	// We are interested only for the networkconfigs resources.
	if resource.GroupVersionResource != netv1alpha1.NetworkConfigGroupVersionResource {
		return true
	}
	switch networkingState {
	case discoveryv1alpha1.NetworkingEnabledNone, discoveryv1alpha1.NetworkingEnabledNo:
		return false
	case discoveryv1alpha1.NetworkingEnabledYes:
		return true
	default:
		klog.Warning("Unknown networking state %v", resource.PeeringPhase)
		return false
	}
}
