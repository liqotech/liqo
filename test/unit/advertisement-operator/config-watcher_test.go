package advertisement_operator

import (
	"context"
	advv1 "github.com/liqoTech/liqo/api/advertisement-operator/v1"
	policyv1 "github.com/liqoTech/liqo/api/cluster-config/v1"
	advcontroller "github.com/liqoTech/liqo/internal/advertisement-operator"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strconv"
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
				BroadcasterConfig: policyv1.BroadcasterConfig{
					ResourceSharingPercentage: 50,
					EnableBroadcaster:         true,
				},
				AdvOperatorConfig: policyv1.AdvOperatorConfig{
					MaxAcceptableAdvertisement: 5,
					AcceptPolicy:               policyv1.AutoAcceptAll,
				},
			},
		},
	}
}

func TestWatchBroadcasterConfig(t *testing.T) {
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
	assert.Equal(t, advcontroller.AdvertisementDeleting, adv2.Status.AdvertisementStatus)
}

func TestWatchAdvOperatorConfig(t *testing.T) {
	t.Run("testManageMaximumUpdate", testManageMaximumUpdate)
	t.Run("testModifyAcceptPolicy", testModifyAcceptPolicy)
}

func testModifyAcceptPolicy(t *testing.T) {
	clusterConfig := createFakeClusterConfig()
	r := createReconcilerWithConfig(clusterConfig.Spec.AdvertisementConfig)

	// create some adv on the cluster
	for i := 0; i < 10; i++ {
		adv := createFakeAdv("cluster-"+strconv.Itoa(i), "default")
		_, err := r.AdvClient.Resource("advertisements").Create(adv, v1.CreateOptions{})
		if err != nil {
			t.Fatal(err)
		}
	}

	// create fake client for configuration watcher
	configClient, err := policyv1.CreateClusterConfigClient("")
	if err != nil {
		t.Fatal(err)
	}

	r.WatchConfiguration("", configClient)
	_, err = configClient.Resource("clusterconfigs").Create(&clusterConfig, v1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(5 * time.Second)

	clusterConfig.Spec.AdvertisementConfig.AcceptPolicy = policyv1.AutoAcceptWithinMaximum
	_, err = configClient.Resource("clusterconfigs").Update(clusterConfig.Name, &clusterConfig, v1.UpdateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(5 * time.Second)
	assert.Equal(t, clusterConfig.Spec.AdvertisementConfig, r.ClusterConfig)
}

func testModifyMaxAcceptableAdvertisement(t *testing.T) {

}

func testManageMaximumUpdate(t *testing.T) {
	r := createReconciler(0, 10, policyv1.AutoAcceptWithinMaximum)
	advList := advv1.AdvertisementList{
		Items: []advv1.Advertisement{},
	}

	advCount := 15

	// given a configuration with max 10 Advertisements, create 15 Advertisement: 10 should be accepted and 5 refused
	for i := 0; i < advCount; i++ {
		adv := createFakeAdv("cluster-"+strconv.Itoa(i), "default")
		err := r.Create(context.Background(), adv, &client.CreateOptions{})
		if err != nil {
			t.Fatal(err)
		}
		r.CheckAdvertisement(adv)
		r.UpdateAdvertisement(adv)
		advList.Items = append(advList.Items, *adv)
	}

	// the advList contains 10 accepted and 5 refused Adv
	// create a new configuration with MaxAcceptableAdv = 15
	// with the new configuration, check the 5 refused Adv are accepted
	config := policyv1.ClusterConfig{
		Spec: policyv1.ClusterConfigSpec{
			AdvertisementConfig: policyv1.AdvertisementConfig{
				AdvOperatorConfig: policyv1.AdvOperatorConfig{
					MaxAcceptableAdvertisement: int32(advCount),
					AcceptPolicy:               policyv1.AutoAcceptWithinMaximum,
				},
			},
		},
	}

	// TRUE TEST
	// test the true branch of ManageMaximumUpdate
	err, flag := r.ManageMaximumUpdate(config.Spec.AdvertisementConfig, &advList)
	assert.Nil(t, err)
	assert.True(t, flag)
	assert.Equal(t, config.Spec.AdvertisementConfig, r.ClusterConfig)
	assert.Equal(t, int32(advCount), r.AcceptedAdvNum)
	for _, adv := range advList.Items {
		assert.Equal(t, advcontroller.AdvertisementAccepted, adv.Status.AdvertisementStatus)
		r.UpdateAdvertisement(&adv)
	}

	// FALSE TEST
	// apply again the same configuration
	// we enter in the false branch of ManageMaximumUpdate but nothing should change
	err, flag = r.ManageMaximumUpdate(config.Spec.AdvertisementConfig, &advList)
	assert.Nil(t, err)
	assert.False(t, flag)
	assert.Equal(t, config.Spec.AdvertisementConfig, r.ClusterConfig)
	assert.Equal(t, int32(advCount), r.AcceptedAdvNum)

	// FALSE TEST with config.MaxAcceptableAdvertisement < r.AcceptedAdvNum
	// the advList contains 15 accepted Adv
	// create a new configuration with MaxAcceptableAdv = 10
	// with the new configuration, check 5 adv are deleted
	advCount = 10
	config = policyv1.ClusterConfig{
		Spec: policyv1.ClusterConfigSpec{
			AdvertisementConfig: policyv1.AdvertisementConfig{
				AdvOperatorConfig: policyv1.AdvOperatorConfig{
					MaxAcceptableAdvertisement: int32(advCount),
					AcceptPolicy:               policyv1.AutoAcceptWithinMaximum,
				},
			},
		},
	}

	err, flag = r.ManageMaximumUpdate(config.Spec.AdvertisementConfig, &advList)
	assert.Nil(t, err)
	assert.False(t, flag)
	assert.Equal(t, config.Spec.AdvertisementConfig, r.ClusterConfig)
	assert.Equal(t, int32(advCount), r.AcceptedAdvNum)
	var advList2 advv1.AdvertisementList
	_ = r.List(context.Background(), &advList2, &client.ListOptions{})
	assert.Equal(t, advCount, len(advList2.Items))
	for _, adv := range advList2.Items {
		assert.Equal(t, advcontroller.AdvertisementAccepted, adv.Status.AdvertisementStatus)
	}
}
