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
	"k8s.io/klog/v2"

	"github.com/liqotech/liqo/pkg/liqoctl/install/aks"
	"github.com/liqotech/liqo/pkg/liqoctl/install/eks"
	"github.com/liqotech/liqo/pkg/liqoctl/install/gke"
	helmutils "github.com/liqotech/liqo/pkg/liqoctl/install/helm"
	"github.com/liqotech/liqo/pkg/liqoctl/install/k3s"
	"github.com/liqotech/liqo/pkg/liqoctl/install/kind"
	"github.com/liqotech/liqo/pkg/liqoctl/install/kubeadm"
	"github.com/liqotech/liqo/pkg/liqoctl/install/provider"
	installutils "github.com/liqotech/liqo/pkg/liqoctl/install/utils"
	"github.com/liqotech/liqo/pkg/utils"
)

func getProviderInstance(providerType string) provider.InstallProviderInterface {
	switch providerType {
	case "kubeadm":
		return kubeadm.NewProvider()
	case "kind":
		return kind.NewProvider()
	case "k3s":
		return k3s.NewProvider()
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
	output, _, err := helmClient.GetChart(helmutils.LiqoChartFullName, &action.ChartPathOptions{Version: cArgs.Version})

	if err != nil {
		return err
	}

	providerValues := make(map[string]interface{})
	k.UpdateChartValues(providerValues)
	values, err := generateValues(output.Values, cArgs.CommonValues, providerValues)
	if err != nil {
		return err
	}

	raw, err := yaml.Marshal(values)
	if err != nil {
		return err
	}

	if cArgs.DumpValues {
		if err := utils.WriteFile(cArgs.DumpValuesPath, raw); err != nil {
			klog.Errorf("Unable to write the Values file in location: %s", cArgs.DumpValuesPath)
			return err
		}
	} else {
		chartSpec := helm.ChartSpec{
			// todo (palexster): Check if it ReleaseName and LiqoNamespace are really configurable
			ReleaseName:      helmutils.LiqoReleaseName,
			ChartName:        helmutils.LiqoChartFullName,
			Namespace:        helmutils.LiqoNamespace,
			ValuesYaml:       string(raw),
			DependencyUpdate: true,
			Timeout:          cArgs.Timeout,
			GenerateName:     false,
			CreateNamespace:  true,
			DryRun:           cArgs.DryRun,
			Devel:            cArgs.Devel,
			Wait:             true,
		}

		_, err = helmClient.InstallOrUpgradeChart(ctx, &chartSpec)
		if err != nil {
			return err
		}
	}
	return nil
}

func generateValues(chartValues, commonValues, providerValues map[string]interface{}) (map[string]interface{}, error) {
	intermediateValues, err := installutils.FusionMap(chartValues, commonValues)
	if err != nil {
		return nil, err
	}
	finalValues, err := installutils.FusionMap(intermediateValues, providerValues)
	if err != nil {
		return nil, err
	}
	return finalValues, nil
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
