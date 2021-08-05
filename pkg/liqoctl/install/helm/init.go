package helm

import (
	helm "github.com/mittwald/go-helm-client"
	"helm.sh/helm/v3/pkg/repo"
	"k8s.io/client-go/rest"
)

// InitializeHelmClientWithRepo initiliazes an helm client for a given *rest.Config and adds the Liqo repository.
func InitializeHelmClientWithRepo(config *rest.Config) (*helm.HelmClient, error) {
	opt := &helm.RestConfClientOptions{
		Options: &helm.Options{
			Namespace:        LiqoNamespace,
			RepositoryConfig: liqoHelmConfigPath,
			RepositoryCache:  liqoHelmCachePath,
			DebugLog:         nil,
		},
		RestConfig: config,
	}

	client, err := helm.NewClientFromRestConf(opt)
	if err != nil {
		return nil, err
	}

	if err := initLiqoRepo(client); err != nil {
		return nil, err
	}

	return client.(*helm.HelmClient), nil
}

func initLiqoRepo(helmClient helm.Client) error {
	// Define a public chart repository
	chartRepo := repo.Entry{
		Name: "liqo",
		URL:  liqoRepo,
	}

	if err := helmClient.AddOrUpdateChartRepo(chartRepo); err != nil {
		return err
	}

	if err := helmClient.UpdateChartRepos(); err != nil {
		return err
	}
	return nil
}
