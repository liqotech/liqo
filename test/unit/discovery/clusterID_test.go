package discovery

import (
	"github.com/liqotech/liqo/pkg/clusterID"
	"gotest.tools/assert"
	"testing"
)

func testSetupClusterID(t *testing.T) {
	clID := clusterID.GetNewClusterID("", clientCluster.client.Client())
	err := clID.SetupClusterID("default")
	assert.NilError(t, err)
	assert.Assert(t, clID.GetClusterID() != "", "cluster id string has not been filled")
}
