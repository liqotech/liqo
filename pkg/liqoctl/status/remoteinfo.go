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

package status

import (
	"context"
	"fmt"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	controllerRuntimeClient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/liqotech/liqo/pkg/utils/getters"
)

// RemoteInfoChecker implements the Check interface.
// holds informations about remote clusters.
type RemoteInfoChecker struct {
	client             controllerRuntimeClient.Client
	namespace          string
	clusterNameFilter  *[]string
	clusterIDFilter    *[]string
	errors             bool
	rootRemoteInfoNode InfoNode
	collectionErrors   []collectionError
}

// newRemoteInfoChecker return a new remote info checker.
func newRemoteInfoChecker(namespace string, clusterNameFilter, clusterIDFilter *[]string, client controllerRuntimeClient.Client) *RemoteInfoChecker {
	return &RemoteInfoChecker{
		client:             client,
		namespace:          namespace,
		clusterNameFilter:  clusterNameFilter,
		clusterIDFilter:    clusterIDFilter,
		errors:             false,
		rootRemoteInfoNode: newRootInfoNode("Remote Clusters Information"),
	}
}

// argsFilterCheck returns true if the given foreignClusterID or foreignClusterName are contained
// in the respective filter list or if filter lists are void.
func (ric *RemoteInfoChecker) argsFilterCheck(foreignClusterID, foreignClusterName string) bool {
	return stringArrayContainsString(ric.clusterIDFilter, foreignClusterID) ||
		stringArrayContainsString(ric.clusterNameFilter, foreignClusterName) ||
		(len(*ric.clusterIDFilter) == 0 && len(*ric.clusterNameFilter) == 0)
}

// stringArrayContainsString returns true if the given string is contained in the given array.
func stringArrayContainsString(a *[]string, s string) bool {
	for _, v := range *a {
		if v == s {
			return true
		}
	}
	return false
}

// Collect implements the collect method of the Checker interface.
// it collects the infos of the Remote cluster.
func (ric *RemoteInfoChecker) Collect(ctx context.Context) error {
	var localNetworkConfigNode, remoteNetworkConfigNode, selectedNode *InfoNode
	var remoteNodeMsg string
	localClusterIdentity, err := getLocalClusterIdentity(ctx, ric.client, ric.namespace)
	localClusterName := localClusterIdentity.ClusterName
	if err != nil {
		ric.addCollectionError("LocalClusterName", "", err)
		ric.errors = true
	}

	networkConfigs, err := getters.GetNetworkConfigsByLabel(ctx, ric.client, "", labels.NewSelector())
	if err != nil {
		ric.addCollectionError("NetworkConfigs", "unable to collect NetworkConfigs", err)
		ric.errors = true
	}

	for i := range networkConfigs.Items {
		foreignClusterName := networkConfigs.Items[i].OwnerReferences[0].Name
		var foreignClusterID string
		if networkConfigs.Items[i].Labels["liqo.io/originID"] != "" {
			foreignClusterID = networkConfigs.Items[i].Labels["liqo.io/originID"]
		} else {
			foreignClusterID = networkConfigs.Items[i].Labels["liqo.io/remoteID"]
		}

		if ric.argsFilterCheck(foreignClusterID, foreignClusterName) {
			clusterNode := findNodeByTitle(ric.rootRemoteInfoNode.nextNodes, foreignClusterName)
			if clusterNode == nil {
				clusterNode = ric.rootRemoteInfoNode.addSectionToNode(foreignClusterName, "")
				localNetworkConfigNode = clusterNode.addSectionToNode("Local Network Configuration", "")
				remoteNetworkConfigNode = clusterNode.addSectionToNode("Remote Network Configuration", "")
			} else {
				localNetworkConfigNode = clusterNode.nextNodes[0]
				remoteNetworkConfigNode = clusterNode.nextNodes[1]
			}

			if networkConfigs.Items[i].Labels["liqo.io/replication"] == "true" {
				selectedNode = localNetworkConfigNode
				remoteNodeMsg = fmt.Sprintf("Status: how %s's CIDRs has been remapped by %s", localClusterName, foreignClusterName)
			} else {
				selectedNode = remoteNetworkConfigNode
				remoteNodeMsg = fmt.Sprintf("Status: how %s remapped %s's CIDRs", localClusterName, foreignClusterName)
			}

			originalNode := selectedNode.addSectionToNode("Original Network Configuration", "Spec")
			originalNode.addDataToNode("Pod CIDR", networkConfigs.Items[i].Spec.PodCIDR)
			originalNode.addDataToNode("External CIDR", networkConfigs.Items[i].Spec.ExternalCIDR)
			remoteNode := selectedNode.addSectionToNode("Remapped Network Configuration", remoteNodeMsg)
			remoteNode.addDataToNode("Pod CIDR", networkConfigs.Items[i].Status.PodCIDRNAT)
			remoteNode.addDataToNode("External CIDR", networkConfigs.Items[i].Status.ExternalCIDRNAT)

			tunnelEnpointSelector, err := v1.LabelSelectorAsSelector(&v1.LabelSelector{
				MatchExpressions: []v1.LabelSelectorRequirement{
					{
						Key:      "clusterID",
						Operator: v1.LabelSelectorOpIn,
						Values:   []string{networkConfigs.Items[i].Labels["liqo.io/remoteID"]},
					},
				},
			})
			if err != nil {
				ric.addCollectionError("TunnelEndpoint", fmt.Sprintf("unable to create TunnelEnpoint Selector for %s", foreignClusterName), err)
				ric.errors = true
			}
			tunnelEndpoint, err := getters.GetTunnelEndpointByLabel(ctx, ric.client, "", tunnelEnpointSelector)
			if err == nil {
				tunnelEndpointNode := clusterNode.addSectionToNode("Tunnel Endpoint", "")
				tunnelEndpointNode.addDataToNode("Gateway IP", tunnelEndpoint.Status.GatewayIP)
				tunnelEndpointNode.addDataToNode("Endpoint IP", tunnelEndpoint.Status.Connection.PeerConfiguration["endpointIP"])
			}
		}
	}

	return nil
}

// Format implements the format method of the Checker interface.
// it outputs the infos about the Remote cluster in a string ready to be
// printed out.
func (ric *RemoteInfoChecker) Format() (string, error) {
	w, buf := newTabWriter("")

	fmt.Fprintf(w, "%s", deepPrintInfo(&ric.rootRemoteInfoNode))

	for _, err := range ric.collectionErrors {
		fmt.Fprintf(w, "%s\t%s\t%s%s%s\n", err.appType, err.appName, red, err.err, reset)
	}

	if err := w.Flush(); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// HasSucceeded return true if no errors have been kept.
func (ric *RemoteInfoChecker) HasSucceeded() bool {
	return !ric.errors
}

// addCollectionError adds a collection error. A collection error is an error that happens while
// collecting the status of a Liqo component.
func (ric *RemoteInfoChecker) addCollectionError(remoteInfoType, remoteInfoName string, err error) {
	ric.collectionErrors = append(ric.collectionErrors, newCollectionError(remoteInfoType, remoteInfoName, err))
}
