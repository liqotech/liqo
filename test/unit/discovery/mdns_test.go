package discovery

import (
	"context"
	"github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/internal/discovery"
	"github.com/liqotech/liqo/pkg/auth"
	discoveryPkg "github.com/liqotech/liqo/pkg/discovery"
	"gotest.tools/assert"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/klog"
	"strconv"
	"strings"
	"testing"
	"time"
)

var txtData discovery.TxtData

func TestMdns(t *testing.T) {
	authSvc := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "auth-service",
		},
		Spec: v1.ServiceSpec{
			Type: v1.ServiceTypeNodePort,
			Ports: []v1.ServicePort{
				{
					Name:       "https",
					Port:       12345,
					TargetPort: intstr.FromInt(12345),
					NodePort:   32123,
				},
			},
		},
	}
	_, err := clientCluster.client.Client().CoreV1().Services(metav1.NamespaceDefault).Create(context.TODO(), authSvc, metav1.CreateOptions{})
	if err != nil {
		klog.Fatal(err)
	}

	t.Run("testTxtData", testTxtData)
	t.Run("testMDNS", testMdns)
	t.Run("testForeignClusterCreation", testForeignClusterCreation)
	t.Run("testTtl", testTtl)
}

// ------
// tests if txtData is correctly encoded/decode to/from DNS format
func testTxtData(t *testing.T) {
	txtData = discovery.TxtData{
		ID:        clientCluster.clusterId.GetClusterID(),
		Name:      "Cluster 1",
		Namespace: "default",
		ApiUrl:    "https://" + serverCluster.cfg.Host,
	}
	txt, err := txtData.Encode()
	assert.NilError(t, err, "Error encoding txtData to DNS format")

	txtData2 := &discovery.TxtData{}
	err = txtData2.Decode("127.0.0.1", strings.Split(serverCluster.cfg.Host, ":")[1], txt)
	assert.NilError(t, err, "Error decoding txtData from DNS format")
	assert.Equal(t, txtData, *txtData2, "TxtData before and after encoding doesn't match")
}

// ------
// tests if mDNS service works
func testMdns(t *testing.T) {
	service := "_liqo_auth._tcp"
	domain := "local."

	go clientCluster.discoveryCtrl.Register()
	go clientCluster.discoveryCtrl.StartGratuitousAnswers()

	time.Sleep(1 * time.Second)

	resultChan := make(chan discovery.DiscoverableData, 10)
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()
	go clientCluster.discoveryCtrl.Resolve(ctx, service, domain, resultChan)

	hasTxts := false
	select {
	case <-resultChan:
		hasTxts = true
	case <-ctx.Done():
		klog.Info("ctx.Done")
	case <-time.NewTimer(10 * time.Second).C:
		klog.Info("timeout")
	}
	assert.Assert(t, hasTxts)
}

// ------
// tests if ForeignCluster can be created from txtData
func testForeignClusterCreation(t *testing.T) {
	tmp, err := clientCluster.client.Resource("foreignclusters").List(metav1.ListOptions{})
	assert.NilError(t, err, "Error listing ForeignClusters")
	fcs, ok := tmp.(*v1alpha1.ForeignClusterList)
	assert.Equal(t, ok, true)
	l := len(fcs.Items)

	txts := &discovery.TxtData{
		ID:        "test",
		Name:      "Test Cluster 1",
		Namespace: "default",
		ApiUrl:    "http://" + serverCluster.cfg.Host,
	}

	clientCluster.discoveryCtrl.UpdateForeignLAN(discovery.NewDiscoveryData(discovery.NewAuthDataTest("127.0.0.1", 30001), &auth.ClusterInfo{
		ClusterID:      txts.ID,
		ClusterName:    txts.Name,
		GuestNamespace: txts.Namespace,
	}), discoveryPkg.TrustModeUntrusted)

	time.Sleep(1 * time.Second)

	tmp, err = clientCluster.client.Resource("foreignclusters").List(metav1.ListOptions{})
	assert.NilError(t, err, "Error listing ForeignClusters")
	fcs, ok = tmp.(*v1alpha1.ForeignClusterList)
	assert.Equal(t, ok, true)
	l2 := len(fcs.Items)
	assert.Assert(t, l2-l == 1, "Foreign Cluster was not created")

	tmp, err = clientCluster.client.Resource("foreignclusters").Get("test", metav1.GetOptions{})
	assert.NilError(t, err, "Error retrieving ForeignCluster")
	fc, ok := tmp.(*v1alpha1.ForeignCluster)
	assert.Equal(t, ok, true)
	assert.Equal(t, fc.Spec.AuthUrl, "fake://127.0.0.1:30001", "AuthUrl doesn't match the specified one")
	assert.Equal(t, fc.Spec.Namespace, "default", "Foreign Namesapce doesn't match the specified one")
	assert.Equal(t, fc.Spec.ClusterIdentity.ClusterID, txts.ID)
	assert.Equal(t, fc.Spec.ClusterIdentity.ClusterName, txts.Name)
}

// ------
// test TTL logic on LAN discovered ForeignClusters
func testTtl(t *testing.T) {
	fc := &v1alpha1.ForeignCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "fc-test-ttl",
			Labels: map[string]string{
				"discovery-type": string(discoveryPkg.LanDiscovery),
			},
			Annotations: map[string]string{
				discoveryPkg.LastUpdateAnnotation: strconv.Itoa(int(time.Now().Unix())),
			},
		},
		Spec: v1alpha1.ForeignClusterSpec{
			ClusterIdentity: v1alpha1.ClusterIdentity{
				ClusterID: "fc-test-ttl",
			},
			Namespace:     "default",
			Join:          false,
			AuthUrl:       "fake://127.0.0.1:30001",
			DiscoveryType: discoveryPkg.LanDiscovery,
			TrustMode:     discoveryPkg.TrustModeUntrusted,
		},
		Status: v1alpha1.ForeignClusterStatus{
			Ttl: 1,
		},
	}

	_, err := clientCluster.client.Resource("foreignclusters").Create(fc, metav1.CreateOptions{})
	assert.NilError(t, err)

	retry := 10
	for {
		time.Sleep(250 * time.Millisecond)

		err = clientCluster.discoveryCtrl.CollectGarbage()
		assert.NilError(t, err)

		time.Sleep(250 * time.Millisecond)

		_, err = clientCluster.client.Resource("foreignclusters").Get(fc.Name, metav1.GetOptions{})

		retry--
		if errors.IsNotFound(err) || retry <= 0 {
			assert.Assert(t, errors.IsNotFound(err), "this resource was not deleted by garbage collector")
			break
		}
	}
}
