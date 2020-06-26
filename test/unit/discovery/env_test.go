package discovery

import (
	v1 "github.com/liqoTech/liqo/api/discovery/v1"
	foreign_cluster_operator "github.com/liqoTech/liqo/internal/discovery/foreign-cluster-operator"
	peering_request_operator "github.com/liqoTech/liqo/internal/peering-request-operator"
	"github.com/liqoTech/liqo/pkg/clusterID"
	discoveryv1 "github.com/liqoTech/liqo/pkg/discovery/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"os"
	"path/filepath"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"time"
)

type Cluster struct {
	env             *envtest.Environment
	cfg             *rest.Config
	k8sClient       *kubernetes.Clientset
	discoveryClient *discoveryv1.DiscoveryV1Client
	fcReconciler    *foreign_cluster_operator.ForeignClusterReconciler
	prReconciler    *peering_request_operator.PeeringRequestReconciler
	clusterId       *clusterID.ClusterID
}

func getClientCluster() *Cluster {
	cluster, mgr := getCluster()
	cluster.clusterId = clusterID.GetNewClusterID("client-cluster", cluster.k8sClient, ctrl.Log)
	cluster.fcReconciler = foreign_cluster_operator.GetFCReconciler(
		ctrl.Log.WithName("controllers").WithName("ForeignCluster"),
		mgr.GetScheme(),
		"default",
		cluster.k8sClient,
		cluster.discoveryClient,
		cluster.clusterId,
		1*time.Minute,
	)
	err := cluster.fcReconciler.SetupWithManager(mgr)
	if err != nil {
		ctrl.Log.Error(err, err.Error())
		os.Exit(1)
	}

	go func() {
		err = mgr.Start(stopChan)
		if err != nil {
			ctrl.Log.Error(err, err.Error())
			os.Exit(1)
		}
	}()
	return cluster
}

func getServerCluster() *Cluster {
	cluster, mgr := getCluster()
	cluster.clusterId = clusterID.GetNewClusterID("server-cluster", cluster.k8sClient, ctrl.Log)
	cluster.prReconciler = peering_request_operator.GetPRReconciler(
		ctrl.Log.WithName("controllers").WithName("PeeringRequest"),
		mgr.GetScheme(),
		cluster.k8sClient,
		cluster.discoveryClient,
		"default",
		cluster.clusterId,
		"liqo-config",
		"broadcaster",
		"br-sa",
	)
	err := cluster.prReconciler.SetupWithManager(mgr)
	if err != nil {
		ctrl.Log.Error(err, err.Error())
		os.Exit(1)
	}

	go func() {
		err = mgr.Start(stopChan)
		if err != nil {
			ctrl.Log.Error(err, err.Error())
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
		ctrl.Log.Error(err, err.Error())
		os.Exit(1)
	}

	err = v1.AddToScheme(scheme.Scheme)
	if err != nil {
		ctrl.Log.Error(err, err.Error())
		os.Exit(1)
	}

	cluster.k8sClient, err = kubernetes.NewForConfig(cluster.cfg)
	if err != nil {
		ctrl.Log.Error(err, err.Error())
		os.Exit(1)
	}
	cluster.discoveryClient, err = discoveryv1.NewForConfig(cluster.cfg)
	if err != nil {
		ctrl.Log.Error(err, err.Error())
		os.Exit(1)
	}
	k8sManager, err := ctrl.NewManager(cluster.cfg, ctrl.Options{
		Scheme:             scheme.Scheme,
		MetricsBindAddress: "0", // this avoids port binding collision
	})
	if err != nil {
		ctrl.Log.Error(err, err.Error())
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
	_, err = cluster.k8sClient.CoreV1().Secrets("default").Create(secret)
	if err != nil {
		ctrl.Log.Error(err, err.Error())
		os.Exit(1)
	}

	getLiqoConfig(cluster.k8sClient)

	return cluster, k8sManager
}

func getLiqoConfig(client *kubernetes.Clientset) {
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
		ctrl.Log.Error(err, err.Error())
		os.Exit(1)
	}
}
