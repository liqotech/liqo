package advertisement_operator

import (
	configv1alpha1 "github.com/liqoTech/liqo/api/config/v1alpha1"
	advtypes "github.com/liqoTech/liqo/api/sharing/v1alpha1"
	advop "github.com/liqoTech/liqo/internal/advertisement-operator"
	"github.com/liqoTech/liqo/pkg/crdClient"
	"github.com/stretchr/testify/assert"
	v12 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"strconv"
	"testing"
)

func createReconciler(acceptedAdv, maxAcceptableAdv int32, acceptPolicy configv1alpha1.AcceptPolicy) advop.AdvertisementReconciler {
	c, evRecorder := createFakeKubebuilderClient()
	// set the client in fake mode
	crdClient.Fake = true
	// create fake client for the home cluster
	advClient, err := advtypes.CreateAdvertisementClient("", nil)
	if err != nil {
		panic(err)
	}

	return advop.AdvertisementReconciler{
		Client:           c,
		Scheme:           nil,
		EventsRecorder:   evRecorder,
		KubeletNamespace: "",
		KindEnvironment:  false,
		VKImage:          "",
		InitVKImage:      "",
		HomeClusterId:    "",
		AcceptedAdvNum:   acceptedAdv,
		ClusterConfig: configv1alpha1.AdvertisementConfig{
			IngoingConfig: configv1alpha1.AdvOperatorConfig{
				MaxAcceptableAdvertisement: maxAcceptableAdv,
				AcceptPolicy:               acceptPolicy,
			},
		},
		AdvClient: advClient,
	}
}

func TestCheckAdvertisement(t *testing.T) {
	t.Run("testAutoAcceptMax", testAutoAcceptMax)
	t.Run("testManualAccept", testManualAccept)
	t.Run("testRefuseInvalidAdvertisement", testRefuseInvalidAdvertisement)
}

func testAutoAcceptMax(t *testing.T) {
	r := createReconciler(0, 10, configv1alpha1.AutoAcceptMax)

	// given a configuration with max 10 Advertisements, create 10 Advertisements
	for i := 0; i < 10; i++ {
		adv := createFakeAdv("cluster-"+strconv.Itoa(i), "default")
		r.CheckAdvertisement(adv)
	}

	// create 5 more Advertisements and check that they are all refused, since the maximum has been reached
	for i := 10; i < 15; i++ {
		adv := createFakeAdv("cluster-"+strconv.Itoa(i), "default")
		r.CheckAdvertisement(adv)
		assert.Equal(t, advop.AdvertisementRefused, adv.Status.AdvertisementStatus)
	}
	// check that the Adv counter has not been modified
	assert.Equal(t, int32(10), r.AcceptedAdvNum)
}

func testManualAccept(t *testing.T) {
	r := createReconciler(0, 10, configv1alpha1.ManualAccept)

	// given a configuration with max 10 Advertisements and ManualAccept policy, create 5 Advertisements and check they are refused
	for i := 0; i < 5; i++ {
		adv := createFakeAdv("cluster-"+strconv.Itoa(i), "default")
		r.CheckAdvertisement(adv)
		assert.Equal(t, advop.AdvertisementRefused, adv.Status.AdvertisementStatus)
	}
	// check that the Adv counter has not been incremented
	assert.Equal(t, int32(0), r.AcceptedAdvNum)
}

func testRefuseInvalidAdvertisement(t *testing.T) {
	r := createReconciler(0, 10, configv1alpha1.AutoAcceptMax)

	// create 5 advertisements with negative values in ResourceQuota field and check they are refused
	for i := 1; i <= 5; i++ {
		quota := v12.ResourceQuotaSpec{
			Hard: map[v12.ResourceName]resource.Quantity{
				v12.ResourceCPU:    resource.MustParse(strconv.Itoa(-i)),
				v12.ResourceMemory: resource.MustParse(strconv.Itoa(-i)),
			},
		}
		adv := createFakeInvalidAdv("cluster-"+strconv.Itoa(i), "default", quota)
		r.CheckAdvertisement(adv)
		assert.Equal(t, advop.AdvertisementRefused, adv.Status.AdvertisementStatus)
	}
	// check that the Adv counter has not been incremented
	assert.Equal(t, int32(0), r.AcceptedAdvNum)
}
