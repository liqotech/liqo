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
//

package peer

import (
	"context"
	"fmt"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	"github.com/liqotech/liqo/pkg/liqoctl/info"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
)

// Info contains some info about the identity and the role of a peer cluster.
type Info struct {
	liqov1beta1.ClusterID `json:"clusterID"`
	Role                  liqov1beta1.RoleType `json:"role"`
}

// InfoChecker collects some info about the identity and the role of a peer cluster.
type InfoChecker struct {
	info.CheckerCommon

	// In this case data is a mapping between ClusterID and peer Info
	data map[liqov1beta1.ClusterID]Info
}

// Collect some data about the identity and the role of the peer cluster.
func (ic *InfoChecker) Collect(_ context.Context, options info.Options) {
	ic.data = map[liqov1beta1.ClusterID]Info{}
	for clusterID := range options.ClustersInfo {
		ic.data[clusterID] = Info{
			ClusterID: clusterID,
			Role:      options.ClustersInfo[clusterID].Status.Role,
		}
	}
}

// FormatForClusterID returns the collected data for the specified clusterID using a user friendly output.
func (ic *InfoChecker) FormatForClusterID(clusterID liqov1beta1.ClusterID, options info.Options) string {
	if data, ok := ic.data[clusterID]; ok {
		main := output.NewRootSection()
		main.AddEntry("Cluster ID", string(data.ClusterID))
		main.AddEntry("Role", string(data.Role))

		return main.SprintForBox(options.Printer)
	}
	return ""
}

// GetDataByClusterID returns the data collected by the checker for the cluster with the give ClusterID.
func (ic *InfoChecker) GetDataByClusterID(clusterID liqov1beta1.ClusterID) (interface{}, error) {
	if res, ok := ic.data[clusterID]; ok {
		return res, nil
	}
	return nil, fmt.Errorf("no data collected for cluster %q", clusterID)
}

// GetID returns the id of the section collected by the checker.
func (ic *InfoChecker) GetID() string {
	return "info"
}

// GetTitle returns the title of the section collected by the checker.
func (ic *InfoChecker) GetTitle() string {
	return "Peer cluster info"
}
