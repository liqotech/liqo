package tester

import (
	"context"

	capsulev1alpha1 "github.com/clastix/capsule/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	configv1alpha1 "github.com/liqotech/liqo/apis/config/v1alpha1"
	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	offv1alpha1 "github.com/liqotech/liqo/apis/offloading/v1alpha1"
	sharingv1alpha1 "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	virtualKubeletv1alpha1 "github.com/liqotech/liqo/apis/virtualKubelet/v1alpha1"
	testutils "github.com/liqotech/liqo/test/e2e/testutils/util"
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
	NativeClient     *kubernetes.Clientset
	ControllerClient client.Client
	ClusterID        string
	KubeconfigPath   string
}

// Environment variable.
const (
	kubeconfigCluster1EnvVar = "KUBECONFIG_1"
	kubeconfigCluster2EnvVar = "KUBECONFIG_2"
	kubeconfigCluster3EnvVar = "KUBECONFIG_3"
	kubeconfigCluster4EnvVar = "KUBECONFIG_4"
	namespaceEnvVar          = "NAMESPACE"
)

var (
	tester        *Tester
	kubeconfigVec = []string{kubeconfigCluster1EnvVar, kubeconfigCluster2EnvVar,
		kubeconfigCluster3EnvVar, kubeconfigCluster4EnvVar}
)

// GetTester returns a Tester instance.
func GetTester(ctx context.Context, clustersNumber int, controllerClientsPresence bool) *Tester {
	if tester == nil {
		tester = createTester(ctx, clustersNumber, controllerClientsPresence)
	}
	return tester
}

func createTester(ctx context.Context, clusterNumbers int, controllerClientsPresence bool) *Tester {
	var kubeconfigs []string
	var configs []*rest.Config
	var clientsets []*kubernetes.Clientset
	var clusterIDs []string
	namespace := testutils.GetEnvironmentVariable(namespaceEnvVar)

	tester = &Tester{
		Namespace: namespace,
	}

	for i := 0; i < clusterNumbers; i++ {
		kubeconfigs = append(kubeconfigs, testutils.GetEnvironmentVariable(kubeconfigVec[i]))
		configs = append(configs, testutils.GetRestConfig(kubeconfigs[i]))
		clientsets = append(clientsets, testutils.GetNativeClient(configs[i]))
		clusterIDs = append(clusterIDs, testutils.GetClusterID(ctx, clientsets[i], namespace))
		tester.Clusters = append(tester.Clusters, ClusterContext{
			Config:         configs[i],
			KubeconfigPath: kubeconfigs[i],
			NativeClient:   clientsets[i],
			ClusterID:      clusterIDs[i],
		})
	}

	if !controllerClientsPresence {
		return tester
	}

	// Here is necessary to add the controller runtime clients.
	scheme := getScheme()
	tester.ClustersClients = map[string]client.Client{}
	for i := 0; i < clusterNumbers; i++ {
		controllerClient := testutils.GetControllerClient(ctx, scheme, configs[i])
		tester.Clusters[i].ControllerClient = controllerClient
		tester.ClustersClients[clusterIDs[i]] = controllerClient
	}
	return tester
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
