package dispatcher

import (
	policyv1 "github.com/liqoTech/liqo/api/cluster-config/v1"
	discoveryv1 "github.com/liqoTech/liqo/api/discovery/v1"
	"github.com/liqoTech/liqo/internal/dispatcher"
	"github.com/liqoTech/liqo/pkg/crdClient"
	"github.com/liqoTech/liqo/pkg/liqonet"
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
	k8sManagerLocal     ctrl.Manager
	k8sManagerRemote    ctrl.Manager
	testEnvLocal        *envtest.Environment
	testEnvRemote       *envtest.Environment
	configClusterClient *crdClient.CRDClient
	dOperator           *dispatcher.DispatcherReconciler
)

func TestMain(m *testing.M) {
	setupEnv()
	defer tearDown()
	err := setupDispatcherOperator()
	if err != nil {
		klog.Error(err)
		os.Exit(-1)
	}
	cacheStartedLocal := make(chan struct{})
	go func() {
		if err = k8sManagerLocal.Start(ctrl.SetupSignalHandler()); err != nil {
			klog.Error(err)
			panic(err)
		}
	}()
	started := k8sManagerLocal.GetCache().WaitForCacheSync(cacheStartedLocal)
	if !started {
		klog.Errorf("an error occurred while waiting for the chache to start")
		os.Exit(-1)
	}

	cacheStartedRemote := make(chan struct{})
	go func() {
		if err = k8sManagerRemote.Start(make(chan struct{})); err != nil {
			klog.Error(err)
			panic(err)
		}
	}()
	started = k8sManagerRemote.GetCache().WaitForCacheSync(cacheStartedRemote)
	if !started {
		klog.Errorf("an error occurred while waiting for the chache to start")
		os.Exit(-1)
	}

	time.Sleep(1 * time.Second)
	os.Exit(m.Run())
}

func getConfigClusterCRDClient(config *rest.Config) *crdClient.CRDClient {
	newConfig := config
	newConfig.ContentConfig.GroupVersion = &policyv1.GroupVersion
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
	testEnvLocal = &envtest.Environment{
		CRDDirectoryPaths: []string{filepath.Join("..", "..", "..", "deployments", "liqo_chart", "crds")},
	}

	configLocal, err := testEnvLocal.Start()
	if err != nil {
		klog.Error(err, "an error occurred while setting up the local testing environment")
		os.Exit(-1)
	}
	newConfig := &rest.Config{
		Host: configLocal.Host,
		// gotta go fast during tests -- we don't really care about overwhelming our test API server
		QPS:   1000.0,
		Burst: 2000.0,
	}

	testEnvRemote = &envtest.Environment{
		CRDDirectoryPaths: []string{filepath.Join("..", "..", "..", "deployments", "liqo_chart", "crds")},
	}

	configRemote, err := testEnvRemote.Start()
	if err != nil {
		klog.Error(err, "an error occurred while setting up the local testing environment")
		os.Exit(-1)
	}

	err = clientgoscheme.AddToScheme(scheme.Scheme)
	if err != nil {
		klog.Error(err, err.Error())
	}
	err = discoveryv1.AddToScheme(scheme.Scheme)
	if err != nil {
		klog.Error(err)
	}

	k8sManagerLocal, err = ctrl.NewManager(configLocal, ctrl.Options{
		Scheme:             scheme.Scheme,
		MetricsBindAddress: "0",
	})
	if err != nil {
		klog.Error(err)
		panic(err)
	}
	k8sManagerRemote, err = ctrl.NewManager(configRemote, ctrl.Options{
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
	err := testEnvRemote.Stop()
	if err != nil {
		klog.Error(err, err.Error())
		os.Exit(1)
	}
	err = testEnvLocal.Stop()
	if err != nil {
		klog.Error(err, err.Error())
		os.Exit(1)
	}
}

func getClusterConfig() *policyv1.ClusterConfig {
	return &policyv1.ClusterConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: "configuration",
		},
		Spec: policyv1.ClusterConfigSpec{
			AdvertisementConfig: policyv1.AdvertisementConfig{
				IngoingConfig: policyv1.AdvOperatorConfig{
					AcceptPolicy:               policyv1.AutoAcceptWithinMaximum,
					MaxAcceptableAdvertisement: 5,
				},
				OutgoingConfig: policyv1.BroadcasterConfig{
					ResourceSharingPercentage: 30,
					EnableBroadcaster:         true,
				},
			},
			DiscoveryConfig: policyv1.DiscoveryConfig{
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
			LiqonetConfig: policyv1.LiqonetConfig{
				ReservedSubnets: []string{"10.0.0.0/16"},
				VxlanNetConfig: liqonet.VxlanNetConfig{
					Network:    "",
					DeviceName: "",
					Port:       "",
					Vni:        "",
				},
			},
			DispatcherConfig: policyv1.DispatcherConfig{ResourcesToReplicate: []policyv1.Resource{{
				Group:    "liqonet.liqo.io",
				Version:  "v1alpha1",
				Resource: "networkconfigs",
			}}},
		},
	}
}
