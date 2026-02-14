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

// package directconnection manages data related to endpoints that can leverage a direct connection.
package directconnection

import (
	"encoding/json"
)

// DirectConnectionData is a struct representing the addresses of the pods deployed on a remote cluster
// for which there may be a direct connection that can be used.
//
// Used to collect data and marshal to an annotation on the ShadowEndpointSlice.
//
// Key: clusterID. Value: list of IPs associated to that cluster.
type DirectConnectionData struct {
	ByCluster map[string][]string `json:"clstrIDtoAddrs"`
}

// DirectConnectionIndex is an in-memory lookup table from IP to clusterID. 
//
// It is used to store the data relative to direct connections, for efficient retrieval of the clusterID.
//
// Key: IP address, value: ClusterID of the cluster where that address is.
type DirectConnectionIndex struct {
	IPToCluster map[string]string
}

// Add inserts IPs associated with the given clusterID.
func (c *DirectConnectionData) Add(clusterID string, ips ...string) {
	if c.ByCluster == nil {
		c.ByCluster = make(map[string][]string)
	}
	c.ByCluster[clusterID] = append(c.ByCluster[clusterID], ips...)
}

// BuildIndex creates an IP-based lookup table from the marshaled data.
func (c *DirectConnectionData) BuildIndex() *DirectConnectionIndex {
	if c == nil || len(c.ByCluster) == 0 {
		return nil
	}

	index := &DirectConnectionIndex{IPToCluster: make(map[string]string)}
	for clusterID, ips := range c.ByCluster {
		for _, ip := range ips {
			index.IPToCluster[ip] = clusterID
		}
	}

	return index
}

// LookupClusterID retrieves the cluster ID associated with a given IP.
func (i *DirectConnectionIndex) LookupClusterID(ip string) (clusterID string, found bool) {
	if i == nil || i.IPToCluster == nil {
		return "", false
	}
	clusterID, found = i.IPToCluster[ip]
	return clusterID, found
}

// ToJSON serializes the DirectConnectionData to JSON.
func (c *DirectConnectionData) ToJSON() ([]byte, error) {
	return json.Marshal(c)
}

// FromJSON deserializes JSON data into the DirectConnectionData.
func (c *DirectConnectionData) FromJSON(data []byte) error {
	return json.Unmarshal(data, c)
}
