// Copyright 2019-2023 The Liqo Authors
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

package aks

import (
	"context"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/profiles/2020-09-01/resources/mgmt/subscriptions"
	"github.com/Azure/azure-sdk-for-go/services/containerservice/mgmt/2021-07-01/containerservice"
	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2021-02-01/network"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/spf13/cobra"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/liqoctl/install"
)

var _ install.Provider = (*Options)(nil)

// Options encapsulates the arguments of the install command.
type Options struct {
	*install.Options

	subscriptionName  string
	subscriptionID    string
	resourceGroupName string
	resourceName      string

	authorizer *autorest.Authorizer
}

// New initializes a new Provider object.
func New(o *install.Options) install.Provider {
	return &Options{Options: o}
}

// Name returns the name of the provider.
func (o *Options) Name() string { return "aks" }

// Examples returns the examples string for the given provider.
func (o *Options) Examples() string {
	return `Examples:
  $ {{ .Executable }} install aks --resource-name foo --resource-group-name bar --subscription-id ***
or
  $ {{ .Executable }} install aks --resource-name foo --resource-group-name bar --subscription-name ***
`
}

// RegisterFlags registers the flags for the given provider.
func (o *Options) RegisterFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&o.subscriptionID, "subscription-id", "",
		"The ID of the Azure Subscription of the cluster (alternative to --subscription-name, takes precedence)")
	cmd.Flags().StringVar(&o.subscriptionName, "subscription-name", "",
		"The name of the Azure Subscription of the cluster (alternative to --subscription-id)")
	cmd.Flags().StringVar(&o.resourceGroupName, "resource-group-name", "",
		"The Azure ResourceGroup name of the cluster")
	cmd.Flags().StringVar(&o.resourceName, "resource-name", "", "The Azure Name of the cluster")

	utilruntime.Must(cmd.MarkFlagRequired("resource-group-name"))
	utilruntime.Must(cmd.MarkFlagRequired("resource-name"))
}

// Initialize performs the initialization tasks to retrieve the provider-specific parameters.
func (o *Options) Initialize(ctx context.Context) error {
	if o.subscriptionID == "" && o.subscriptionName == "" {
		return fmt.Errorf("neither --subscription-id nor --subscription-name specified")
	}

	o.Printer.Verbosef("AKS SubscriptionID: %q", o.subscriptionID)
	o.Printer.Verbosef("AKS SubscriptionName: %q", o.subscriptionName)
	o.Printer.Verbosef("AKS ResourceGroupName: %q", o.resourceGroupName)
	o.Printer.Verbosef("AKS ResourceName: %q", o.resourceName)

	// if the cluster name has not been provided, we default it to the cloud provider resource name.
	if o.ClusterName == "" {
		o.ClusterName = o.resourceName
	}

	authorizer, err := auth.NewAuthorizerFromCLI()
	if err != nil {
		return fmt.Errorf("failed connecting to the Azure API: %w", err)
	}
	o.authorizer = &authorizer

	if o.subscriptionID == "" {
		if err := o.retrieveSubscriptionID(ctx); err != nil {
			return fmt.Errorf("failed retrieving subscription ID for name %q: %w", o.subscriptionName, err)
		}
	}

	aksClient := containerservice.NewManagedClustersClient(o.subscriptionID)
	aksClient.Authorizer = *o.authorizer

	cluster, err := aksClient.Get(ctx, o.resourceGroupName, o.resourceName)
	if err != nil {
		return fmt.Errorf("failed retrieving cluster information: %w", err)
	}

	if err = o.parseClusterOutput(ctx, &cluster); err != nil {
		return fmt.Errorf("failed retrieving cluster information: %w", err)
	}

	return nil
}

// Values returns the customized provider-specifc values file parameters.
func (o *Options) Values() map[string]interface{} {
	return map[string]interface{}{
		"virtualKubelet": map[string]interface{}{
			"virtualNode": map[string]interface{}{
				"extra": map[string]interface{}{
					"labels": map[string]interface{}{
						"kubernetes.azure.com/managed": "false",
					},
				},
			},
		},
	}
}

