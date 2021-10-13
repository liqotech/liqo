// Copyright 2019-2021 The Liqo Authors
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
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	helm "github.com/mittwald/go-helm-client"
	"gopkg.in/yaml.v3"
	"helm.sh/helm/v3/pkg/action"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	"github.com/liqotech/liqo/pkg/liqoctl/install/aks"
	"github.com/liqotech/liqo/pkg/liqoctl/install/eks"
	"github.com/liqotech/liqo/pkg/liqoctl/install/gke"
	"github.com/liqotech/liqo/pkg/liqoctl/install/k3s"
	"github.com/liqotech/liqo/pkg/liqoctl/install/kind"
	"github.com/liqotech/liqo/pkg/liqoctl/install/kubeadm"
	"github.com/liqotech/liqo/pkg/liqoctl/install/openshift"
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
	case "openshift":
		return openshift.NewProvider()
	default:
		return nil
	}
}

func initHelmClient(config *rest.Config, arguments *provider.CommonArguments) (*helm.HelmClient, error) {
	helmClient, err := InitializeHelmClientWithRepo(config, arguments)
	if err != nil {
		fmt.Printf("Unable to create helmClient: %s", err)
		return nil, err
	}
	return helmClient, nil
}

func installOrUpdate(ctx context.Context, helmClient *helm.HelmClient, k provider.InstallProviderInterface, cArgs *provider.CommonArguments) error {
	if cArgs.Version == "" {
		version, err := findNewestRelease()
		if err != nil {
			return err
		}
		cArgs.Version = version
	}

	output, _, err := helmClient.GetChart(cArgs.ChartPath, &action.ChartPathOptions{Version: cArgs.Version})

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
			ReleaseName:      installutils.LiqoReleaseName,
			ChartName:        cArgs.ChartPath,
			Namespace:        installutils.LiqoNamespace,
			ValuesYaml:       string(raw),
			DependencyUpdate: true,
			Timeout:          cArgs.Timeout,
			GenerateName:     false,
			CreateNamespace:  true,
			DryRun:           cArgs.DryRun,
			Devel:            cArgs.Devel,
			Wait:             true,
			Version:          cArgs.Version,
		}

		// provide the possibility to exit installation on context cancellation
		errCh := make(chan error)
		defer close(errCh)
		go func() {
			_, err = helmClient.InstallOrUpgradeChart(ctx, &chartSpec)
			errCh <- err
		}()

		select {
		case err = <-errCh:
			if err != nil {
				return err
			}
		case <-ctx.Done():
			return nil
		}
	}
	return nil
}

type tagsAPITag struct {
	Name string
}

type tagsAPIResponse struct {
	Next    string
	Results []tagsAPITag
}

// findNewestRelease queries the Docker Hub and gets the first release tag (i.e. not a release candidate, alpha, etc)
func findNewestRelease() (string, error) {
	page := "https://registry.hub.docker.com/v2/repositories/liqo/liqo-controller-manager/tags/"
	for {
		resp, err := http.Get(page)
		if err != nil {
			return "", err
		}
		respJson, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return "", err
		}
		var response tagsAPIResponse
		err = json.Unmarshal(respJson, &response)
		if err != nil {
			return "", err
		}

		for i := range response.Results {
			tag := strings.ToLower(response.Results[i].Name)
			if tag != "latest" &&
				!strings.Contains(tag, "rc") &&
				!strings.Contains(tag, "alpha") {
				return tag, nil
			}
		}
		// No tags found in this page; visit the next one
		if response.Next == "" {
			return "", fmt.Errorf("no release found in Docker tags")
		}
		page = response.Next
	}
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
