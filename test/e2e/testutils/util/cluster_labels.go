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

package util

import (
	"fmt"

	"github.com/liqotech/liqo/test/e2e/testconsts"
)

// GetClusterLabels provides the labels which characterize the indexed cluster
// when exposed remotely as a virtual node.
func GetClusterLabels(index int) map[string]string {
	var clusterLabels map[string]string
	switch {
	case index == 0:
		clusterLabels = map[string]string{
			testconsts.ProviderKey: testconsts.ProviderAzure,
			testconsts.RegionKey:   testconsts.RegionA,
		}
	case index == 1:
		clusterLabels = map[string]string{
			testconsts.ProviderKey: testconsts.ProviderAWS,
			testconsts.RegionKey:   testconsts.RegionB,
		}
	case index == 2:
		clusterLabels = map[string]string{
			testconsts.ProviderKey: testconsts.ProviderGKE,
			testconsts.RegionKey:   testconsts.RegionC,
		}
	case index == 3:
		clusterLabels = map[string]string{
			testconsts.ProviderKey: testconsts.ProviderGKE,
			testconsts.RegionKey:   testconsts.RegionD,
		}
	default:
		panic(fmt.Errorf("there is no cluster with index '%d'", index))
	}
	return clusterLabels
}
