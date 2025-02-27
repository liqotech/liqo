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

package aks

import (
	"context"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v6"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v6"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/subscription/armsubscription"
	"github.com/spf13/cobra"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/ptr"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/liqoctl/install"
)

const (
	defaultAzureCNIPodCIDR = "10.224.0.0/16"
	defaultAksNodeCIDR     = "10.224.0.0/16"
)

var _ install.Provider = (*Options)(nil)

// Options encapsulates the arguments of the install aks command.
type Options struct {
	*install.Options

	subscriptionName      string
	subscriptionID        string
	resourceGroupName     string
	resourceName          string
	vnetResourceGroupName string
	privateLink           bool
	fqdn                  string

	azurecredential *azidentity.DefaultAzureCredential
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
	cmd.Flags().StringVar(&o.vnetResourceGroupName, "vnet-resource-group-name", "",
		"The Azure ResourceGroup name of the Virtual Network (defaults to --resource-group-name if not provided)")
	cmd.Flags().StringVar(&o.fqdn, "fqdn", "", "The private AKS cluster fqdn")
	cmd.Flags().BoolVar(&o.privateLink, "private-link", false, "Use the private FQDN for the API server")
	cmd.Flags().StringVar(&o.PodCIDR, "pod-cidr", "",
		"Pod CIDR of the cluster, only used for AzureCNI (legacy) clusters with no defined subnet")

	utilruntime.Must(cmd.MarkFlagRequired("resource-group-name"))
	utilruntime.Must(cmd.MarkFlagRequired("resource-name"))
}

// Initialize performs the initialization tasks to retrieve the provider-specific parameters.
func (o *Options) Initialize(ctx context.Context) error {
	if o.subscriptionID == "" && o.subscriptionName == "" {
		return fmt.Errorf("neither --subscription-id nor --subscription-name specified")
	}
	if o.vnetResourceGroupName == "" {
		// use AKS resource group if vnet resource group not provided
		o.vnetResourceGroupName = o.resourceGroupName
	}

	o.Printer.Verbosef("AKS SubscriptionID: %q", o.subscriptionID)
	o.Printer.Verbosef("AKS SubscriptionName: %q", o.subscriptionName)
	o.Printer.Verbosef("AKS ResourceGroupName: %q", o.resourceGroupName)
	o.Printer.Verbosef("AKS ResourceName: %q", o.resourceName)
	o.Printer.Verbosef("VNET ResourceGroupName: %q", o.vnetResourceGroupName)

	// if the cluster name has not been provided, we default it to the cloud provider resource name.
	if o.ClusterID == "" {
		o.ClusterID = liqov1beta1.ClusterID(o.resourceName)
	}

	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return fmt.Errorf("failed connecting to the Azure API: %w", err)
	}
	o.azurecredential = cred

	if o.subscriptionID == "" {
		if err := o.retrieveSubscriptionIDByName(ctx); err != nil {
			return fmt.Errorf("failed retrieving subscription ID for name %q: %w", o.subscriptionName, err)
		}
	}

	aksClient, err := armcontainerservice.NewManagedClustersClient(o.subscriptionID, o.azurecredential, nil)
	if err != nil {
		return fmt.Errorf("failed creating new managed clusters client: %w", err)
	}

	clusterResp, err := aksClient.Get(ctx, o.resourceGroupName, o.resourceName, nil)
	if err != nil {
		return fmt.Errorf("failed retrieving cluster %s: %w", o.resourceName, err)
	}

	if err = o.parseClusterOutput(ctx, &clusterResp.ManagedCluster); err != nil {
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
		"networking": map[string]interface{}{
			"fabric": map[string]interface{}{
				"config": map[string]interface{}{
					"gatewayMasqueradeBypass": true,
				},
			},
		},
	}
}

func (o *Options) parseClusterOutput(ctx context.Context, cluster *armcontainerservice.ManagedCluster) error {
	if cluster.Properties == nil || cluster.Properties.NetworkProfile == nil {
		return fmt.Errorf("failed to retrieve cluster network profile")
	}

	switch ptr.Deref(cluster.Properties.NetworkProfile.NetworkPlugin, "") {
	case armcontainerservice.NetworkPluginKubenet:
		if err := o.setupKubenetOrAzureCNIOverlay(ctx, cluster); err != nil {
			return err
		}
	case armcontainerservice.NetworkPluginAzure:
		switch ptr.Deref(cluster.Properties.NetworkProfile.NetworkPluginMode, "") {
		case armcontainerservice.NetworkPluginModeOverlay:
			if err := o.setupKubenetOrAzureCNIOverlay(ctx, cluster); err != nil {
				return err
			}
		default:
			if err := o.setupAzureCNI(ctx, cluster); err != nil {
				return err
			}
		}
	case armcontainerservice.NetworkPluginNone:
		if o.PodCIDR == "" {
			return fmt.Errorf("azure network plugin is set to `none`, please specify the PodCIDR with --pod-cidr")
		}
		o.ServiceCIDR = *cluster.Properties.NetworkProfile.ServiceCidr
	default:
		return fmt.Errorf("unknown AKS network plugin %v", cluster.Properties.NetworkProfile.NetworkPlugin)
	}

	switch {
	case o.privateLink:
		if cluster.Properties.PrivateFQDN == nil {
			return fmt.Errorf("private FQDN not found on cluster")
		}
		o.APIServer = *cluster.Properties.PrivateFQDN
	case cluster.Properties.Fqdn != nil:
		o.APIServer = *cluster.Properties.Fqdn
	case o.fqdn != "":
		o.APIServer = o.fqdn
	default:
		return fmt.Errorf("failed to retrieve cluster APIServer FQDN, is the cluster running?")
	}

	if cluster.Location != nil {
		o.ClusterLabels[consts.TopologyRegionClusterLabel] = *cluster.Location
	}

	return nil
}

