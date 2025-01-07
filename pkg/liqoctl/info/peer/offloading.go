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

	corev1 "k8s.io/api/core/v1"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/liqoctl/info"
	"github.com/liqotech/liqo/pkg/liqoctl/info/common"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
	"github.com/liqotech/liqo/pkg/utils/getters"
)

// VirtualNodeStatus contains info about a VirtualNodeStatus CR.
type VirtualNodeStatus struct {
	Name          string              `json:"name"`
	Status        common.ModuleStatus `json:"status"`
	Secret        string              `json:"secret"`
	ResourceSlice string              `json:"resourceSlice,omitempty"`
	Resources     corev1.ResourceList `json:"resources"`
}

// Offloading contains info about offloaded resources and virtual nodes.
type Offloading struct {
	Status       common.ModuleStatus `json:"status"`
	Alerts       []string            `json:"alerts,omitempty"`
	VirtualNodes []VirtualNodeStatus `json:"virtualNodes"`
}

// OffloadingChecker collects info about offloaded resources and virtual nodes.
type OffloadingChecker struct {
	info.CheckerCommon

	// In this case data is a mapping between ClusterID and Offloading info
	data map[liqov1beta1.ClusterID]Offloading
}

// Collect some info about the status of the network between the local cluster and the active peers.
func (oc *OffloadingChecker) Collect(ctx context.Context, options info.Options) {
	oc.data = map[liqov1beta1.ClusterID]Offloading{}
	for clusterID := range options.ClustersInfo {
		offloadingState := Offloading{}

		// Collect the status of the offloading module
		oc.collectStatusInfo(clusterID, options.ClustersInfo, &offloadingState)

		// Get the VirtualNode resources pointing to the given remote clusterID
		virtualNodes, err := getters.ListVirtualNodesByClusterID(ctx, options.CRClient, clusterID)
		if err != nil {
			oc.AddCollectionError(fmt.Errorf("unable to get VirtualNodes pointing to cluster %q: %w", clusterID, err))
		} else {
			oc.collectVirtualNodes(virtualNodes, &offloadingState)
		}
		oc.data[clusterID] = offloadingState
	}
}

// FormatForClusterID returns the collected data for the specified clusterID using a user friendly output.
func (oc *OffloadingChecker) FormatForClusterID(clusterID liqov1beta1.ClusterID, options info.Options) string {
	if data, ok := oc.data[clusterID]; ok {
		main := output.NewRootSection()
		main.AddEntry("Status", common.FormatStatus(data.Status))
		if data.Status != common.ModuleDisabled {
			// Show alerts if any
			if len(data.Alerts) > 0 {
				main.AddEntryWarning("Alerts", data.Alerts...)
			}

			// Show virtual nodes
			nodesSection := main.AddSection("Virtual nodes")
			for i := range data.VirtualNodes {
				vNode := &data.VirtualNodes[i]
				currNodeSection := nodesSection.AddSection(vNode.Name)
				currNodeSection.AddEntry("Status", common.FormatStatus(vNode.Status))
				currNodeSection.AddEntry("Secret", vNode.Secret)
				currNodeSection.AddEntry("Resource slice", vNode.ResourceSlice)
				resourcesSection := currNodeSection.AddSection("Resources")
				for resource, quantity := range vNode.Resources {
					resourcesSection.AddEntry(string(resource), quantity.String())
				}
			}
		}
		return main.SprintForBox(options.Printer)
	}
	return ""
}

// GetDataByClusterID returns the data collected by the checker for the cluster with the give ClusterID.
func (oc *OffloadingChecker) GetDataByClusterID(clusterID liqov1beta1.ClusterID) (interface{}, error) {
	if res, ok := oc.data[clusterID]; ok {
		return res, nil
	}
	return nil, fmt.Errorf("no data collected for cluster %q", clusterID)
}

// GetID returns the id of the section collected by the checker.
func (oc *OffloadingChecker) GetID() string {
	return "offloading"
}

// GetTitle returns the title of the section collected by the checker.
func (oc *OffloadingChecker) GetTitle() string {
	return "Offloading"
}

// collectStatusInfo collects the info about the status of the offloading module.
func (oc *OffloadingChecker) collectStatusInfo(clusterID liqov1beta1.ClusterID,
	clusterInfo map[liqov1beta1.ClusterID]*liqov1beta1.ForeignCluster, offloadingState *Offloading) {
	cluster := clusterInfo[clusterID]
	offloadingState.Status, offloadingState.Alerts = common.CheckModuleStatusAndAlerts(cluster.Status.Modules.Offloading)
}

// collectVirtualNodes collects the data from the list of VirtualNode CRs.
func (oc *OffloadingChecker) collectVirtualNodes(virtualNodes []offloadingv1beta1.VirtualNode, offloadingState *Offloading) {
	offloadingState.VirtualNodes = []VirtualNodeStatus{}
	for i := range virtualNodes {
		vNode := &virtualNodes[i]

		// Check the status of the virtual node
		status := common.ModuleUnhealthy
		for _, condition := range vNode.Status.Conditions {
			if condition.Status == offloadingv1beta1.RunningConditionStatusType || condition.Status == offloadingv1beta1.NoneConditionStatusType {
				status = common.ModuleHealthy
			} else {
				status = common.ModuleUnhealthy
				break
			}
		}

		// Get the ResourceSlice assigned to VirtualNode secret
		resourceSlice := vNode.ObjectMeta.Labels[consts.ResourceSliceNameLabelKey]

		// Get the secret configured in the virtual node
		secretName := ""
		if vNode.Spec.KubeconfigSecretRef != nil {
			secretName = vNode.Spec.KubeconfigSecretRef.Name
		}

		offloadingState.VirtualNodes = append(offloadingState.VirtualNodes, VirtualNodeStatus{
			Name:          vNode.Name,
			Status:        status,
			ResourceSlice: resourceSlice,
			Secret:        secretName,
			Resources:     vNode.Spec.ResourceQuota.Hard,
		})
	}
}
