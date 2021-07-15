package util

import (
	"context"
	"os"
	"strconv"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	testutils "github.com/liqotech/liqo/pkg/utils"
	cachedclientutils "github.com/liqotech/liqo/pkg/utils/cachedClient"
	"github.com/liqotech/liqo/test/e2e/testutils/tester"
)

// GetEnvironmentVariable retrieves the value of the environment variable named by the key.
// If the variable is not present calls klog.Fatal().
func GetEnvironmentVariable(key string) string {
	envVariable := os.Getenv(key)
	if envVariable == "" {
		klog.Fatalf("Environment variable '%s' not set", key)
	}
	return envVariable
}

// GetRestConfig retrieves the rest.Config from the kubeconfig variable.
// If there is an error calls klog.Fatal().
func GetRestConfig(kubeconfig string) *rest.Config {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		klog.Fatal(err)
	}
	return config
}

// GetNativeClient creates a new Clientset for the given config.
// If there is an error calls klog.Fatal().
func GetNativeClient(config *rest.Config) *kubernetes.Clientset {
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Fatal(err)
	}
	return clientset
}

// GetControllerClient creates a new controller runtime client for the given config.
// If there is an error calls klog.Fatal().
func GetControllerClient(ctx context.Context, scheme *runtime.Scheme, config *rest.Config) client.Client {
	controllerClient, err := cachedclientutils.GetCachedClientWithConfig(ctx, scheme, config)
	if err != nil {
		klog.Fatal(err)
	}
	return controllerClient
}

// GetClusterID provides the clusterID for the cluster associated with the client.
// If there is an error returns an empty clusterID.
func GetClusterID(ctx context.Context, cl kubernetes.Interface, namespace string) string {
	clusterID, err := testutils.GetClusterIDWithNativeClient(ctx, cl, namespace)
	if err != nil {
		klog.Warningf("an error occurred while getting cluster-id configmap %s", err)
		clusterID = ""
	}
	return clusterID
}

// CheckIfTestIsSkipped checks if the number of clusters required by the test is less than
// the number of cluster really present.
func CheckIfTestIsSkipped(t *testing.T, clustersRequired int, testName string) {
	numberOfTestClusters, err := strconv.Atoi(GetEnvironmentVariable(tester.ClusterNumberVarKey))
	if err != nil {
		klog.Fatalf(" %s -> unable to covert the '%s' environment variable", err, tester.ClusterNumberVarKey)
	}
	if numberOfTestClusters < clustersRequired {
		t.Skipf("not enough cluster for the '%s'", testName)
	}
}
