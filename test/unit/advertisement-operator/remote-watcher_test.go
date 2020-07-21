package advertisement_operator

import (
	protocolv1 "github.com/liqoTech/liqo/api/advertisement-operator/v1"
	advertisement_operator "github.com/liqoTech/liqo/internal/advertisement-operator"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
	"time"
)

func TestWatchAdvertisement(t *testing.T) {
	// prepare resources for advertisement
	clusterConfig := createFakeClusterConfig()
	b := createBroadcaster(clusterConfig.Spec)

	// create fake home and foreign cluster advertisements
	homeAdv := prepareAdv(b)
	foreignAdv := homeAdv.DeepCopy()
	foreignAdv.Name = "advertisement-" + b.ForeignClusterId
	foreignAdv.Spec.ClusterId = b.ForeignClusterId

	_, err := b.LocalClient.Resource("advertisements").Create(foreignAdv, v1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = b.RemoteClient.Resource("advertisements").Create(&homeAdv, v1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(1 * time.Second)

	// after having created home adv on foreign cluster, start watching it
	go advertisement_operator.WatchAdvertisement(b.LocalClient, b.RemoteClient, homeAdv.Name, foreignAdv.Name)

	// set home cluster adv status and update it: this will trigger the watcher
	newPodCIDR := "1.2.3.4/16"
	homeAdv.Status.RemoteRemappedPodCIDR = newPodCIDR
	_, err = b.RemoteClient.Resource("advertisements").Update(homeAdv.Name, &homeAdv, v1.UpdateOptions{})
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(5 * time.Second)

	tmp, err := b.LocalClient.Resource("advertisements").Get(foreignAdv.Name, v1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	foreignAdv = tmp.(*protocolv1.Advertisement)
	assert.Equal(t, newPodCIDR, foreignAdv.Status.LocalRemappedPodCIDR)

	err = b.RemoteClient.Resource("advertisements").Delete(homeAdv.Name, v1.DeleteOptions{})
	assert.Nil(t, err)
}
