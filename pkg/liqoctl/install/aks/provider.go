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
	flag "github.com/spf13/pflag"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	"github.com/liqotech/liqo/pkg/liqoctl/install/provider"
	installutils "github.com/liqotech/liqo/pkg/liqoctl/install/utils"
)

const (
	providerPrefix = "aks"

	defaultAksNodeCIDR = "10.240.0.0/16"
)

type aksProvider struct {
	subscriptionName  string
	subscriptionID    string
	resourceGroupName string
	resourceName      string

	authorizer *autorest.Authorizer

	endpoint    string
	serviceCIDR string
	podCIDR     string

	reservedSubnets []string
}

// NewProvider initializes a new AKS provider struct.
func NewProvider() provider.InstallProviderInterface {
	return &aksProvider{}
}

// ValidateCommandArguments validates specific arguments passed to the install command.
func (k *aksProvider) ValidateCommandArguments(flags *flag.FlagSet) (err error) {
	k.subscriptionID, err = flags.GetString(installutils.PrefixedName(providerPrefix, "subscription-id"))
	if err != nil {
		return err
	}
	klog.V(3).Infof("AKS SubscriptionID: %v", k.subscriptionID)

	if k.subscriptionID == "" {
		k.subscriptionName, err = installutils.CheckStringFlagIsSet(flags, providerPrefix, "subscription-name")
		if err != nil {
			return err
		}
		klog.V(3).Infof("AKS SubscriptionName: %v", k.subscriptionName)
	}

	k.resourceGroupName, err = installutils.CheckStringFlagIsSet(flags, providerPrefix, "resource-group-name")
	if err != nil {
		return err
	}
	klog.V(3).Infof("AKS ResourceGroupName: %v", k.resourceGroupName)

	k.resourceName, err = installutils.CheckStringFlagIsSet(flags, providerPrefix, "resource-name")
	if err != nil {
		return err
	}
	klog.V(3).Infof("AKS ResourceName: %v", k.resourceName)

	return nil
}

// ExtractChartParameters fetches the parameters used to customize the Liqo installation on a specific cluster of a
// given provider.
func (k *aksProvider) ExtractChartParameters(ctx context.Context, _ *rest.Config) error {
	authorizer, err := auth.NewAuthorizerFromCLI()
	if err != nil {
		return err
	}

	k.authorizer = &authorizer

	if k.subscriptionID == "" {
		if err := k.retrieveSubscriptionID(ctx); err != nil {
			return err
		}
	}

	aksClient := containerservice.NewManagedClustersClient(k.subscriptionID)
	aksClient.Authorizer = *k.authorizer

	cluster, err := aksClient.Get(ctx, k.resourceGroupName, k.resourceName)
	if err != nil {
		return err
	}

	if err = k.parseClusterOutput(ctx, &cluster); err != nil {
		return err
	}

	return nil
}

// UpdateChartValues patches the values map with the values required for the selected cluster.
func (k *aksProvider) UpdateChartValues(values map[string]interface{}) {
	values["apiServer"] = map[string]interface{}{
		"address": k.endpoint,
	}
	values["networkManager"] = map[string]interface{}{
		"config": map[string]interface{}{
			"serviceCIDR":     k.serviceCIDR,
			"podCIDR":         k.podCIDR,
			"reservedSubnets": installutils.GetInterfaceSlice(k.reservedSubnets),
		},
	}
}

// GenerateFlags generates the set of specific subpath and flags are accepted for a specific provider.
func GenerateFlags(flags *flag.FlagSet) {
	subFlag := flag.NewFlagSet(providerPrefix, flag.ExitOnError)
	subFlag.SetNormalizeFunc(func(f *flag.FlagSet, name string) flag.NormalizedName {
		return flag.NormalizedName(installutils.PrefixedName(providerPrefix, name))
	})

	subFlag.String("subscription-id", "", "The ID of the Azure Subscription of your cluster,"+
		" if empty it will be retrieved using the value provided in --aks.subscription-name (optional)")
	subFlag.String("subscription-name", "", "The Name of the Azure Subscription of your cluster,"+
		" you have to provide it if you don't specify the --aks.subscription-id value (optional)")
	subFlag.String("resource-group-name", "", "The Azure ResourceGroup name of your cluster")
	subFlag.String("resource-name", "", "The Azure Name of your cluster")

	flags.AddFlagSet(subFlag)
}

