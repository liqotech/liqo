package advertisement_operator

import (
	discoveryv1alpha1 "github.com/liqotech/liqo/api/discovery/v1alpha1"
	advtypes "github.com/liqotech/liqo/api/sharing/v1alpha1"
	pkg "github.com/liqotech/liqo/pkg/virtualKubelet"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
	"time"
)

func TestWatchAdvertisementAcceptance(t *testing.T) {
	// prepare resources for advertisement
	clusterConfig := createFakeClusterConfig()
	b := createBroadcaster(clusterConfig.Spec)
	// reset the store, which is needs to be created in WatchAdvertisement method
	b.RemoteClient.Store = nil
	// launch the watcher
	advName := pkg.AdvertisementPrefix + b.HomeClusterId
	go b.WatchAdvertisement(advName)
	// Waiting for the correct initialization of the client
	deadline := time.Now().Add(10 * time.Second)
	for {
		if b.RemoteClient.Store != nil || time.Now().After(deadline) {
			break
		} else {
			time.Sleep(500 * time.Millisecond)
		}
	}
	// create fake peering request on cluster home
	pr := discoveryv1alpha1.PeeringRequest{
		TypeMeta: v1.TypeMeta{},
		ObjectMeta: v1.ObjectMeta{
			Name: b.PeeringRequestName,
		},
		Spec: discoveryv1alpha1.PeeringRequestSpec{
			ClusterIdentity: discoveryv1alpha1.ClusterIdentity{
				ClusterID: b.PeeringRequestName,
			},
			Namespace:     "test",
			KubeConfigRef: nil,
		},
		Status: discoveryv1alpha1.PeeringRequestStatus{},
	}
	_, err := b.DiscoveryClient.Resource("peeringrequests").Create(&pr, v1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}

	// create fake advertisement on cluster foreign
	homeAdv := prepareAdv(&b)
	_, err = b.RemoteClient.Resource("advertisements").Create(&homeAdv, v1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	err = waitEvent(b.RemoteClient, "advertisements", advName)
	if err != nil {
		t.Fatal(err)
	}

	// set adv status and update it: this will trigger the watcher
	homeAdv.Status.AdvertisementStatus = advtypes.AdvertisementAccepted
	_, err = b.RemoteClient.Resource("advertisements").Update(homeAdv.Name, &homeAdv, v1.UpdateOptions{})
	if err != nil {
		t.Fatal(err)
	}

	err = waitEvent(b.RemoteClient, "advertisements", advName)
	if err != nil {
		t.Fatal(err)
	}

	tmp, err := b.DiscoveryClient.Resource("peeringrequests").Get(b.PeeringRequestName, v1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	pr2 := tmp.(*discoveryv1alpha1.PeeringRequest)
	assert.Equal(t, advtypes.AdvertisementAccepted, pr2.Status.AdvertisementStatus)
}
