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
	"strings"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	authv1beta1 "github.com/liqotech/liqo/apis/authentication/v1beta1"
	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/liqoctl/info"
	"github.com/liqotech/liqo/pkg/liqoctl/info/common"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
	"github.com/liqotech/liqo/pkg/utils/getters"
)

// ResourceSliceAction tells whether the cluster is consuming or providing the resources.
type ResourceSliceAction string

const (
	// ConsumingAction tells that the cluster is requesting and consuming the resources.
	ConsumingAction ResourceSliceAction = "Consuming"
	// ProvidingAction tells that the cluster is providing the resources.
	ProvidingAction ResourceSliceAction = "Providing"
)

// ResourceSliceStatus contains info about a ResourceSliceStatus CR.
type ResourceSliceStatus struct {
	Name      string              `json:"name"`
	Action    ResourceSliceAction `json:"action"`
	Accepted  bool                `json:"accepted"`
	Alerts    []string            `json:"alerts,omitempty"`
	Resources corev1.ResourceList `json:"resources"`
}

// Auth contains some info about the current status of the authentication module.
type Auth struct {
	Status         common.ModuleStatus `json:"status"`
	Alerts         []string            `json:"alerts,omitempty"`
	APIServerAddr  string
	ResourceSlices []ResourceSliceStatus `json:"resourceSlices"`
}

// AuthChecker collects some info about the current status of the authentication module.
type AuthChecker struct {
	info.CheckerCommon

	// In this case data is a mapping between ClusterID and peer Authentication info
	data map[liqov1beta1.ClusterID]Auth
}

// Collect some info about the current status of the authentication module.
func (ac *AuthChecker) Collect(ctx context.Context, options info.Options) {
	ac.data = map[liqov1beta1.ClusterID]Auth{}
	for clusterID := range options.ClustersInfo {
		authStatus := Auth{}
		ac.collectStatusInfo(clusterID, options.ClustersInfo, &authStatus)

		if authStatus.Status != common.ModuleDisabled {
			if options.ClustersInfo[clusterID].Status.Role == liqov1beta1.ProviderRole {
				if err := ac.collectAPIAddress(ctx, options.CRClient, clusterID, &authStatus); err != nil {
					ac.AddCollectionError(fmt.Errorf("unable to get API server address of cluster %q: %w", clusterID, err))
				}
			}

			// Get the ResourceSlices related to the given remote clusterID
			resSlices, err := getters.ListResourceSlicesByClusterID(ctx, options.CRClient, clusterID)
			if err != nil {
				ac.AddCollectionError(fmt.Errorf("unable to get ResourceSlices of cluster %q: %w", clusterID, err))
			} else {
				ac.collectResourceSlices(resSlices, &authStatus)
			}
		}

		ac.data[clusterID] = authStatus
	}
}

// FormatForClusterID returns the collected data for the specified clusterID using a user friendly output.
func (ac *AuthChecker) FormatForClusterID(clusterID liqov1beta1.ClusterID, options info.Options) string {
	if data, ok := ac.data[clusterID]; ok {
		main := output.NewRootSection()
		main.AddEntry("Status", common.FormatStatus(data.Status))

		if data.Status != common.ModuleDisabled {
			// Show alerts if any
			if len(data.Alerts) > 0 {
				main.AddEntryWarning("Alerts", data.Alerts...)
			}

			if data.APIServerAddr != "" {
				main.AddEntry("API server", data.APIServerAddr)
			}

			// Show resource slices
			slicesSection := main.AddSection("Resource slices")
			for i := range data.ResourceSlices {
				slice := &data.ResourceSlices[i]
				currSliceSection := slicesSection.AddSection(slice.Name)
				if slice.Accepted {
					currSliceSection.AddSectionSuccess("Resource slice accepted")
				} else {
					currSliceSection.AddSectionFailure("Resource slice not accepted")
				}
				if len(slice.Alerts) > 0 {
					main.AddEntryWarning("Alerts", slice.Alerts...)
				}
				currSliceSection.AddEntry("Action", string(slice.Action))

				resourcesSection := currSliceSection.AddSection("Resources")
				for resource, quantity := range slice.Resources {
					resourcesSection.AddEntry(string(resource), quantity.String())
				}
			}
		}

		return main.SprintForBox(options.Printer)
	}
	return ""
}

// GetData returns the data collected by the checker.
func (ac *AuthChecker) GetData() interface{} {
	return ac.data
}

// GetDataByClusterID returns the data collected by the checker for the cluster with the give ClusterID.
func (ac *AuthChecker) GetDataByClusterID(clusterID liqov1beta1.ClusterID) (interface{}, error) {
	if res, ok := ac.data[clusterID]; ok {
		return res, nil
	}
	return nil, fmt.Errorf("no data collected for cluster %q", clusterID)
}

// GetID returns the id of the section collected by the checker.
func (ac *AuthChecker) GetID() string {
	return "authentication"
}

// GetTitle returns the title of the section collected by the checker.
func (ac *AuthChecker) GetTitle() string {
	return "Authentication"
}

// collectStatusInfo collects the info about the status of the network between the local cluster and the one with the given ClusterID.
func (ac *AuthChecker) collectStatusInfo(clusterID liqov1beta1.ClusterID,
	clusterInfo map[liqov1beta1.ClusterID]*liqov1beta1.ForeignCluster, authStatus *Auth) {
	cluster := clusterInfo[clusterID]
	authStatus.Status, authStatus.Alerts = common.CheckModuleStatusAndAlerts(cluster.Status.Modules.Authentication)
}

// collectResourceSlices collect the data from the list of ResourceSlice CRs.
func (ac *AuthChecker) collectResourceSlices(resourceSlices []authv1beta1.ResourceSlice, authStatus *Auth) {
	authStatus.ResourceSlices = []ResourceSliceStatus{}
	for i := range resourceSlices {
		resSlice := &resourceSlices[i]

		// Check the status of the ResourceSlice
		// If no condition is present in the status, it means that the operator has not already reconciled the
		// resource, so the resource is not accepted. Otherwise, we initialize the variable to `true` and we set
		// it to false if we find a condition not accepted.
		accepted := len(resSlice.Status.Conditions) > 0
		alerts := []string{}
		for _, condition := range resSlice.Status.Conditions {
			isConditionAccepted := condition.Status == authv1beta1.ResourceSliceConditionAccepted
			if !isConditionAccepted {
				accepted = false
				alerts = append(alerts, condition.Message)
			}
		}

		// To check who is the provider of the ResourceSlice we can verify whether the CR has been replicated from the cluster consumer
		action := ConsumingAction
		if value, isProvider := resSlice.ObjectMeta.Labels[consts.ReplicationStatusLabel]; isProvider && strings.EqualFold(value, "true") {
			action = ProvidingAction
		}

		authStatus.ResourceSlices = append(authStatus.ResourceSlices, ResourceSliceStatus{
			Name:      resSlice.Name,
			Action:    action,
			Accepted:  accepted,
			Alerts:    alerts,
			Resources: resSlice.Status.Resources,
		})
	}
}

func (ac *AuthChecker) collectAPIAddress(ctx context.Context, cl client.Client, clusterID liqov1beta1.ClusterID, authStatus *Auth) error {
	identity, err := getters.GetControlPlaneIdentityByClusterID(ctx, cl, clusterID)
	if err != nil {
		return err
	}

	if identity.Spec.AuthParams.ProxyURL != nil {
		authStatus.APIServerAddr = *identity.Spec.AuthParams.ProxyURL
	} else {
		authStatus.APIServerAddr = identity.Spec.AuthParams.APIServer
	}
	return nil
}
