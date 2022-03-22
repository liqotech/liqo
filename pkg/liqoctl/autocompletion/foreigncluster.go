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

package autocompletion

import (
	"context"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/pkg/liqoctl/common"
)

// GetClusterNames returns the list of foreign cluster names that start with the given string.
func GetClusterNames(ctx context.Context, startWith string) ([]string, error) {
	restConfig, err := common.GetLiqoctlRestConf()
	if err != nil {
		return nil, err
	}

	k8sClient, err := client.New(restConfig, client.Options{})
	if err != nil {
		return nil, err
	}

	var foreignClusters discoveryv1alpha1.ForeignClusterList
	if err := k8sClient.List(ctx, &foreignClusters); err != nil {
		return nil, err
	}

	var clusterNames []string
	for i := range foreignClusters.Items {
		fc := &foreignClusters.Items[i]
		if strings.HasPrefix(fc.Name, startWith) {
			clusterNames = append(clusterNames, fc.Name)
		}
	}

	return clusterNames, nil
}
