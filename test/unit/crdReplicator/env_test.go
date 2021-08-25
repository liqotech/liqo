package crdReplicator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	configv1alpha1 "github.com/liqotech/liqo/apis/config/v1alpha1"
	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	crdreplicator "github.com/liqotech/liqo/internal/crdReplicator"
	"github.com/liqotech/liqo/pkg/consts"
	crdclient "github.com/liqotech/liqo/pkg/crdClient"
	"github.com/liqotech/liqo/pkg/utils/restcfg"
)

var (
	numberPeeringClusters = 1

	peeringIDTemplate           = "peering-cluster-"
	localClusterID              = "localClusterID"
	peeringClustersTestEnvs     = map[string]*envtest.Environment{}
	peeringClustersManagers     = map[string]ctrl.Manager{}
	peeringClustersDynClients   = map[string]dynamic.Interface{}
	peeringClustersDynFactories = map[string]dynamicinformer.DynamicSharedInformerFactory{}
	configClusterClient         *crdclient.CRDClient
	k8sManagerLocal             ctrl.Manager
	testEnvLocal                *envtest.Environment
	dOperator                   *crdreplicator.Controller
)

func TestMain(m *testing.M) {
	setupEnv()
	defer tearDown()
	startDispatcherOperator()
	time.Sleep(10 * time.Second)
	klog.Info("main set up")
	os.Exit(m.Run())
}

func startDispatcherOperator() {
	err := setupDispatcherOperator()
	if err != nil {
		klog.Error(err)
		os.Exit(-1)
	}
	ctx := context.TODO()
	go func() {
		if err = k8sManagerLocal.Start(ctrl.SetupSignalHandler()); err != nil {
			klog.Error(err)
			panic(err)
		}
	}()
	started := k8sManagerLocal.GetCache().WaitForCacheSync(ctx)
	if !started {
		klog.Errorf("an error occurred while waiting for the chache to start")
		os.Exit(-1)
	}
	configLocal := k8sManagerLocal.GetConfig()
	// gotta go fast during tests -- we don't really care about overwhelming our test API server
	restcfg.SetRateLimiterWithCustomParamenters(configLocal, 1000, 2000)
	err = dOperator.WatchConfiguration(configLocal, &configv1alpha1.GroupVersion)
	if err != nil {
		klog.Errorf("an error occurred while starting the configuration watcher of crdreplicator operator: %s", err)
		os.Exit(-1)
	}
	fc := getForeignClusterResource()
	_, err = dOperator.LocalDynClient.Resource(fcGVR).Create(context.TODO(), fc, metav1.CreateOptions{})
	if err != nil {
		klog.Error(err, err.Error())
		os.Exit(-1)
	}
}

func getConfigClusterCRDClient(config *rest.Config) *crdclient.CRDClient {
	newConfig := config
	newConfig.ContentConfig.GroupVersion = &configv1alpha1.GroupVersion
	newConfig.APIPath = "/apis"
	newConfig.NegotiatedSerializer = scheme.Codecs.WithoutConversion()
	newConfig.UserAgent = rest.DefaultKubernetesUserAgent()
	CRDclient, err := crdclient.NewFromConfig(newConfig)
	if err != nil {
		klog.Error(err, err.Error())
		os.Exit(1)
	}
	return CRDclient
}