// setupKubenet setups the data for a Kubenet cluster.
func (o *Options) setupKubenetOrAzureCNIOverlay(ctx context.Context, cluster *armcontainerservice.ManagedCluster) error {
	if ptr.Deref(cluster.Properties.NetworkProfile.PodCidr, "") == "" {
		return fmt.Errorf("no PodCIDR found on network profile")
	}
	o.PodCIDR = *cluster.Properties.NetworkProfile.PodCidr

	if ptr.Deref(cluster.Properties.NetworkProfile.ServiceCidr, "") == "" {
		return fmt.Errorf("no ServiceCIDR found on network profile")
	}
	o.ServiceCIDR = *cluster.Properties.NetworkProfile.ServiceCidr

	// AKS Kubenet cluster does not have a subnet (and a subnetID) by default, in this case the node CIDR
	// is the default one.
	// But it is possible to specify an existent subnet during the cluster creation to be used as node CIDR,
	// in that case the vnet subnetID will be provided and we have to retrieve this network information.
	if len(cluster.Properties.AgentPoolProfiles) == 0 ||
		cluster.Properties.AgentPoolProfiles[0] == nil ||
		ptr.Deref(cluster.Properties.AgentPoolProfiles[0].VnetSubnetID, "") == "" {
		o.ReservedSubnets = append(o.ReservedSubnets, defaultAksNodeCIDR)
		return nil
	}

	// VnetSubnet is specified, retrieve the subnet information.
	vnetSubnetID := *cluster.Properties.AgentPoolProfiles[0].VnetSubnetID

	subnetsClient, err := armnetwork.NewSubnetsClient(o.subscriptionID, o.azurecredential, nil)
	if err != nil {
		return err
	}

	vnetName, subnetName, err := parseSubnetID(vnetSubnetID)
	if err != nil {
		return err
	}

	vnet, err := subnetsClient.Get(ctx, o.vnetResourceGroupName, vnetName, subnetName, nil)
	if err != nil {
		return err
	}

	o.ReservedSubnets = append(o.ReservedSubnets, *vnet.Subnet.Properties.AddressPrefix)

	return nil
}

// setupAzureCNI setups the data for an Azure CNI cluster.
func (o *Options) setupAzureCNI(ctx context.Context, cluster *armcontainerservice.ManagedCluster) error {
	o.ServiceCIDR = *cluster.Properties.NetworkProfile.ServiceCidr

	if len(cluster.Properties.AgentPoolProfiles) == 0 || cluster.Properties.AgentPoolProfiles[0] == nil ||
		ptr.Deref(cluster.Properties.AgentPoolProfiles[0].VnetSubnetID, "") == "" {
		// VnetSubnet is not specified, use specified PodCIDR.
		if o.PodCIDR == "" {
			o.PodCIDR = defaultAzureCNIPodCIDR
		}
		// PodCIDR already set, nothing to do.
		return nil
	}

	// Else, retrieve the pod CIDR from the subnet, as for azure CNI the pod CIDR is the subnet CIDR.
	vnetSubnetID := *cluster.Properties.AgentPoolProfiles[0].VnetSubnetID

	subnetsClient, err := armnetwork.NewSubnetsClient(o.subscriptionID, o.azurecredential, nil)
	if err != nil {
		return err
	}

	vnetName, subnetName, err := parseSubnetID(vnetSubnetID)
	if err != nil {
		return err
	}

	vnet, err := subnetsClient.Get(ctx, o.vnetResourceGroupName, vnetName, subnetName, nil)
	if err != nil {
		return err
	}

	o.PodCIDR = *vnet.Subnet.Properties.AddressPrefix

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

func (o *Options) retrieveSubscriptionIDByName(ctx context.Context) error {
	subClient, err := armsubscription.NewSubscriptionsClient(o.azurecredential, nil)
	if err != nil {
		return err
	}

	pager := subClient.NewListPager(nil)

	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("failed to retrieve subscription page: %w", err)
		}

		for _, sub := range page.Value {
			if sub == nil {
				continue
			}
			if sub.DisplayName != nil && *sub.DisplayName == o.subscriptionName {
				if sub.SubscriptionID == nil || *sub.SubscriptionID == "" {
					return fmt.Errorf("subscription %q has an empty ID", o.subscriptionName)
				}
				o.subscriptionID = *sub.SubscriptionID
				o.Printer.Verbosef("Found AKS SubscriptionID: %q", o.subscriptionID)
				return nil
			}
		}
	}

	return fmt.Errorf("subscription %q not found", o.subscriptionName)
}
