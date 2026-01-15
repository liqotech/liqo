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

package directconnection

import (
	"encoding/json"
)

// Info contains the IPs (both local and remapped) of the pods deployed on a remote cluster
// for which there may be a direct connection that can be used.
//
// ClusterID is the ID of the remote cluster where the pods are located.
// IPs are the Addresses in the Endpoint as seen by the consumer cluster BEFORE REMAPPING -> needed to extract the host part to remap correctly.
// RemappedIPs are the same IPs AFTER the remapping of the consumer -> needed to identify which IPs to replace.
type Info struct {
	ClusterID   string   `json:"ID"`
	IPs         []string `json:"IPs"`
	RemappedIPs []string `json:"rIPs"`
}

// InfoList is a collection if elements of type Info.
//
// There will be an item per ClusterID.
type InfoList struct {
	Items []Info `json:"items"`
}

// GetConnectionDataByIP retrieves the cluster ID and original IP address for a given remapped IP.
//
// NOTE: RemappedIP is used to identify the IP address to remap;
//
// IP is used to extract the host part;
//
// clusterID is used to retrieve the right podCIDR.
func (l *InfoList) GetConnectionDataByIP(ip string) (clusterID, originalIP string, found bool) {
	for _, entry := range l.Items {
		for i, remappedIP := range entry.RemappedIPs {
			if ip == remappedIP {
				return entry.ClusterID, entry.IPs[i], true
			}
		}
	}
	return "", "", false
}

// Add inserts new elements to the List.
// A new entry is created only if the clusterID is not already present.
func (l *InfoList) Add(clusterID string, ips, remappedIPs []string) {
	for i := range l.Items {
		if l.Items[i].ClusterID == clusterID {
			// ClusterID already exists, update the existing entry.
			l.Items[i].IPs = append(l.Items[i].IPs, ips...)
			l.Items[i].RemappedIPs = append(l.Items[i].RemappedIPs, remappedIPs...)
			return
		}
	}
	// ClusterID not found, create a new entry.
	l.Items = append(l.Items, Info{
		ClusterID:   clusterID,
		IPs:         ips,
		RemappedIPs: remappedIPs,
	})
}

// ToJSON serializes the Info to JSON.
func (d *Info) ToJSON() ([]byte, error) {
	return json.Marshal(d)
}

// FromJSON deserializes JSON data into the Info.
func (d *Info) FromJSON(data []byte) error {
	return json.Unmarshal(data, d)
}

// ToJSON serializes the InfoList to JSON.
func (l *InfoList) ToJSON() ([]byte, error) {
	return json.Marshal(l)
}

// FromJSON deserializes JSON data into the InfoList.
func (l *InfoList) FromJSON(data []byte) error {
	return json.Unmarshal(data, l)
}