func setupEnv() {
	err := discoveryv1alpha1.AddToScheme(scheme.Scheme)
	if err != nil {
		klog.Error(err)
	}
	//save the environment variables in the map
	for i := 1; i <= numberPeeringClusters; i++ {
		peeringClusterID := peeringIDTemplate + fmt.Sprintf("%d", i)
		peeringClustersTestEnvs[peeringClusterID] = &envtest.Environment{
			CRDDirectoryPaths: []string{filepath.Join("..", "..", "..", "deployments", "liqo", "crds")},
		}
	}
	//start the peering environments, save the managers, create dynamic clients
	for peeringClusterID, testEnv := range peeringClustersTestEnvs {
		config, err := testEnv.Start()
		if err != nil {
			klog.Errorf("%s -> an error occurred while setting test environment: %s", peeringClusterID, err)
			os.Exit(-1)
		} else {
			klog.Infof("%s -> created test environment", peeringClusterID)
		}
		manager, err := ctrl.NewManager(config, ctrl.Options{
			Scheme:             scheme.Scheme,
			MetricsBindAddress: "0",
		})
		if err != nil {
			klog.Errorf("%s -> an error occurred while creating the manager %s", peeringClusterID, err)
			os.Exit(-1)
		}
		peeringClustersManagers[peeringClusterID] = manager
		dynClient := dynamic.NewForConfigOrDie(manager.GetConfig())
		peeringClustersDynClients[peeringClusterID] = dynClient
		dynFac := dynamicinformer.NewFilteredDynamicSharedInformerFactory(dynClient, crdreplicator.ResyncPeriod, metav1.NamespaceAll, func(options *metav1.ListOptions) {
			//we want to watch only the resources that have been created by us on the remote cluster
			options.LabelSelector = crdreplicator.RemoteLabelSelector + "=" + localClusterID
		})
		peeringClustersDynFactories[peeringClusterID] = dynFac
	}
	//setup the local testing environment
	testEnvLocal = &envtest.Environment{
		CRDDirectoryPaths: []string{filepath.Join("..", "..", "..", "deployments", "liqo", "crds")},
	}
	configLocal, err := testEnvLocal.Start()
	if err != nil {
		klog.Error(err, "an error occurred while setting up the local testing environment")
	}
	klog.Infof("%s -> created test environmen", localClusterID)
	restcfg.SetRateLimiterWithCustomParamenters(configLocal, 1000, 2000)
	k8sManagerLocal, err = ctrl.NewManager(configLocal, ctrl.Options{
		Scheme:             scheme.Scheme,
		MetricsBindAddress: "0",
	})
	if err != nil {
		klog.Errorf("%s -> an error occurred while creating the manager %s", localClusterID, err)
		os.Exit(-1)
	}
	configClusterClient = getConfigClusterCRDClient(configLocal)
	cc := getClusterConfig()
	_, err = configClusterClient.Resource("clusterconfigs").Create(cc, &metav1.CreateOptions{})
	if err != nil {
		klog.Error(err, err.Error())
		os.Exit(-1)
	}
	klog.Info("setup of testing environments finished")
}

func tearDown() {
	//stop the peering testing environments
	for id, env := range peeringClustersTestEnvs {
		err := env.Stop()
		if err != nil {
			klog.Errorf("%s -> an error occurred while stopping peering environment test: %s", id, err)
		}
	}
	err := testEnvLocal.Stop()
	if err != nil {
		klog.Errorf("%s -> an error occurred while stopping local environment test: %s", localClusterID, err)
	}
}

func updateOwnership(ownership consts.OwnershipType) {
	tmp, err := configClusterClient.Resource("clusterconfigs").Get("configuration", &metav1.GetOptions{})
	if err != nil {
		klog.Error(err)
		os.Exit(1)
	}
	cc, _ := tmp.(*configv1alpha1.ClusterConfig)
	for i := range cc.Spec.DispatcherConfig.ResourcesToReplicate {
		cc.Spec.DispatcherConfig.ResourcesToReplicate[i].Ownership = ownership
	}
	_, err = configClusterClient.Resource("clusterconfigs").Update("configuration", cc, &metav1.UpdateOptions{})
	if err != nil {
		klog.Error(err)
		os.Exit(1)
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
				TTL:                 30,
			},
			LiqonetConfig: configv1alpha1.LiqonetConfig{
				PodCIDR:         "10.0.0.0/16",
				ServiceCIDR:     "10.96.0.0/12",
				ReservedSubnets: []configv1alpha1.CIDR{"10.0.0.0/16"},
				AdditionalPools: []configv1alpha1.CIDR{},
			},
			DispatcherConfig: configv1alpha1.DispatcherConfig{ResourcesToReplicate: []configv1alpha1.Resource{{
				GroupVersionResource: metav1.GroupVersionResource{
					Group:    netv1alpha1.GroupVersion.Group,
					Version:  netv1alpha1.GroupVersion.Version,
					Resource: "tunnelendpoints",
				},
				PeeringPhase: consts.PeeringPhaseAll,
				Ownership:    consts.OwnershipLocal,
			}}},
			AuthConfig: configv1alpha1.AuthConfig{
				EnableAuthentication: pointer.BoolPtr(false),
			},
		},
	}
}
