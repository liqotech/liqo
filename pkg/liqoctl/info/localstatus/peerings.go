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

package localstatus

import (
	"context"
	"fmt"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	"github.com/liqotech/liqo/pkg/liqoctl/info"
	"github.com/liqotech/liqo/pkg/liqoctl/info/common"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
)

// PeeringInfo represents the peering with another cluster.
type PeeringInfo struct {
	liqov1beta1.ClusterID `json:"clusterID"`
	Role                  liqov1beta1.RoleType `json:"role"`
	NetworkingStatus      common.ModuleStatus  `json:"networkingStatus"`
	AuthenticationStatus  common.ModuleStatus  `json:"authenticationStatus"`
	OffloadingStatus      common.ModuleStatus  `json:"offloadingStatus"`
}

// Peerings contains some brief data about the active peering of the local cluster.
type Peerings struct {
	Peers []PeeringInfo `json:"peers"`
}

// PeeringChecker collects the data about the active peering of the local cluster.
type PeeringChecker struct {
	info.CheckerCommon
	data Peerings
}

// Collect some brief data about the active peering of the local Liqo installation.
func (p *PeeringChecker) Collect(ctx context.Context, options info.Options) {
	var peeringsList liqov1beta1.ForeignClusterList
	if err := options.CRClient.List(ctx, &peeringsList); err != nil {
		p.AddCollectionError(fmt.Errorf("unable to retrieve peerings: %w", err))
		return
	}

	p.data.Peers = []PeeringInfo{}
	for i := range peeringsList.Items {
		peer := &peeringsList.Items[i]

		moduleStatus := map[string]common.ModuleStatus{}

		const NetworkingModuleName = "networking"
		const AuthenticationModuleName = "authentication"
		const OffloadingModuleName = "offloading"
		modules := map[string]liqov1beta1.Module{
			NetworkingModuleName:     peer.Status.Modules.Networking,
			AuthenticationModuleName: peer.Status.Modules.Authentication,
			OffloadingModuleName:     peer.Status.Modules.Offloading,
		}

		for moduleName, moduleInfo := range modules {
			moduleStatus[moduleName] = common.CheckModuleStatus(moduleInfo)
		}

		p.data.Peers = append(p.data.Peers, PeeringInfo{
			ClusterID:            peer.Spec.ClusterID,
			Role:                 peer.Status.Role,
			NetworkingStatus:     moduleStatus[NetworkingModuleName],
			AuthenticationStatus: moduleStatus[AuthenticationModuleName],
			OffloadingStatus:     moduleStatus[OffloadingModuleName],
		})
	}
}

// Format returns the collected data using a user friendly output.
func (p *PeeringChecker) Format(options info.Options) string {
	main := output.NewRootSection()
	for i := range p.data.Peers {
		peer := &p.data.Peers[i]
		peerSection := main.AddSectionInfo(string(peer.ClusterID))
		peerSection.AddEntry("Role", string(peer.Role))
		peerSection.AddEntry("Networking status", common.FormatStatus(peer.NetworkingStatus))
		peerSection.AddEntry("Authentication status", common.FormatStatus(peer.AuthenticationStatus))
		peerSection.AddEntry("Offloading status", common.FormatStatus(peer.OffloadingStatus))
	}

	return main.SprintForBox(options.Printer)
}

// GetData returns the data collected by the checker.
func (p *PeeringChecker) GetData() interface{} {
	return p.data
}

// GetID returns the id of the section collected by the checker.
func (p *PeeringChecker) GetID() string {
	return "peerings"
}

// GetTitle returns the title of the section collected by the checker.
func (p *PeeringChecker) GetTitle() string {
	return "Active peerings"
}
