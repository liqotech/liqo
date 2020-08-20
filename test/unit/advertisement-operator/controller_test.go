package advertisement_operator

import (
	v1 "github.com/liqoTech/liqo/api/advertisement-operator/v1"
	policyv1 "github.com/liqoTech/liqo/api/cluster-config/v1"
	advcontroller "github.com/liqoTech/liqo/internal/advertisement-operator"
	"github.com/liqoTech/liqo/pkg/crdClient"
	"github.com/stretchr/testify/assert"
	"strconv"
	"testing"
)

func createReconciler(acceptedAdv, maxAcceptableAdv int32, acceptPolicy policyv1.AcceptPolicy) advcontroller.AdvertisementReconciler {
	c, evRecorder := createFakeKubebuilderClient()
	// set the client in fake mode
	crdClient.Fake = true
	// create fake client for the home cluster
	advClient, err := v1.CreateAdvertisementClient("", nil)
	if err != nil {
		panic(err)
	}

	return advcontroller.AdvertisementReconciler{
		Client:           c,
		Scheme:           nil,
		EventsRecorder:   evRecorder,
		KubeletNamespace: "",
		KindEnvironment:  false,
		VKImage:          "",
		InitVKImage:      "",
		HomeClusterId:    "",
		AcceptedAdvNum:   acceptedAdv,
		ClusterConfig: policyv1.AdvertisementConfig{
			AdvOperatorConfig: policyv1.AdvOperatorConfig{
				MaxAcceptableAdvertisement: maxAcceptableAdv,
				AcceptPolicy:               acceptPolicy,
			},
		},
		AdvClient: advClient,
	}
}

func createReconcilerWithConfig(config policyv1.AdvertisementConfig) advcontroller.AdvertisementReconciler {
	c, evRecorder := createFakeKubebuilderClient()
	// set the client in fake mode
	crdClient.Fake = true
	// create fake client for the home cluster
	advClient, err := v1.CreateAdvertisementClient("", nil)
	if err != nil {
		panic(err)
	}

	return advcontroller.AdvertisementReconciler{
		Client:           c,
		Scheme:           nil,
		EventsRecorder:   evRecorder,
		KubeletNamespace: "",
		KindEnvironment:  false,
		VKImage:          "",
		InitVKImage:      "",
		HomeClusterId:    "",
		AcceptedAdvNum:   0,
		ClusterConfig: config,
		AdvClient: advClient,
	}
}

func TestCheckAdvertisement(t *testing.T) {
	t.Run("testAutoAcceptAll", testAutoAcceptAll)
	t.Run("testAutoAcceptWithinMaximum", testAutoAcceptWithinMaximum)
	t.Run("testAutoRefuseAll", testAutoRefuseAll)
	t.Run("testManualAccept", testManualAccept)
}
func testAutoAcceptAll(t *testing.T) {
	r := createReconciler(0, 10, policyv1.AutoAcceptAll)

	// given a configuration with AutoAcceptAll policy, create 15 Advertisements and check that they are all accepted, even if the Maximum is set to 10
	for i := 0; i < 15; i++ {
		adv := createFakeAdv("cluster-"+strconv.Itoa(i), "default")
		r.CheckAdvertisement(adv)
		assert.Equal(t, advcontroller.AdvertisementAccepted, adv.Status.AdvertisementStatus)
	}
	// check that the Adv counter has been incremented
	assert.Equal(t, int32(15), r.AcceptedAdvNum)
}

func testAutoAcceptWithinMaximum(t *testing.T) {
	r := createReconciler(0, 10, policyv1.AutoAcceptWithinMaximum)

	// given a configuration with max 10 Advertisements, create 10 Advertisements
	for i := 0; i < 10; i++ {
		adv := createFakeAdv("cluster-"+strconv.Itoa(i), "default")
		r.CheckAdvertisement(adv)
	}

	// create 10 more Advertisements and check that they are all refused, since the maximum has been reached
	for i := 10; i < 20; i++ {
		adv := createFakeAdv("cluster-"+strconv.Itoa(i), "default")
		r.CheckAdvertisement(adv)
		assert.Equal(t, advcontroller.AdvertisementRefused, adv.Status.AdvertisementStatus)
	}
	// check that the Adv counter has not been modified
	assert.Equal(t, int32(10), r.AcceptedAdvNum)
}

func testAutoRefuseAll(t *testing.T) {
	r := createReconciler(0, 10, policyv1.AutoRefuseAll)

	// given a configuration with max 10 Advertisements but RefuseAll policy, create 10 Advertisements and check they are refused
	for i := 0; i < 10; i++ {
		adv := createFakeAdv("cluster-"+strconv.Itoa(i), "default")
		r.CheckAdvertisement(adv)
		assert.Equal(t, advcontroller.AdvertisementRefused, adv.Status.AdvertisementStatus)
	}
	// check that the Adv counter has not been incremented
	assert.Equal(t, int32(0), r.AcceptedAdvNum)
}

func testManualAccept(t *testing.T) {
	r := createReconciler(0, 10, policyv1.ManualAccept)

	// given a configuration with max 10 Advertisements and ManualAccept policy, create 10 Advertisements and check they are refused
	for i := 0; i < 10; i++ {
		adv := createFakeAdv("cluster-"+strconv.Itoa(i), "default")
		r.CheckAdvertisement(adv)
		assert.Equal(t, advcontroller.AdvertisementRefused, adv.Status.AdvertisementStatus)
	}
	// check that the Adv counter has not been incremented
	assert.Equal(t, int32(0), r.AcceptedAdvNum)
}