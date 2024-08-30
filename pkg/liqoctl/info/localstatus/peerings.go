// Copyright 2019-2024 The Liqo Authors
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

	"github.com/pterm/pterm"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	"github.com/liqotech/liqo/pkg/liqoctl/info"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
)

// ModuleStatus represents the status of each of the modules.
type ModuleStatus string

const (
	// Healthy indicates a module that works as expected.
	Healthy ModuleStatus = "Healthy"
	// Unhealthy indicates that there are issues with the module.
	Unhealthy ModuleStatus = "Unhealthy"
	// Disabled indicates that the modules is not currently used.
	Disabled ModuleStatus = "Disabled"
)

// PeeringInfo represents the peering with another cluster.
type PeeringInfo struct {
	liqov1beta1.ClusterID `json:"clusterID"`
	Role                  liqov1beta1.RoleType `json:"role"`
	NetworkingStatus      ModuleStatus         `json:"networkingStatus"`
	AuthenticationStatus  ModuleStatus         `json:"authenticationStatus"`
	OffloadingStatus      ModuleStatus         `json:"offloadingStatus"`
}

// Peerings contains some brief data about the active peering of the local cluster.
type Peerings struct {
	Peers []PeeringInfo `json:"peers"`
}

// PeeringChecker collects the data about the active peering of the local cluster.
type PeeringChecker struct {
	info.CheckerBase
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

		moduleStatus := map[string]ModuleStatus{}

		modules := map[string]liqov1beta1.Module{
			"networking":     peer.Status.Modules.Networking,
			"authentication": peer.Status.Modules.Authentication,
			"offloading":     peer.Status.Modules.Offloading,
		}

		for moduleName, moduleInfo := range modules {
			if moduleInfo.Enabled {
				for i := range moduleInfo.Conditions {
					condition := &moduleInfo.Conditions[i]

					if condition.Status == liqov1beta1.ConditionStatusEstablished || condition.Status == liqov1beta1.ConditionStatusReady {
						moduleStatus[moduleName] = Healthy
					} else {
						moduleStatus[moduleName] = Unhealthy
					}
				}
			} else {
				moduleStatus[moduleName] = Disabled
			}
		}

		p.data.Peers = append(p.data.Peers, PeeringInfo{
			ClusterID:            peer.Spec.ClusterID,
			Role:                 peer.Status.Role,
			NetworkingStatus:     moduleStatus["networking"],
			AuthenticationStatus: moduleStatus["authentication"],
			OffloadingStatus:     moduleStatus["offloading"],
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
		peerSection.AddEntry("Networking status", p.formatStatus(peer.NetworkingStatus))
		peerSection.AddEntry("Authentication status", p.formatStatus(peer.AuthenticationStatus))
		peerSection.AddEntry("Offloading status", p.formatStatus(peer.OffloadingStatus))
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

func (p *PeeringChecker) formatStatus(moduleStatus ModuleStatus) string {
	var color pterm.Color
	switch moduleStatus {
	case Healthy:
		color = pterm.FgGreen
	case Disabled:
		color = pterm.FgLightCyan
	default:
		color = pterm.FgRed
	}
	return pterm.NewStyle(color, pterm.Bold).Sprint(moduleStatus)
}
