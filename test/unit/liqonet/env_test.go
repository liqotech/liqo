package liqonet

import (
	"context"
	protocolv1 "github.com/liqoTech/liqo/api/advertisement-operator/v1"
	policyv1 "github.com/liqoTech/liqo/api/cluster-config/v1"
	liqonetv1 "github.com/liqoTech/liqo/api/tunnel-endpoint/v1"
	controllers "github.com/liqoTech/liqo/internal/liqonet"
	"github.com/liqoTech/liqo/pkg/crdClient"
	"github.com/liqoTech/liqo/pkg/liqonet"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"k8s.io/kubectl/pkg/scheme"
	"net"
	"os"
	"path/filepath"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"time"
)

var (
	k8sManager          ctrl.Manager
	testEnv             *envtest.Environment
	ctx                 = context.Background()
	tunEndpointCreator  *controllers.TunnelEndpointCreator
	configClusterClient *crdClient.CRDClient
)

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
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{filepath.Join("..", "..", "..", "deployments", "liqo_chart", "crds")},
	}

	config, err := testEnv.Start()
	newConfig := &rest.Config{
		Host: config.Host,
		// gotta go fast during tests -- we don't really care about overwhelming our test API server
		QPS:   1000.0,
		Burst: 2000.0,
	}
	if err != nil {
		klog.Error(err, "an error occurred while setting up the testing environment")
		os.Exit(-1)
	}
	err = clientgoscheme.AddToScheme(scheme.Scheme)
	if err != nil {
		klog.Error(err, err.Error())
	}
	err = protocolv1.AddToScheme(scheme.Scheme)
	if err != nil {
		klog.Error(err, err.Error())
	}
	err = liqonetv1.AddToScheme(scheme.Scheme)
	if err != nil {
		klog.Error(err, err.Error())
	}

	k8sManager, err = ctrl.NewManager(config, ctrl.Options{
		Scheme:             scheme.Scheme,
		MetricsBindAddress: "0",
	})
	if err != nil {
		klog.Error(err)
		panic(err)
	}
	configClusterClient = getConfigClusterCRDClient(newConfig)
	tunEndpointCreator = &controllers.TunnelEndpointCreator{
		Client:          k8sManager.GetClient(),
		Log:             ctrl.Log.WithName("controllers").WithName("TunnelEndpointCreator"),
		Scheme:          k8sManager.GetScheme(),
		ReservedSubnets: make(map[string]*net.IPNet),
		Configured:      make(chan bool, 1),
		IPManager: liqonet.IpManager{
			UsedSubnets:        make(map[string]*net.IPNet),
			FreeSubnets:        make(map[string]*net.IPNet),
			SubnetPerCluster:   make(map[string]*net.IPNet),
			ConflictingSubnets: make(map[string]*net.IPNet),
			Log:                ctrl.Log.WithName("IPAM"),
		},
		RetryTimeout: 30 * time.Second,
	}
	tunEndpointCreator.WatchConfiguration(newConfig, &policyv1.GroupVersion)
	err = tunEndpointCreator.SetupWithManager(k8sManager)
	if err != nil {
		klog.Error(err, err.Error())
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
	cc := getClusterConfig()
	_, err = configClusterClient.Resource("clusterconfigs").Create(cc, metav1.CreateOptions{})
	if err != nil {
		klog.Error(err, err.Error())
		os.Exit(-1)
	}
	adv := getAdv()
	err = tunEndpointCreator.Create(ctx, adv)
	if err != nil {
		klog.Error(err, err.Error())
		os.Exit(-1)
	}
	time.Sleep(1 * time.Second)
	klog.Info("setupenv finished")
}

func tearDown() {
	err := testEnv.Stop()
	if err != nil {
		klog.Error(err, err.Error())
		os.Exit(1)
	}
}

func getAdv() *protocolv1.Advertisement {
	return &protocolv1.Advertisement{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name: "testadv",
		},
		Spec: protocolv1.AdvertisementSpec{
			ClusterId: "cluster1",
			KubeConfigRef: corev1.ObjectReference{
				Kind:      "Secret",
				Namespace: "fake",
				Name:      "fake-kubeconfig",
			},
			LimitRange: corev1.LimitRangeSpec{Limits: []corev1.LimitRangeItem{}},
			Network: protocolv1.NetworkInfo{
				PodCIDR:            "10.96.0.0/16",
				GatewayIP:          "192.168.1.2",
				GatewayPrivateIP:   "10.0.0.1",
				SupportedProtocols: nil,
			},
			Timestamp:  metav1.NewTime(time.Now()),
			TimeToLive: metav1.NewTime(time.Now().Add(30 * time.Minute)),
		},
	}
}

func getClusterConfig() *policyv1.ClusterConfig {
	return &policyv1.ClusterConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: "configuration",
		},
		Spec: policyv1.ClusterConfigSpec{
			AdvertisementConfig: policyv1.AdvertisementConfig{
				AutoAccept:                 true,
				MaxAcceptableAdvertisement: 5,
				ResourceSharingPercentage:  30,
				EnableBroadcaster:          true,
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
				ReservedSubnets:  []string{"10.0.0.0/16"},
				GatewayPrivateIP: "192.168.1.1",
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
