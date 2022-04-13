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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	controllerRuntimeClient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/pkg/liqoctl/common"
	"github.com/liqotech/liqo/pkg/utils/getters"
	liqolabels "github.com/liqotech/liqo/pkg/utils/labels"
)

// LocalInfoChecker implements the Check interface.
// holds the Localinformation about the cluster.
type LocalInfoChecker struct {
	client            controllerRuntimeClient.Client
	namespace         string
	errors            bool
	rootLocalInfoNode InfoNode
	collectionErrors  []collectionError
}

// newPodChecker return a new pod checker.
func newLocalInfoChecker(namespace string, client controllerRuntimeClient.Client) *LocalInfoChecker {
	return &LocalInfoChecker{
		client:            client,
		namespace:         namespace,
		errors:            false,
		rootLocalInfoNode: newRootInfoNode("Local Cluster Informations"),
	}
}

func getLocalClusterIdentity(ctx context.Context, client controllerRuntimeClient.Client, namespace string) (*v1alpha1.ClusterIdentity, error) {
	selector, err := metav1.LabelSelectorAsSelector(&liqolabels.ClusterIDConfigMapLabelSelector)
	if err != nil {
		return nil, fmt.Errorf("unable to collect Selector from LabelSelector: %w", err)
	}
	configMap, err := getters.GetConfigMapByLabel(ctx, client, namespace, selector)
	if err != nil {
		return nil, fmt.Errorf("unable to collect ConfigMap using Selector: %w", err)
	}
	clusterIdentity, err := getters.RetrieveClusterIDFromConfigMap(configMap)
	if err != nil {
		return nil, fmt.Errorf("unable to collect ClusterIdentity using ConfigMap: %w", err)
	}
	return clusterIdentity, nil
}

func getLocalNetworkConfig(ctx context.Context, client controllerRuntimeClient.Client) (*getters.NetworkConfig, error) {
	selector, err := metav1.LabelSelectorAsSelector(&liqolabels.IPAMStorageLabelSelector)
	if err != nil {
		return nil, fmt.Errorf("unable to collect Selector using LabelSelector: %w", err)
	}
	ipamStorage, err := getters.GetIPAMStorageByLabel(ctx, client, "default", selector)
	if err != nil {
		return nil, fmt.Errorf("unable to collect IPAMStorage map using Selector: %w", err)
	}
	networkConfig, err := getters.RetrieveNetworkConfiguration(ipamStorage)
	if err != nil {
		return nil, fmt.Errorf("unable to collect NetworkConfig map using IPAMStorage: %w", err)
	}
	return networkConfig, nil
}

// Collect implements the collect method of the Checker interface.
// it collects the infos of the local cluster.
func (lic *LocalInfoChecker) Collect(ctx context.Context) error {
	clusterIdentity, err := getLocalClusterIdentity(ctx, lic.client, lic.namespace)
	if err != nil {
		lic.addCollectionError("ClusterIdentity", "", err)
		lic.errors = true
	}
	clusterIdentityNode := lic.rootLocalInfoNode.addSectionToNode("Cluster Identity", "")
	clusterIdentityNode.addDataToNode("Cluster ID", clusterIdentity.ClusterID)
	clusterIdentityNode.addDataToNode("Cluster Name", clusterIdentity.ClusterName)

	clusterLabelsNode := lic.rootLocalInfoNode.addSectionToNode("Cluster Labels", "")
	hc, err := common.NewLiqoHelmClient()
	if err != nil {
		lic.addCollectionError("ClusterIdentity", "Creating new LiqoHelmClient", err)
		lic.errors = true
	}
	clusterLabels, err := hc.GetClusterLabels()
	if err != nil {
		lic.addCollectionError("Cluster Labels", "Getting cluster labels", err)
		lic.errors = true
	}
	for k, v := range clusterLabels {
		clusterLabelsNode.addDataToNode(k, v)
	}

	networkConfigNode := lic.rootLocalInfoNode.addSectionToNode("Network Configuration", "")
	networkConfig, err := getLocalNetworkConfig(ctx, lic.client)
	if err != nil {
		lic.addCollectionError("Network Configuration", "", err)
		lic.errors = true
	}
	networkConfigNode.addDataToNode("Pod CIDR", networkConfig.PodCIDR)
	networkConfigNode.addDataToNode("External CIDR", networkConfig.ExternalCIDR)
	networkConfigNode.addDataToNode("Service CIDR", networkConfig.ServiceCIDR)
	if len(networkConfig.ReservedSubnets) != 0 {
		networkConfigNode.addDataListToNode("Reserved Subnets", networkConfig.ReservedSubnets)
	}

	return nil
}

// Format implements the format method of the Checker interface.
// it outputs the infos about the local cluster in a string ready to be
// printed out.
func (lic *LocalInfoChecker) Format() (string, error) {
	w, buf := newTabWriter("")

	fmt.Fprintf(w, "%s", deepPrintInfo(&lic.rootLocalInfoNode))

	for _, err := range lic.collectionErrors {
		fmt.Fprintf(w, "%s\t%s\t%s%s%s\n", err.appType, err.appName, red, err.err, reset)
	}

	if err := w.Flush(); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// HasSucceeded return true if no errors have been kept.
func (lic *LocalInfoChecker) HasSucceeded() bool {
	return !lic.errors
}

// addCollectionError adds a collection error. A collection error is an error that happens while
// collecting the status of a Liqo component.
func (lic *LocalInfoChecker) addCollectionError(localInfoType, localInfoName string, err error) {
	lic.collectionErrors = append(lic.collectionErrors, newCollectionError(localInfoType, localInfoName, err))
}
