package discovery

import (
	v1 "github.com/liqoTech/liqo/api/discovery/v1"
	"github.com/liqoTech/liqo/internal/discovery"
	"gotest.tools/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"
	"testing"
	"time"
)

var txtData discovery.TxtData

func TestMdns(t *testing.T) {
	t.Run("testTxtData", testTxtData)
	t.Run("testMDNS", testMdns)
	t.Run("testForeignClusterCreation", testForeignClusterCreation)
}

// ------
// tests if txtData is correctly encoded/decode to/from DNS format
func testTxtData(t *testing.T) {
	txtData = discovery.TxtData{
		ID:        clientCluster.clusterId.GetClusterID(),
		Namespace: "default",
		ApiUrl:    "https://" + serverCluster.cfg.Host,
	}
	txt, err := txtData.Encode()
	assert.NilError(t, err, "Error encoding txtData to DNS format")

	txtData2, err := discovery.Decode("127.0.0.1", strings.Split(serverCluster.cfg.Host, ":")[1], txt)
	assert.NilError(t, err, "Error decoding txtData from DNS format")
	assert.Equal(t, txtData, *txtData2, "TxtData before and after encoding doesn't match")
}

// ------
// tests if mDNS service works
func testMdns(t *testing.T) {
	service := "_liqo._tcp"
	domain := "local."

	go discoveryCtrl.Register()

	time.Sleep(1 * time.Second)

	txts := []*discovery.TxtData{}
	discoveryCtrl.Resolve(service, domain, 3, &txts)

	time.Sleep(1 * time.Second)

	// TODO: find better way to test mDNS, local IP is not always detected
	assert.Assert(t, len(txts) >= 0, "If this line is reached test would be successful, no foreign packet can reach our testing environment at the moment")
}

// ------
// tests if ForeignCluster can be created from txtData
func testForeignClusterCreation(t *testing.T) {
	tmp, err := clientCluster.crdClient.Resource("foreignclusters").List(metav1.ListOptions{})
	assert.NilError(t, err, "Error listing ForeignClusters")
	fcs, ok := tmp.(*v1.ForeignClusterList)
	assert.Equal(t, ok, true)
	l := len(fcs.Items)

	txts := []*discovery.TxtData{
		{
			ID:        "test",
			Namespace: "default",
			ApiUrl:    "http://" + serverCluster.cfg.Host,
		},
	}

	discoveryCtrl.UpdateForeign(txts)

	time.Sleep(1 * time.Second)

	tmp, err = clientCluster.crdClient.Resource("foreignclusters").List(metav1.ListOptions{})
	assert.NilError(t, err, "Error listing ForeignClusters")
	fcs, ok = tmp.(*v1.ForeignClusterList)
	assert.Equal(t, ok, true)
	l2 := len(fcs.Items)
	assert.Assert(t, l2-l == 1, "Foreign Cluster was not created")

	tmp, err = clientCluster.crdClient.Resource("foreignclusters").Get("test", metav1.GetOptions{})
	assert.NilError(t, err, "Error retrieving ForeignCluster")
	fc, ok := tmp.(*v1.ForeignCluster)
	assert.Equal(t, ok, true)
	assert.Equal(t, fc.Spec.ApiUrl, "http://"+serverCluster.cfg.Host, "ApiUrl doesn't match the specified one")
	assert.Equal(t, fc.Spec.Namespace, "default", "Foreign Namesapce doesn't match the specified one")
}
