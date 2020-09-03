package liqonet

import (
	"context"
	configv1alpha1 "github.com/liqoTech/liqo/api/config/v1alpha1"
	liqonetv1 "github.com/liqoTech/liqo/api/liqonet/v1"
	netv1alpha1 "github.com/liqoTech/liqo/api/liqonet/v1alpha1"
	advtypes "github.com/liqoTech/liqo/api/sharing/v1alpha1"
	controllers "github.com/liqoTech/liqo/internal/liqonet"
	"github.com/liqoTech/liqo/pkg/crdClient"
	"github.com/liqoTech/liqo/pkg/liqonet"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"k8s.io/kubectl/pkg/scheme"
	"os"
	"path/filepath"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"testing"
	"time"
)

var (
	k8sManager          ctrl.Manager
	testEnv             *envtest.Environment
	ctx                 = context.Background()
	tunEndpointCreator  *controllers.TunnelEndpointCreator
	configClusterClient *crdClient.CRDClient
	routeOperator       *controllers.RouteController
)

func TestMain(m *testing.M) {
	setupEnv()
	defer tearDown()

	err := setupRouteOperator()
	if err != nil {
		os.Exit(-2)
	}
	cacheStarted := make(chan struct{})
	go func() {
		if err = k8sManager.Start(ctrl.SetupSignalHandler()); err != nil {
			klog.Error(err)
			panic(err)
		}
	}()
	started := k8sManager.GetCache().WaitForCacheSync(cacheStarted)
	if !started {
		klog.Errorf("an error occurred while waiting for the chache to start")
		os.Exit(-1)
	}
	err = setupTunnelEndpointCreatorOperator()
	if err != nil {
		os.Exit(-1)
	}
	/*	adv := getAdv()
		err = tunEndpointCreator.Create(ctx, adv)
		if err != nil {
			klog.Error(err, err.Error())
			os.Exit(-2)
		}*/
	time.Sleep(1 * time.Second)
	os.Exit(m.Run())
}

func getConfigClusterCRDClient(config *rest.Config) *crdClient.CRDClient {
	newConfig := config
	newConfig.ContentConfig.GroupVersion = &configv1alpha1.GroupVersion
	newConfig.APIPath = "/apis"
	newConfig.NegotiatedSerializer = clientgoscheme.Codecs.WithoutConversion()
	newConfig.UserAgent = rest.DefaultKubernetesUserAgent()
	CRDclient, err := crdClient.NewFromConfig(config)
	if err != nil {
		klog.Error(err, err.Error())
		os.Exit(1)
	}
	return CRDclient
}

func setupEnv() {
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{filepath.Join("..", "..", "..", "deployments", "liqo_chart", "crds")},
	}

	config, err := testEnv.Start()
	if err != nil {
		klog.Error(err, "an error occurred while setting up the testing environment")
		os.Exit(-1)
	}
	newConfig := &rest.Config{
		Host: config.Host,
		// gotta go fast during tests -- we don't really care about overwhelming our test API server
		QPS:   1000.0,
		Burst: 2000.0,
	}
	err = clientgoscheme.AddToScheme(scheme.Scheme)
	if err != nil {
		klog.Error(err, err.Error())
	}
	err = advtypes.AddToScheme(scheme.Scheme)
	if err != nil {
		klog.Error(err, err.Error())
	}
	err = liqonetv1.AddToScheme(scheme.Scheme)
	if err != nil {
		klog.Error(err, err.Error())
	}

	_ = netv1alpha1.AddToScheme(scheme.Scheme)

	k8sManager, err = ctrl.NewManager(config, ctrl.Options{
		Scheme:             scheme.Scheme,
		MetricsBindAddress: "0",
	})
	if err != nil {
		klog.Error(err)
		panic(err)
	}
	configClusterClient = getConfigClusterCRDClient(newConfig)
	cc := getClusterConfig()
	_, err = configClusterClient.Resource("clusterconfigs").Create(cc, metav1.CreateOptions{})
	if err != nil {
		klog.Error(err, err.Error())
		os.Exit(-1)
	}
	klog.Info("setupenv finished")
}

func tearDown() {
	err := testEnv.Stop()
	if err != nil {
		klog.Error(err, err.Error())
		os.Exit(1)
	}
}

func getAdv() *advtypes.Advertisement {
	return &advtypes.Advertisement{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Advertisement",
			APIVersion: "sharing.liqo.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "testadv",
		},
		Spec: advtypes.AdvertisementSpec{
			ClusterId: "cluster1",
			KubeConfigRef: corev1.SecretReference{
				Namespace: "fake",
				Name:      "fake-kubeconfig",
			},
			LimitRange: corev1.LimitRangeSpec{Limits: []corev1.LimitRangeItem{}},
			Timestamp:  metav1.NewTime(time.Now()),
			TimeToLive: metav1.NewTime(time.Now().Add(30 * time.Minute)),
		},
	}
}

func getClusterConfig() *configv1alpha1.ClusterConfig {
	return &configv1alpha1.ClusterConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: "configuration",
		},
		Spec: configv1alpha1.ClusterConfigSpec{
			AdvertisementConfig: configv1alpha1.AdvertisementConfig{
				IngoingConfig: configv1alpha1.AdvOperatorConfig{
					AcceptPolicy:               configv1alpha1.AutoAcceptMax,
					MaxAcceptableAdvertisement: 5,
				},
				OutgoingConfig: configv1alpha1.BroadcasterConfig{
					ResourceSharingPercentage: 30,
					EnableBroadcaster:         true,
				},
			},
			DiscoveryConfig: configv1alpha1.DiscoveryConfig{
				AutoJoin:            true,
				Domain:              "local.",
				EnableAdvertisement: true,
				EnableDiscovery:     true,
				Name:                "MyLiqo",
				Port:                6443,
				Service:             "_liqo._tcp",
				UpdateTime:          3,
				WaitTime:            2,
				DnsServer:           "8.8.8.8:53",
			},
			LiqonetConfig: configv1alpha1.LiqonetConfig{
				ReservedSubnets: []string{"10.0.0.0/16"},
				PodCIDR:         "10.244.0.0/16",
				ServiceCIDR:     "10.1.0.0/12",
				VxlanNetConfig: liqonet.VxlanNetConfig{
					Network:    "",
					DeviceName: "",
					Port:       "",
					Vni:        "",
				},
			},
		},
	}
}
