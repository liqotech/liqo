package discovery

import (
	v1 "github.com/liqoTech/liqo/api/discovery/v1"
	"github.com/liqoTech/liqo/internal/discovery"
	search_domain_operator "github.com/liqoTech/liqo/internal/discovery/search-domain-operator"
	"gotest.tools/assert"
	v12 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
	"time"
)

func TestDns(t *testing.T) {
	t.Run("testDNS", testDns)
	t.Run("testSDCreation", testSDCreation)
	t.Run("testSDDelete", testSDDelete)
}

// ------
// tests if is able to get txtData from local DNS server
func testDns(t *testing.T) {
	targetTxts := []*discovery.TxtData{
		getTxtData(clientCluster, "dns-client-cluster"),
		getTxtData(serverCluster, "dns-server-cluster"),
	}

	txts, err := search_domain_operator.Wan("127.0.0.1:8053", registryDomain)
	assert.NilError(t, err, "Error during WAN DNS discovery")

	assert.Equal(t, len(txts), len(targetTxts))
	assert.Assert(t, func() bool {
		for _, target := range targetTxts {
			contains := false
			for _, txt := range txts {
				if txt.ID == target.ID && txt.ApiUrl == target.ApiUrl && txt.Namespace == target.Namespace {
					contains = true
				}
			}
			if !contains {
				return false
			}
		}
		return true
	}(), "Retrieved data does not match the expected DNS records")
}

// ------
// tests if SearchDomain operator is able to create ForeignClusters
func testSDCreation(t *testing.T) {
	tmp, err := clientCluster.client.Resource("foreignclusters").List(metav1.ListOptions{})
	assert.NilError(t, err, "Error listing ForeignClusters")
	fcs, ok := tmp.(*v1.ForeignClusterList)
	assert.Equal(t, ok, true)
	l := len(fcs.Items)

	sd := &v1.SearchDomain{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-domain",
		},
		Spec: v1.SearchDomainSpec{
			Domain:   registryDomain,
			AutoJoin: false,
		},
		Status: v1.SearchDomainStatus{
			ForeignClusters: []v12.ObjectReference{},
		},
	}
	_, err = clientCluster.client.Resource("searchdomains").Create(sd, metav1.CreateOptions{})
	assert.NilError(t, err, "Error creating SearchDomain")

	time.Sleep(1 * time.Second)

	tmp, err = clientCluster.client.Resource("searchdomains").List(metav1.ListOptions{})
	assert.NilError(t, err)
	sds, ok := tmp.(*v1.SearchDomainList)
	assert.Equal(t, ok, true)
	assert.Equal(t, len(sds.Items), 1, "SearchDomain not created")

	tmp, err = clientCluster.client.Resource("foreignclusters").List(metav1.ListOptions{})
	assert.NilError(t, err, "Error listing ForeignClusters")
	fcs, ok = tmp.(*v1.ForeignClusterList)
	assert.Equal(t, ok, true)
	l2 := len(fcs.Items)
	assert.Assert(t, l2-l == 2, "Foreign Cluster was not created")
}

// ------
// tests if SearchDomain operator is able to delete ForeignClusters
func testSDDelete(t *testing.T) {
	tmp, err := clientCluster.client.Resource("searchdomains").Get("test-domain", metav1.GetOptions{})
	assert.NilError(t, err)
	sd, ok := tmp.(*v1.SearchDomain)
	assert.Equal(t, ok, true)

	tmp, err = clientCluster.client.Resource("foreignclusters").List(metav1.ListOptions{})
	assert.NilError(t, err, "Error listing ForeignClusters")
	fcs, ok := tmp.(*v1.ForeignClusterList)
	assert.Equal(t, ok, true)
	l := len(fcs.Items)

	err = clientCluster.client.Resource("searchdomains").Delete(sd.Name, metav1.DeleteOptions{})
	assert.NilError(t, err)

	time.Sleep(1 * time.Second)

	tmp, err = clientCluster.client.Resource("searchdomains").List(metav1.ListOptions{})
	assert.NilError(t, err)
	sds, ok := tmp.(*v1.SearchDomainList)
	assert.Equal(t, ok, true)
	assert.Equal(t, len(sds.Items), 0, "SearchDomain not deleted")

	tmp, err = clientCluster.client.Resource("foreignclusters").List(metav1.ListOptions{})
	assert.NilError(t, err, "Error listing ForeignClusters")
	fcs, ok = tmp.(*v1.ForeignClusterList)
	assert.Equal(t, ok, true)
	l2 := len(fcs.Items)
	// delete doesn't work on testing, no control plan to delete object with owner reference
	assert.Assert(t, l2-l == 0, "Foreign Cluster was not deleted")

	// delete garbage
	for _, fcRef := range sd.Status.ForeignClusters {
		err = clientCluster.client.Resource("foreignclusters").Delete(fcRef.Name, metav1.DeleteOptions{})
		assert.NilError(t, err)
	}
}

// utility functions

func getTxtData(cluster *Cluster, id string) *discovery.TxtData {
	return &discovery.TxtData{
		ID:        id,
		Namespace: "default",
		ApiUrl:    "https://" + cluster.cfg.Host,
	}
}
