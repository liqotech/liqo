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

package install

import (
	helm "github.com/mittwald/go-helm-client"
	"helm.sh/helm/v3/pkg/repo"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	"github.com/liqotech/liqo/pkg/liqoctl/install/provider"
	installutils "github.com/liqotech/liqo/pkg/liqoctl/install/utils"
)

// InitializeHelmClientWithRepo initiliazes an helm client for a given *rest.Config and adds the Liqo repository.
func InitializeHelmClientWithRepo(config *rest.Config, commonArgs *provider.CommonArguments) (helm.Client, error) {
	opt := &helm.RestConfClientOptions{
		Options: &helm.Options{
			Namespace:        installutils.LiqoNamespace,
			RepositoryConfig: liqoHelmConfigPath,
			RepositoryCache:  liqoHelmCachePath,
			Debug:            commonArgs.Debug,
			Linting:          false,
			DebugLog:         klog.V(4).Infof,
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

	return client, nil
}

func initLiqoRepo(helmClient helm.Client) error {
	// Define a public chart repository
	chartRepo := repo.Entry{
		URL:  liqoRepo,
		Name: liqoChartName,
	}

	if err := helmClient.AddOrUpdateChartRepo(chartRepo); err != nil {
		return err
	}

	if err := helmClient.UpdateChartRepos(); err != nil {
		return err
	}
	return nil
}
