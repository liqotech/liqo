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

package common

import (
	"log"
	"os"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli"

	installutils "github.com/liqotech/liqo/pkg/liqoctl/install/utils"
)

// LiqoHelmClient is a wrapper around the Helm client.
type LiqoHelmClient struct {
	release   string
	getValues func() (map[string]interface{}, error)
}

// NewLiqoHelmClient creates a new instance of LiqoHelmClient.
func NewLiqoHelmClient() (*LiqoHelmClient, error) {
	lhc := &LiqoHelmClient{release: installutils.LiqoReleaseName}
	settings := cli.New()
	clientConfig := &action.Configuration{}
	helmdriver := os.Getenv("HELM_DRIVER")
	err := clientConfig.Init(settings.RESTClientGetter(), installutils.LiqoNamespace, helmdriver, log.Printf)
	if err != nil {
		return nil, err
	}
	lhc.getValues = func() (map[string]interface{}, error) { return action.NewGetValues(clientConfig).Run(lhc.release) }
	return lhc, nil
}

// GetClusterLabels returns a map of cluster labels.
func (lhc *LiqoHelmClient) GetClusterLabels() (map[string]string, error) {
	values, err := lhc.getValues()
	if err != nil {
		return nil, err
	}

	result, err := ExtractValuesFromNestedMaps(values, "discovery", "config", "clusterLabels")
	if err != nil {
		return nil, err
	}

	clusterLabels := make(map[string]string)
	tmp := result.(map[string]interface{})
	for k, v := range tmp {
		clusterLabels[k] = v.(string)
	}
	return clusterLabels, nil
}
