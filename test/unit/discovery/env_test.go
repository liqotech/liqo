package discovery

import (
	policyv1 "github.com/liqoTech/liqo/api/cluster-config/v1"
	v1 "github.com/liqoTech/liqo/api/discovery/v1"
	foreign_cluster_operator "github.com/liqoTech/liqo/internal/discovery/foreign-cluster-operator"
	peering_request_operator "github.com/liqoTech/liqo/internal/peering-request-operator"
	"github.com/liqoTech/liqo/pkg/clusterID"
	"github.com/liqoTech/liqo/pkg/crdClient/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/klog"
	"os"
	"path/filepath"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"time"
)

type Cluster struct {
	env          *envtest.Environment
	cfg          *rest.Config
	crdClient    *v1alpha1.CRDClient
	fcReconciler *foreign_cluster_operator.ForeignClusterReconciler
	prReconciler *peering_request_operator.PeeringRequestReconciler
	clusterId    *clusterID.ClusterID
}

func getClientCluster() *Cluster {
	cluster, mgr := getCluster()
	cluster.clusterId = clusterID.GetNewClusterID("client-cluster", cluster.crdClient.Client())
	cluster.fcReconciler = foreign_cluster_operator.GetFCReconciler(
		mgr.GetScheme(),
		"default",
		cluster.crdClient,
		cluster.clusterId,
		1*time.Minute,
	)
	err := cluster.fcReconciler.SetupWithManager(mgr)
	if err != nil {
		klog.Error(err, err.Error())
		os.Exit(1)
	}

	go func() {
		err = mgr.Start(stopChan)
		if err != nil {
			klog.Error(err, err.Error())
			os.Exit(1)
		}
	}()
	return cluster
}

func getServerCluster() *Cluster {
	cluster, mgr := getCluster()
	cluster.clusterId = clusterID.GetNewClusterID("server-cluster", cluster.crdClient.Client())
	cluster.prReconciler = peering_request_operator.GetPRReconciler(
		mgr.GetScheme(),
		cluster.crdClient,
		"default",
		cluster.clusterId,
		"liqo-config",
		"broadcaster",
		"br-sa",
	)
	err := cluster.prReconciler.SetupWithManager(mgr)
	if err != nil {
		klog.Error(err, err.Error())
		os.Exit(1)
	}

	go func() {
		err = mgr.Start(stopChan)
		if err != nil {
			klog.Error(err, err.Error())
			os.Exit(1)
		}
	}()
	return cluster
}

func getCluster() (*Cluster, manager.Manager) {
	cluster := &Cluster{}

	cluster.env = &envtest.Environment{
		CRDDirectoryPaths: []string{filepath.Join("..", "..", "..", "deployments", "liqo_chart", "crds")},
	}

	/*
		Then, we start the envtest cluster.
	*/
	var err error
	cluster.cfg, err = cluster.env.Start()
	if err != nil {
		klog.Error(err, err.Error())
		os.Exit(1)
	}

	cluster.cfg.ContentConfig.GroupVersion = &v1.GroupVersion
	cluster.cfg.APIPath = "/apis"
	cluster.cfg.NegotiatedSerializer = scheme.Codecs.WithoutConversion()
	cluster.cfg.UserAgent = rest.DefaultKubernetesUserAgent()

	err = v1.AddToScheme(scheme.Scheme)
	if err != nil {
		klog.Error(err, err.Error())
		os.Exit(1)
	}

	cluster.crdClient, err = v1alpha1.NewFromConfig(cluster.cfg)
	if err != nil {
		klog.Error(err, err.Error())
		os.Exit(1)
	}
	k8sManager, err := ctrl.NewManager(cluster.cfg, ctrl.Options{
		Scheme:             scheme.Scheme,
		MetricsBindAddress: "0", // this avoids port binding collision
	})
	if err != nil {
		klog.Error(err, err.Error())
		os.Exit(1)
	}

	// creates empty CaData secret
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "ca-data",
		},
		Data: map[string][]byte{
			"ca.crt": []byte(""),
		},
	}
	_, err = cluster.crdClient.Client().CoreV1().Secrets("default").Create(secret)
	if err != nil {
		klog.Error(err, err.Error())
		os.Exit(1)
	}

	getLiqoConfig(cluster.crdClient.Client())
	getClusterConfig(*cluster.cfg)

	return cluster, k8sManager
}

func getLiqoConfig(client kubernetes.Interface) {
	// default config values
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: "liqo-config",
		},
		Data: map[string]string{
			"clusterID":        "cluster-1",
			"podCIDR":          "10.244.0.0/16",
			"serviceCIDR":      "10.96.0.0/12",
			"gatewayPrivateIP": "10.244.2.47",
			"gatewayIP":        "10.251.0.1",
		},
	}
	_, err := client.CoreV1().ConfigMaps("default").Create(cm)
	if err != nil {
		klog.Error(err, err.Error())
		os.Exit(1)
	}
}

func getClusterConfig(config rest.Config) {
	cc := &policyv1.ClusterConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: "configuration",
		},
		Spec: policyv1.ClusterConfigSpec{
			AdvertisementConfig: policyv1.AdvertisementConfig{
				AutoAccept:                 true,
				MaxAcceptableAdvertisement: 5,
				ResourceSharingPercentage:  30,
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
			},
		},
	}

	config.GroupVersion = &policyv1.GroupVersion
	client, err := v1alpha1.NewFromConfig(&config)
	if err != nil {
		ctrl.Log.Error(err, err.Error())
		os.Exit(1)
	}

	_, err = client.Resource("clusterconfigs").Create(cc, metav1.CreateOptions{})
	if err != nil {
		ctrl.Log.Error(err, err.Error())
		os.Exit(1)
	}
}
