package advertisement_operator

import (
	advv1 "github.com/liqoTech/liqo/api/advertisement-operator/v1"
	policyv1 "github.com/liqoTech/liqo/api/cluster-config/v1"
	advertisement_operator "github.com/liqoTech/liqo/internal/advertisement-operator"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
	"time"
)

func createFakeClusterConfig() policyv1.ClusterConfig {
	return policyv1.ClusterConfig{
		ObjectMeta: v1.ObjectMeta{
			Name: "fake-configuration",
		},
		Spec: policyv1.ClusterConfigSpec{
			AdvertisementConfig: policyv1.AdvertisementConfig{
				ResourceSharingPercentage:  50,
				MaxAcceptableAdvertisement: 5,
				AutoAccept:                 true,
				EnableBroadcaster:          true,
			},
		},
	}
}

func TestWatchBroadcasterClusterConfig(t *testing.T) {
	t.Run("testModifySharingPercentage", testModifySharingPercentage)
	t.Run("testDisableBroadcaster", testDisableBroadcaster)
}

func testModifySharingPercentage(t *testing.T) {
	clusterConfig := createFakeClusterConfig()
	b := createBroadcaster(clusterConfig.Spec)
	// create fake client for configuration watcher
	configClient, err := policyv1.CreateClusterConfigClient("")
	if err != nil {
		t.Fatal(err)
	}
	// create resources on cluster
	pNodes, vNodes, _, _, pods := createFakeResources()
	err = createResourcesOnCluster(b.LocalClient, pNodes, vNodes, pods)
	if err != nil {
		t.Fatal(err)
	}
	// launch watcher over cluster config
	b.WatchConfiguration("", configClient)
	_, err = configClient.Resource("clusterconfigs").Create(&clusterConfig, v1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(5 * time.Second)
	// create advertisement on foreign cluster
	adv := prepareAdv(b)
	_, err = b.RemoteClient.Resource("advertisements").Create(&adv, v1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(1 * time.Second)

	// get adv resources
	cpu := adv.Spec.ResourceQuota.Hard.Cpu().Value()
	mem := adv.Spec.ResourceQuota.Hard.Memory().Value()
	// modify sharing percentage
	clusterConfig.Spec.AdvertisementConfig.ResourceSharingPercentage = int32(30)
	_, err = configClient.Resource("clusterconfigs").Update(clusterConfig.Name, &clusterConfig, v1.UpdateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(5 * time.Second)
	// get the new adv
	tmp, err := b.RemoteClient.Resource("advertisements").Get(adv.Name, v1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	adv2 := tmp.(*advv1.Advertisement)
	cpu2 := adv2.Spec.ResourceQuota.Hard.Cpu().Value()
	mem2 := adv2.Spec.ResourceQuota.Hard.Memory().Value()
	assert.Less(t, cpu2, cpu)
	assert.Less(t, mem2, mem)
}

func testDisableBroadcaster(t *testing.T) {
	clusterConfig := createFakeClusterConfig()
	b := createBroadcaster(clusterConfig.Spec)
	// create fake client for configuration watcher
	configClient, err := policyv1.CreateClusterConfigClient("")
	if err != nil {
		t.Fatal(err)
	}
	// launch watcher over cluster config
	b.WatchConfiguration("", configClient)
	_, err = configClient.Resource("clusterconfigs").Create(&clusterConfig, v1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(5 * time.Second)
	// create adv on foreign cluster
	adv := prepareAdv(b)
	_, err = b.RemoteClient.Resource("advertisements").Create(&adv, v1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(1 * time.Second)

	// disable advertisement
	clusterConfig.Spec.AdvertisementConfig.EnableBroadcaster = false
	_, err = configClient.Resource("clusterconfigs").Update(clusterConfig.Name, &clusterConfig, v1.UpdateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(5 * time.Second)
	// check adv status has been set
	tmp, err := b.RemoteClient.Resource("advertisements").Get(adv.Name, v1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	adv2 := tmp.(*advv1.Advertisement)
	assert.Equal(t, advertisement_operator.AdvertisementDeleting, adv2.Status.AdvertisementStatus)
}
