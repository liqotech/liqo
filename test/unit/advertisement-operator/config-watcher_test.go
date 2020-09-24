package advertisement_operator

import (
	"context"
	configv1alpha1 "github.com/liqotech/liqo/api/config/v1alpha1"
	advtypes "github.com/liqotech/liqo/api/sharing/v1alpha1"
	"github.com/liqotech/liqo/pkg/crdClient"
	"github.com/stretchr/testify/assert"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/klog"
	api "k8s.io/kubernetes/pkg/apis/core"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strconv"
	"testing"
	"time"
)

func createFakeClusterConfig() configv1alpha1.ClusterConfig {
	return configv1alpha1.ClusterConfig{
		ObjectMeta: v1.ObjectMeta{
			Name: "fake-configuration",
		},
		Spec: configv1alpha1.ClusterConfigSpec{
			AdvertisementConfig: configv1alpha1.AdvertisementConfig{
				OutgoingConfig: configv1alpha1.BroadcasterConfig{
					ResourceSharingPercentage: 50,
					EnableBroadcaster:         true,
				},
				IngoingConfig: configv1alpha1.AdvOperatorConfig{
					MaxAcceptableAdvertisement: 5,
					AcceptPolicy:               configv1alpha1.AutoAcceptMax,
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
	configClient, err := configv1alpha1.CreateClusterConfigClient("", false)
	if err != nil {
		t.Fatal(err)
	}
	// create resources on cluster
	b.WatchConfiguration("", configClient)
	deadline := time.Now().Add(10 * time.Second)

	// Waiting for the correct initialization of the client
	for {
		if configClient.Store != nil || time.Now().After(deadline) {
			break
		} else {
			time.Sleep(500 * time.Millisecond)
		}
	}
	pNodes, vNodes, _, _, pods := createFakeResources()
	err = createResourcesOnCluster(b.LocalClient, pNodes, vNodes, pods)
	if err != nil {
		t.Fatal(err)
	}
	// launch watcher over cluster config
	_, err = configClient.Resource("clusterconfigs").Create(&clusterConfig, v1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	err = waitEvent(configClient, "clusterconfigs", clusterConfig.Name)
	if err != nil {
		t.Fatal(err)
	}
	// create advertisement on foreign cluster
	adv := prepareAdv(&b)
	_, err = b.RemoteClient.Resource("advertisements").Create(&adv, v1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	err = waitEvent(b.RemoteClient, "advertisements", adv.Name)
	if err != nil {
		t.Fatal(err)
	}
	// get adv resources
	cpu := adv.Spec.ResourceQuota.Hard.Cpu().Value()
	mem := adv.Spec.ResourceQuota.Hard.Memory().Value()
	// modify sharing percentage
	clusterConfig.Spec.AdvertisementConfig.OutgoingConfig.ResourceSharingPercentage = int32(30)
	_, err = configClient.Resource("clusterconfigs").Update(clusterConfig.Name, &clusterConfig, v1.UpdateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	err = waitEvent(b.RemoteClient, "advertisements", adv.Name)
	if err != nil {
		t.Fatal(err)
	}
	// get the new adv
	tmp, err := b.RemoteClient.Resource("advertisements").Get(adv.Name, v1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	adv2 := tmp.(*advtypes.Advertisement)
	cpu2 := adv2.Spec.ResourceQuota.Hard.Cpu().Value()
	mem2 := adv2.Spec.ResourceQuota.Hard.Memory().Value()
	assert.Less(t, cpu2, cpu)
	assert.Less(t, mem2, mem)
}

func testDisableBroadcaster(t *testing.T) {
	clusterConfig := createFakeClusterConfig()
	b := createBroadcaster(clusterConfig.Spec)
	// create fake client for configuration watcher
	configClient, err := configv1alpha1.CreateClusterConfigClient("", false)
	if err != nil {
		t.Fatal(err)
	}
	// launch watcher over cluster config
	b.WatchConfiguration("", configClient)
	deadline := time.Now().Add(10 * time.Second)
	for {
		if configClient.Store != nil || time.Now().After(deadline) {
			break
		} else {
			time.Sleep(500 * time.Millisecond)
		}
	}
	_, err = configClient.Resource("clusterconfigs").Create(&clusterConfig, v1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	err = waitEvent(configClient, "clusterconfigs", clusterConfig.Name)
	if err != nil {
		t.Fatal(err)
	}
	// create adv on foreign cluster
	adv := prepareAdv(&b)
	_, err = b.RemoteClient.Resource("advertisements").Create(&adv, v1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	err = waitEvent(b.RemoteClient, "advertisements", adv.Name)
	if err != nil {
		t.Fatal(err)
	}
	// disable advertisement
	clusterConfig.Spec.AdvertisementConfig.OutgoingConfig.EnableBroadcaster = false
	_, err = configClient.Resource("clusterconfigs").Update(clusterConfig.Name, &clusterConfig, v1.UpdateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	err = waitEvent(b.RemoteClient, "advertisements", adv.Name)
	if err != nil {
		t.Fatal(err)
	}
	// check adv has been deleted
	_, err = b.RemoteClient.Resource("advertisements").Get(adv.Name, v1.GetOptions{})
	assert.Equal(t, k8serrors.IsNotFound(err), true, "Advertisement has not been deleted")
}

func waitEvent(client *crdClient.CRDClient, resourcetype string, name string) error {
	var timeout int64 = 10
	watcher, err := client.Resource(resourcetype).Watch(v1.ListOptions{
		FieldSelector:  fields.OneTermEqualSelector(api.ObjectNameField, name).String(),
		TimeoutSeconds: &timeout,
	})
	if err != nil {
		return nil
	}
	for event := range watcher.ResultChan() {
		switch event.Type {
		case watch.Added:
			klog.Infof("Resource of type %s Created", resourcetype)
			return nil
		case watch.Modified:
			klog.Infof("Resource of type %s Modified", resourcetype)
			return nil
		case watch.Deleted:
			klog.Infof("Resource of type %s Deleted", resourcetype)
			return nil
		default:
			klog.Infof("Timeout Excedeed or Error Occurred watching resource of type %s", resourcetype)
			return nil
		}
	}
	return nil
}

func TestWatchAdvOperatorConfig(t *testing.T) {
	t.Run("testManageMaximumUpdate", testManageMaximumUpdate)
}

func testManageMaximumUpdate(t *testing.T) {
	r := createReconciler(0, 10, configv1alpha1.AutoAcceptMax)
	advList := advtypes.AdvertisementList{
		Items: []advtypes.Advertisement{},
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
	config := configv1alpha1.ClusterConfig{
		Spec: configv1alpha1.ClusterConfigSpec{
			AdvertisementConfig: configv1alpha1.AdvertisementConfig{
				IngoingConfig: configv1alpha1.AdvOperatorConfig{
					MaxAcceptableAdvertisement: int32(advCount),
					AcceptPolicy:               configv1alpha1.AutoAcceptMax,
				},
			},
		},
	}

	// TRUE TEST
	// test the true branch of ManageMaximumUpdate
	err, advToUpdate := r.ManageMaximumUpdate(config.Spec.AdvertisementConfig, &advList)
	assert.Nil(t, err)
	assert.NotEmpty(t, advToUpdate)
	assert.NotEmpty(t, advToUpdate.Items)
	assert.Equal(t, config.Spec.AdvertisementConfig, r.ClusterConfig)
	assert.Equal(t, int32(advCount), r.AcceptedAdvNum)
	for _, adv := range advToUpdate.Items {
		assert.Equal(t, advtypes.AdvertisementAccepted, adv.Status.AdvertisementStatus)
		r.UpdateAdvertisement(&adv)
	}

	// FALSE TEST
	// apply again the same configuration
	// we enter in the false branch of ManageMaximumUpdate but nothing should change
	err, advToUpdate = r.ManageMaximumUpdate(config.Spec.AdvertisementConfig, &advList)
	assert.Nil(t, err)
	assert.NotEmpty(t, advToUpdate)
	assert.Empty(t, advToUpdate.Items)
	assert.Equal(t, config.Spec.AdvertisementConfig, r.ClusterConfig)
	assert.Equal(t, int32(advCount), r.AcceptedAdvNum)

	// FALSE TEST with new config
	// check the new config is saved
	advCount = 10
	config = configv1alpha1.ClusterConfig{
		Spec: configv1alpha1.ClusterConfigSpec{
			AdvertisementConfig: configv1alpha1.AdvertisementConfig{
				IngoingConfig: configv1alpha1.AdvOperatorConfig{
					MaxAcceptableAdvertisement: int32(advCount),
					AcceptPolicy:               configv1alpha1.AutoAcceptMax,
				},
			},
		},
	}

	err, advToUpdate = r.ManageMaximumUpdate(config.Spec.AdvertisementConfig, &advList)
	assert.Nil(t, err)
	assert.NotEmpty(t, advToUpdate)
	assert.Empty(t, advToUpdate.Items)
	assert.Equal(t, config.Spec.AdvertisementConfig, r.ClusterConfig)
}