func (k *aksProvider) parseClusterOutput(ctx context.Context, cluster *containerservice.ManagedCluster) error {
	switch cluster.NetworkProfile.NetworkPlugin {
	case containerservice.NetworkPluginKubenet:
		if err := k.setupKubenet(ctx, cluster); err != nil {
			return err
		}
	case containerservice.NetworkPluginAzure:
		if err := k.setupAzureCNI(ctx, cluster); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown AKS network plugin %v", cluster.NetworkProfile.NetworkPlugin)
	}

	k.endpoint = *cluster.Fqdn

	return nil
}

// setupKubenet setups the data for a Kubenet cluster.
func (k *aksProvider) setupKubenet(ctx context.Context, cluster *containerservice.ManagedCluster) error {
	k.podCIDR = *cluster.ManagedClusterProperties.NetworkProfile.PodCidr
	k.serviceCIDR = *cluster.ManagedClusterProperties.NetworkProfile.ServiceCidr

	// AKS Kubenet cluster does not have a subnet (and a subnetID) by default, in this case the node CIDR
	// is the default one.
	// But it's possible to specify an existent subnet during the cluster creation to be used as node CIDR,
	// in that case the vnet subnetID will be provided and we have to retrieve this network information.
	vnetSubjectID := (*cluster.AgentPoolProfiles)[0].VnetSubnetID
	if vnetSubjectID == nil {
		k.reservedSubnets = append(k.reservedSubnets, defaultAksNodeCIDR)
	} else {
		networkClient := network.NewSubnetsClient(k.subscriptionID)
		networkClient.Authorizer = *k.authorizer

		vnetName, subnetName, err := parseSubnetID(*vnetSubjectID)
		if err != nil {
			return err
		}

		vnet, err := networkClient.Get(ctx, k.resourceGroupName, vnetName, subnetName, "")
		if err != nil {
			return err
		}

		k.reservedSubnets = append(k.reservedSubnets, *vnet.SubnetPropertiesFormat.AddressPrefix)
	}

	return nil
}

// setupAzureCNI setups the data for an Azure CNI cluster.
func (k *aksProvider) setupAzureCNI(ctx context.Context, cluster *containerservice.ManagedCluster) error {
	vnetSubjectID := (*cluster.AgentPoolProfiles)[0].VnetSubnetID

	networkClient := network.NewSubnetsClient(k.subscriptionID)
	networkClient.Authorizer = *k.authorizer

	vnetName, subnetName, err := parseSubnetID(*vnetSubjectID)
	if err != nil {
		return err
	}

	vnet, err := networkClient.Get(ctx, k.resourceGroupName, vnetName, subnetName, "")
	if err != nil {
		return err
	}

	k.podCIDR = *vnet.AddressPrefix
	k.serviceCIDR = *cluster.ManagedClusterProperties.NetworkProfile.ServiceCidr

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

func (k *aksProvider) retrieveSubscriptionID(ctx context.Context) error {
	subClient := subscriptions.NewClient()
	subClient.Authorizer = *k.authorizer

	subList, err := subClient.List(ctx)
	if err != nil {
		return err
	}

	for subList.NotDone() {
		for _, v := range subList.Values() {
			klog.Infof("%v %v", *v.SubscriptionID, *v.DisplayName)
			if *v.DisplayName == k.subscriptionName {
				k.subscriptionID = *v.SubscriptionID
				return nil
			}
		}

		if err := subList.NextWithContext(ctx); err != nil {
			return err
		}
	}

	return fmt.Errorf("no subscription found with name: %v", k.subscriptionName)
}