func (o *Options) parseClusterOutput(ctx context.Context, cluster *containerservice.ManagedCluster) error {
	switch cluster.NetworkProfile.NetworkPlugin {
	case containerservice.NetworkPluginKubenet:
		if err := o.setupKubenet(ctx, cluster); err != nil {
			return err
		}
	case containerservice.NetworkPluginAzure:
		if err := o.setupAzureCNI(ctx, cluster); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown AKS network plugin %v", cluster.NetworkProfile.NetworkPlugin)
	}

	if cluster.Fqdn == nil {
		return fmt.Errorf("failed to retrieve cluster APIServer FQDN, is the cluster running?")
	}
	o.APIServer = *cluster.Fqdn

	if cluster.Location != nil {
		o.ClusterLabels[consts.TopologyRegionClusterLabel] = *cluster.Location
	}

	return nil
}

// setupKubenet setups the data for a Kubenet cluster.
func (o *Options) setupKubenet(ctx context.Context, cluster *containerservice.ManagedCluster) error {
	const defaultAksNodeCIDR = "10.240.0.0/16"

	o.PodCIDR = *cluster.ManagedClusterProperties.NetworkProfile.PodCidr
	o.ServiceCIDR = *cluster.ManagedClusterProperties.NetworkProfile.ServiceCidr

	// AKS Kubenet cluster does not have a subnet (and a subnetID) by default, in this case the node CIDR
	// is the default one.
	// But it is possible to specify an existent subnet during the cluster creation to be used as node CIDR,
	// in that case the vnet subnetID will be provided and we have to retrieve this network information.
	vnetSubjectID := (*cluster.AgentPoolProfiles)[0].VnetSubnetID
	if vnetSubjectID == nil {
		o.ReservedSubnets = append(o.ReservedSubnets, defaultAksNodeCIDR)
	} else {
		networkClient := network.NewSubnetsClient(o.subscriptionID)
		networkClient.Authorizer = *o.authorizer

		vnetName, subnetName, err := parseSubnetID(*vnetSubjectID)
		if err != nil {
			return err
		}

		vnet, err := networkClient.Get(ctx, o.resourceGroupName, vnetName, subnetName, "")
		if err != nil {
			return err
		}

		o.ReservedSubnets = append(o.ReservedSubnets, *vnet.SubnetPropertiesFormat.AddressPrefix)
	}

	return nil
}

// setupAzureCNI setups the data for an Azure CNI cluster.
func (o *Options) setupAzureCNI(ctx context.Context, cluster *containerservice.ManagedCluster) error {
	vnetSubjectID := (*cluster.AgentPoolProfiles)[0].VnetSubnetID

	networkClient := network.NewSubnetsClient(o.subscriptionID)
	networkClient.Authorizer = *o.authorizer

	vnetName, subnetName, err := parseSubnetID(*vnetSubjectID)
	if err != nil {
		return err
	}

	vnet, err := networkClient.Get(ctx, o.resourceGroupName, vnetName, subnetName, "")
	if err != nil {
		return err
	}

	o.PodCIDR = *vnet.AddressPrefix
	o.ServiceCIDR = *cluster.ManagedClusterProperties.NetworkProfile.ServiceCidr

	return nil
}

// parseSubnetID parses an Azure subnetID returning its vnet name and its subnet name.
// For example, if the subnetID is
// "/subscriptions/<YOUR_SUBSCRIPTION>/resourceGroups/test/providers/Microsoft.Network/virtualNetworks/testvnet663/subnets/default"
// it will return "testvnet663" as vnet name and "default" as subnet name.
func parseSubnetID(subnetID string) (vnetName, subnetName string, err error) {
	strs := strings.Split(subnetID, "/")
	l := len(strs)

	if l != 11 {
		err = fmt.Errorf("cannot parse SubnetID %v", subnetID)
		return "", "", err
	}

	return strs[l-3], strs[l-1], nil
}

func (o *Options) retrieveSubscriptionID(ctx context.Context) error {
	subClient := subscriptions.NewClient()
	subClient.Authorizer = *o.authorizer

	subList, err := subClient.List(ctx)
	if err != nil {
		return err
	}

	for subList.NotDone() {
		for _, v := range subList.Values() {
			if *v.DisplayName == o.subscriptionName {
				o.subscriptionID = *v.SubscriptionID
				return nil
			}
		}

		if err := subList.NextWithContext(ctx); err != nil {
			return err
		}
	}

	return fmt.Errorf("no subscription found matching the name")
}
