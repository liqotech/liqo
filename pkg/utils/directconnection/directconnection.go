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

// Package directconnection manages data related to endpoints that can leverage a direct connection.
package directconnection

import (
	"encoding/json"
)

// ClusterAddresses represents the addresses of pods deployed on remote clusters
// for which a direct connection may be available.
//
// Used to collect data and marshal to an annotation on the ShadowEndpointSlice.
//
// Key: clusterID. Value: list of IPs associated to that cluster.
type ClusterAddresses struct {
	Clusters map[string][]string `json:"clusterAddresses"`
}

// AddressIndex is an in-memory lookup table from IP to clusterID.
//
// It is used to store the data related to direct connections, for efficient retrieval of the clusterID.
//
// Key: IP address, value: ClusterID of the cluster where that endpoint is.
type AddressIndex struct {
	IPToCluster map[string]string
}

// Add inserts IPs associated with the given clusterID.
func (c *ClusterAddresses) Add(clusterID string, ips ...string) {
	if c.Clusters == nil {
		c.Clusters = make(map[string][]string)
	}
	c.Clusters[clusterID] = append(c.Clusters[clusterID], ips...)
}

// BuildIndex creates an IP-based lookup table from the marshaled data.
func (c *ClusterAddresses) BuildIndex() *AddressIndex {
	if c == nil || len(c.Clusters) == 0 {
		return nil
	}

	index := &AddressIndex{IPToCluster: make(map[string]string)}
	for clusterID, ips := range c.Clusters {
		for _, ip := range ips {
			index.IPToCluster[ip] = clusterID
		}
	}

	return index
}

// LookupClusterID retrieves the cluster ID associated with a given IP.
func (i *AddressIndex) LookupClusterID(ip string) (clusterID string, found bool) {
	if i == nil || i.IPToCluster == nil {
		return "", false
	}
	clusterID, found = i.IPToCluster[ip]
	return clusterID, found
}

// ToJSON serializes the ClusterAddresses to JSON.
func (c *ClusterAddresses) ToJSON() ([]byte, error) {
	return json.Marshal(c)
}

// FromJSON deserializes JSON data into ClusterAddresses.
func (c *ClusterAddresses) FromJSON(data []byte) error {
	return json.Unmarshal(data, c)
}
