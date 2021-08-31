package kubeadm

import (
	"context"
	"fmt"

	flag "github.com/spf13/pflag"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/liqoctl/install/provider"
	installutils "github.com/liqotech/liqo/pkg/liqoctl/install/utils"
)

// NewProvider initializes a new Kubeadm struct.
func NewProvider() provider.InstallProviderInterface {
	return &Kubeadm{
		ClusterLabels: map[string]string{
			consts.ProviderClusterLabel: providerPrefix,
		},
	}
}

// ValidateCommandArguments validates specific arguments passed to the install command.
func (k *Kubeadm) ValidateCommandArguments(flags *flag.FlagSet) error {
	return nil
}

// ExtractChartParameters fetches the parameters used to customize the Liqo installation on a specific cluster of a
// given provider.
func (k *Kubeadm) ExtractChartParameters(ctx context.Context, config *rest.Config, _ *provider.CommonArguments) error {
	k8sClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		fmt.Printf("Unable to create client: %s", err)
		return err
	}

	k.K8sClient = k8sClient
	k.Config = config
	k.APIServer = config.Host

	k.PodCIDR, k.ServiceCIDR, err = retrieveClusterParameters(ctx, k.K8sClient)
	if err != nil {
		return err
	}
	return nil
}

// UpdateChartValues patches the values map with the values required for the selected cluster.
func (k *Kubeadm) UpdateChartValues(values map[string]interface{}) {
	values["apiServer"] = map[string]interface{}{
		"address": k.APIServer,
	}
	values["networkManager"] = map[string]interface{}{
		"config": map[string]interface{}{
			"serviceCIDR": k.ServiceCIDR,
			"podCIDR":     k.PodCIDR,
		},
	}
	values["discovery"] = map[string]interface{}{
		"config": map[string]interface{}{
			"clusterLabels": installutils.GetInterfaceMap(k.ClusterLabels),
		},
	}
}

// GenerateFlags generates the set of specific subpath and flags are accepted for a specific provider.
func GenerateFlags(flags *flag.FlagSet) {
	subFlag := flag.NewFlagSet(providerPrefix, flag.ExitOnError)
	subFlag.SetNormalizeFunc(func(f *flag.FlagSet, name string) flag.NormalizedName {
		return flag.NormalizedName(providerPrefix + "." + name)
	})

	flags.AddFlagSet(subFlag)
}
