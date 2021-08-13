package install

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	helm "github.com/mittwald/go-helm-client"
	"gopkg.in/yaml.v3"
	"helm.sh/helm/v3/pkg/action"
	"k8s.io/client-go/rest"

	"github.com/liqotech/liqo/pkg/liqoctl/install/aks"
	"github.com/liqotech/liqo/pkg/liqoctl/install/eks"
	"github.com/liqotech/liqo/pkg/liqoctl/install/gke"
	helmutils "github.com/liqotech/liqo/pkg/liqoctl/install/helm"
	"github.com/liqotech/liqo/pkg/liqoctl/install/kubeadm"
	"github.com/liqotech/liqo/pkg/liqoctl/install/provider"
	"github.com/liqotech/liqo/pkg/utils"
)

func getProviderInstance(providerType string) provider.InstallProviderInterface {
	switch providerType {
	case "kubeadm":
		return kubeadm.NewProvider()
	case "eks":
		return eks.NewProvider()
	case "gke":
		return gke.NewProvider()
	case "aks":
		return aks.NewProvider()
	default:
		return nil
	}
}

func initHelmClient(config *rest.Config, arguments *provider.CommonArguments) (*helm.HelmClient, error) {
	helmClient, err := helmutils.InitializeHelmClientWithRepo(config, arguments)
	if err != nil {
		fmt.Printf("Unable to create helmClient: %s", err)
		return nil, err
	}
	return helmClient, nil
}

func installOrUpdate(ctx context.Context, helmClient *helm.HelmClient, k provider.InstallProviderInterface, cArgs *provider.CommonArguments) error {
	output, _, err := helmutils.GetChart(helmutils.LiqoChartFullName, &action.ChartPathOptions{
		Version: cArgs.Version,
	}, helmClient.Settings)
	if err != nil {
		return err
	}

	k.UpdateChartValues(output.Values)

	raw, err := yaml.Marshal(&output.Values)
	if err != nil {
		return err
	}
	chartSpec := helm.ChartSpec{
		// todo (palexster): Check if it ReleaseName and LiqoNamespace are really configurable
		ReleaseName:      helmutils.LiqoReleaseName,
		ChartName:        helmutils.LiqoChartFullName,
		Namespace:        helmutils.LiqoNamespace,
		ValuesYaml:       string(raw),
		DependencyUpdate: true,
		Timeout:          cArgs.Timeout,
		GenerateName:     false,
	}

	_, err = helmClient.InstallOrUpgradeChart(ctx, &chartSpec)
	if err != nil {
		return err
	}

	return nil
}

func initClientConfig() (*rest.Config, error) {
	kubeconfigPath, ok := os.LookupEnv("KUBECONFIG")
	if !ok {
		kubeconfigPath = filepath.Join(os.Getenv("HOME"), ".kube", "config")
	}

	config, err := utils.UserConfig(kubeconfigPath)
	if err != nil {
		fmt.Printf("Unable to create client config: %s", err)
		return nil, err
	}

	return config, nil
}
