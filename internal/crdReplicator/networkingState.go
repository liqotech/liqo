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

package crdreplicator

// getNetworkingState returns the state of the networking for a cluster given its clusterID.
func (c *Controller) getNetworkingEnabled(clusterID string) bool {
	c.networkingEnabledMutex.RLock()
	defer c.networkingEnabledMutex.RUnlock()
	return c.networkingEnabled[clusterID]
}

// setNetworkingState sets the networking state for a given clusterID.
func (c *Controller) setNetworkingEnabled(clusterID string, enabled bool) {
	c.networkingEnabledMutex.RLock()
	defer c.networkingEnabledMutex.RUnlock()
	c.networkingEnabled[clusterID] = enabled
}
