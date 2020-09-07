package advertisement_operator

import (
	discoveryv1alpha1 "github.com/liqoTech/liqo/api/discovery/v1alpha1"
	advtypes "github.com/liqoTech/liqo/api/sharing/v1alpha1"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
	"time"
)

func TestWatchAdvertisementAcceptance(t *testing.T) {
	// prepare resources for advertisement
	clusterConfig := createFakeClusterConfig()
	b := createBroadcaster(clusterConfig.Spec)

	// create fake peering request on cluster home
	pr := discoveryv1alpha1.PeeringRequest{
		TypeMeta: v1.TypeMeta{},
		ObjectMeta: v1.ObjectMeta{
			Name: b.PeeringRequestName,
		},
		Spec: discoveryv1alpha1.PeeringRequestSpec{
			ClusterID:     b.PeeringRequestName,
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
	time.Sleep(1 * time.Second)

	// after having created home adv on foreign cluster, start watching it
	go b.WatchAdvertisement(homeAdv.Name, "")

	// set adv status and update it: this will trigger the watcher
	homeAdv.Status.AdvertisementStatus = advtypes.AdvertisementAccepted
	_, err = b.RemoteClient.Resource("advertisements").Update(homeAdv.Name, &homeAdv, v1.UpdateOptions{})
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(5 * time.Second)

	tmp, err := b.DiscoveryClient.Resource("peeringrequests").Get(b.PeeringRequestName, v1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	pr2 := tmp.(*discoveryv1alpha1.PeeringRequest)
	assert.Equal(t, advtypes.AdvertisementAccepted, pr2.Status.AdvertisementStatus)
}
