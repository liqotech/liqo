package tester

import (
	"context"
	"os"

	capsulev1alpha1 "github.com/clastix/capsule/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	configv1alpha1 "github.com/liqotech/liqo/apis/config/v1alpha1"
	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	offv1alpha1 "github.com/liqotech/liqo/apis/offloading/v1alpha1"
	sharingv1alpha1 "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	virtualKubeletv1alpha1 "github.com/liqotech/liqo/apis/virtualKubelet/v1alpha1"
	testutils "github.com/liqotech/liqo/pkg/utils"
	cachedclientutils "github.com/liqotech/liqo/pkg/utils/cachedClient"
)

// Tester is used to encapsulate the context where the test is executed.
type Tester struct {
	Clusters  []ClusterContext
	Namespace string
	// the key is the clusterID and the value is the corresponding client
	ClustersClients map[string]client.Client
}

// ClusterContext encapsulate all information and objects used to access a test cluster.
type ClusterContext struct {
	Config           *rest.Config
	Client           *kubernetes.Clientset
	ControllerClient client.Client
	ClusterID        string
	KubeconfigPath   string
}

var (
	tester *Tester
)

// GetTester returns a Tester instance.
func GetTester(ctx context.Context) *Tester {
	if tester == nil {
		tester = createTester(ctx)
	}

	return tester
}

func createTester(ctx context.Context) *Tester {
	kubeconfig1 := os.Getenv("KUBECONFIG_1")
	if kubeconfig1 == "" {
		klog.Fatal("KUBECONFIG_1 not set")
	}
	kubeconfig2 := os.Getenv("KUBECONFIG_2")
	if kubeconfig2 == "" {
		klog.Fatal("KUBECONFIG_2 not set")
	}
	kubeconfig3 := os.Getenv("KUBECONFIG_3")
	if kubeconfig3 == "" {
		klog.Fatal("KUBECONFIG_3 not set")
	}
	kubeconfig4 := os.Getenv("KUBECONFIG_4")
	if kubeconfig4 == "" {
		klog.Fatal("KUBECONFIG_4 not set")
	}

	namespace := os.Getenv("NAMESPACE")
	if namespace == "" {
		klog.Fatal("NAMESPACE not set")
	}

	config1, err := clientcmd.BuildConfigFromFlags("", kubeconfig1)
	if err != nil {
		klog.Fatal(err)
	}
	config2, err := clientcmd.BuildConfigFromFlags("", kubeconfig2)
	if err != nil {
		klog.Fatal(err)
	}
	config3, err := clientcmd.BuildConfigFromFlags("", kubeconfig3)
	if err != nil {
		klog.Fatal(err)
	}
	config4, err := clientcmd.BuildConfigFromFlags("", kubeconfig4)
	if err != nil {
		klog.Fatal(err)
	}

	clientset1, err := kubernetes.NewForConfig(config1)
	if err != nil {
		klog.Fatal(err)
	}
	clientset2, err := kubernetes.NewForConfig(config2)
	if err != nil {
		klog.Fatal(err)
	}
	clientset3, err := kubernetes.NewForConfig(config3)
	if err != nil {
		klog.Fatal(err)
	}
	clientset4, err := kubernetes.NewForConfig(config4)
	if err != nil {
		klog.Fatal(err)
	}

	scheme := getScheme()
	controllerClient1, err := cachedclientutils.GetCachedClientWithConfig(ctx, scheme, config1)
	if err != nil {
		klog.Fatal(err)
	}
	controllerClient2, err := cachedclientutils.GetCachedClientWithConfig(ctx, scheme, config2)
	if err != nil {
		klog.Fatal(err)
	}
	controllerClient3, err := cachedclientutils.GetCachedClientWithConfig(ctx, scheme, config3)
	if err != nil {
		klog.Fatal(err)
	}
	controllerClient4, err := cachedclientutils.GetCachedClientWithConfig(ctx, scheme, config4)
	if err != nil {
		klog.Fatal(err)
	}

	clusterID1, err := testutils.GetClusterIDWithNativeClient(ctx, clientset1, namespace)
	if err != nil {
		klog.Warningf("an error occurred while getting cluster-id configmap %s", err)
		clusterID1 = ""
	}
	clusterID2, err := testutils.GetClusterIDWithNativeClient(ctx, clientset2, namespace)
	if err != nil {
		klog.Warningf("an error occurred while getting cluster-id configmap %s", err)
		clusterID2 = ""
	}
	clusterID3, err := testutils.GetClusterIDWithNativeClient(ctx, clientset3, namespace)
	if err != nil {
		klog.Warningf("an error occurred while getting cluster-id configmap %s", err)
		clusterID2 = ""
	}
	clusterID4, err := testutils.GetClusterIDWithNativeClient(ctx, clientset4, namespace)
	if err != nil {
		klog.Warningf("an error occurred while getting cluster-id configmap %s", err)
		clusterID2 = ""
	}

	return &Tester{
		Namespace: namespace,
		Clusters: []ClusterContext{
			{
				Config:           config1,
				KubeconfigPath:   kubeconfig1,
				Client:           clientset1,
				ClusterID:        clusterID1,
				ControllerClient: controllerClient1,
			},
			{
				Config:           config2,
				KubeconfigPath:   kubeconfig2,
				Client:           clientset2,
				ClusterID:        clusterID2,
				ControllerClient: controllerClient2,
			},
			{
				Config:           config3,
				KubeconfigPath:   kubeconfig3,
				Client:           clientset3,
				ClusterID:        clusterID3,
				ControllerClient: controllerClient3,
			},
			{
				Config:           config4,
				KubeconfigPath:   kubeconfig4,
				Client:           clientset4,
				ClusterID:        clusterID4,
				ControllerClient: controllerClient4,
			},
		},
		ClustersClients: map[string]client.Client{
			clusterID1: controllerClient1,
			clusterID2: controllerClient2,
			clusterID3: controllerClient3,
			clusterID4: controllerClient4,
		},
	}
}

func getScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	_ = offv1alpha1.AddToScheme(scheme)
	_ = configv1alpha1.AddToScheme(scheme)
	_ = discoveryv1alpha1.AddToScheme(scheme)
	_ = netv1alpha1.AddToScheme(scheme)
	_ = sharingv1alpha1.AddToScheme(scheme)
	_ = virtualKubeletv1alpha1.AddToScheme(scheme)
	_ = capsulev1alpha1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)
	_ = appsv1.AddToScheme(scheme)
	_ = rbacv1.AddToScheme(scheme)
	return scheme
}
