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
