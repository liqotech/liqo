package discovery

import (
	"github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/internal/discovery"
	search_domain_operator "github.com/liqotech/liqo/internal/discovery/search-domain-operator"
	"gotest.tools/assert"
	v12 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"
	"testing"
	"time"
)

func TestDns(t *testing.T) {
	// TODO: this tests will be re-enabled in a future pr, when auth discovery in WAN will be completed
	t.SkipNow()

	t.Run("testDNS", testDns)
	t.Run("testCNAME", testCname)
	t.Run("testSDCreation", testSDCreation)
	t.Run("testSDDelete", testSDDelete)
}

// ------
// tests if is able to get txtData from local DNS server
func testDns(t *testing.T) {
	targetTxts := []*discovery.TxtData{
		getTxtData("https://client.test.liqo.io.:"+getPort(clientCluster.cfg.Host), "dns-client-cluster"),
		getTxtData("https://server.test.liqo.io.:"+getPort(serverCluster.cfg.Host), "dns-server-cluster"),
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
// tests if is able to get txtData from local DNS server, with CNAME record
func testCname(t *testing.T) {
	hasCname = true

	targetTxts := []*discovery.TxtData{
		getTxtData("https://client.test.liqo.io.:"+getPort(clientCluster.cfg.Host), "dns-client-cluster"),
		getTxtData("https://server.test.liqo.io.:"+getPort(serverCluster.cfg.Host), "dns-server-cluster"),
	}

	txts, err := search_domain_operator.Wan("127.0.0.1:8053", registryDomain)
	assert.NilError(t, err, "Error during WAN DNS discovery")

	assert.Equal(t, len(txts), len(targetTxts))
	assert.Assert(t, func() bool {
		for _, target := range targetTxts {
			contains := false
			for _, txt := range txts {
				if txt.ID == target.ID && txt.Namespace == target.Namespace {
					contains = true
				}
			}
			if !contains {
				return false
			}
		}
		return true
	}(), "Retrieved data does not match the expected DNS records")

	hasCname = false
}

// ------
// tests if SearchDomain operator is able to create ForeignClusters
func testSDCreation(t *testing.T) {
	fcIncoming := &v1alpha1.ForeignCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "dns-server-cluster",
		},
		Spec: v1alpha1.ForeignClusterSpec{
			ClusterIdentity: v1alpha1.ClusterIdentity{
				ClusterID: "dns-server-cluster",
			},
			Namespace:     "default",
			Join:          false,
			ApiUrl:        serverCluster.cfg.Host,
			DiscoveryType: v1alpha1.IncomingPeeringDiscovery,
		},
		Status: v1alpha1.ForeignClusterStatus{
			Incoming: v1alpha1.Incoming{
				Joined:         true,
				PeeringRequest: &v12.ObjectReference{},
			},
		},
	}

	// create an IncomingPeering ForeignCluster to be overwritten
	_, err := clientCluster.client.Resource("foreignclusters").Create(fcIncoming, metav1.CreateOptions{})
	assert.NilError(t, err)

	sd := &v1alpha1.SearchDomain{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-domain",
		},
		Spec: v1alpha1.SearchDomainSpec{
			Domain:   registryDomain,
			AutoJoin: false,
		},
		Status: v1alpha1.SearchDomainStatus{
			ForeignClusters: []v12.ObjectReference{},
		},
	}
	_, err = clientCluster.client.Resource("searchdomains").Create(sd, metav1.CreateOptions{})
	assert.NilError(t, err, "Error creating SearchDomain")

	time.Sleep(5 * time.Second)

	tmp, err := clientCluster.client.Resource("searchdomains").List(metav1.ListOptions{})
	assert.NilError(t, err)
	sds, ok := tmp.(*v1alpha1.SearchDomainList)
	assert.Equal(t, ok, true)
	assert.Equal(t, len(sds.Items), 1, "SearchDomain not created")

	tmp, err = clientCluster.client.Resource("foreignclusters").List(metav1.ListOptions{
		LabelSelector: strings.Join([]string{"discovery-type", string(v1alpha1.WanDiscovery)}, "="),
	})
	assert.NilError(t, err, "Error listing ForeignClusters")
	fcs, ok := tmp.(*v1alpha1.ForeignClusterList)
	assert.Equal(t, ok, true)
	l := len(fcs.Items)
	assert.Assert(t, l == 2, "Foreign Cluster was not created")

	tmp, err = clientCluster.client.Resource("foreignclusters").Get(fcIncoming.Name, metav1.GetOptions{})
	assert.NilError(t, err)
	fc, ok := tmp.(*v1alpha1.ForeignCluster)
	assert.Assert(t, ok)
	assert.Equal(t, fc.Spec.DiscoveryType, v1alpha1.WanDiscovery, "Discovery type was not set to WAN")
}

// ------
// tests if SearchDomain operator is able to delete ForeignClusters
func testSDDelete(t *testing.T) {
	tmp, err := clientCluster.client.Resource("searchdomains").Get("test-domain", metav1.GetOptions{})
	assert.NilError(t, err)
	sd, ok := tmp.(*v1alpha1.SearchDomain)
	assert.Equal(t, ok, true)

	tmp, err = clientCluster.client.Resource("foreignclusters").List(metav1.ListOptions{})
	assert.NilError(t, err, "Error listing ForeignClusters")
	fcs, ok := tmp.(*v1alpha1.ForeignClusterList)
	assert.Equal(t, ok, true)
	l := len(fcs.Items)

	err = clientCluster.client.Resource("searchdomains").Delete(sd.Name, metav1.DeleteOptions{})
	assert.NilError(t, err)

	time.Sleep(1 * time.Second)

	tmp, err = clientCluster.client.Resource("searchdomains").List(metav1.ListOptions{})
	assert.NilError(t, err)
	sds, ok := tmp.(*v1alpha1.SearchDomainList)
	assert.Equal(t, ok, true)
	assert.Equal(t, len(sds.Items), 0, "SearchDomain not deleted")

	tmp, err = clientCluster.client.Resource("foreignclusters").List(metav1.ListOptions{})
	assert.NilError(t, err, "Error listing ForeignClusters")
	fcs, ok = tmp.(*v1alpha1.ForeignClusterList)
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

func getTxtData(url string, id string) *discovery.TxtData {
	return &discovery.TxtData{
		ID:        id,
		Namespace: "default",
		ApiUrl:    url,
	}
}

func getPort(url string) string {
	return strings.Split(url, ":")[1]
}
