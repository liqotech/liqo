// Copyright 2019-2021 The Liqo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package util

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	utils "github.com/liqotech/liqo/pkg/utils"
	"github.com/liqotech/liqo/test/e2e/testconsts"
)

// GetEnvironmentVariableOrDie retrieves the value of the environment variable named by the key.
// If the variable is not present calls klog.Fatal().
func GetEnvironmentVariableOrDie(key string) string {
	envVariable := os.Getenv(key)
	if envVariable == "" {
		klog.Fatalf("Environment variable '%s' not set", key)
	}
	return envVariable
}

// GetRestConfigOrDie retrieves the rest.Config from the kubeconfig variable.
// If there is an error calls klog.Fatal().
func GetRestConfigOrDie(kubeconfig string) *rest.Config {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		klog.Fatal(err)
	}
	return config
}

// GetControllerClient creates a new controller runtime client for the given config.
// If there is an error calls klog.Fatal().
func GetControllerClient(scheme *runtime.Scheme, config *rest.Config) client.Client {
	controllerClient, err := client.New(config, client.Options{
		Scheme: scheme,
	})
	if err != nil {
		klog.Fatal(err)
	}
	return controllerClient
}

// GetClusterID provides the clusterID for the cluster associated with the client.
func GetClusterID(ctx context.Context, cl kubernetes.Interface, namespace string) (string, error) {
	clusterID, err := utils.GetClusterIDWithNativeClient(ctx, cl, namespace)
	if err != nil {
		return "", fmt.Errorf("an error occurred while getting cluster-id configmap %w", err)
	}
	return clusterID, nil
}

// CheckIfTestIsSkipped checks if the number of clusters required by the test is less than
// the number of cluster really present.
func CheckIfTestIsSkipped(t *testing.T, clustersRequired int, testName string) {
	numberOfTestClusters, err := strconv.Atoi(GetEnvironmentVariableOrDie(testconsts.ClusterNumberVarKey))
	if err != nil {
		klog.Fatalf(" %s -> unable to covert the '%s' environment variable", err, testconsts.ClusterNumberVarKey)
	}
	if numberOfTestClusters < clustersRequired {
		t.Skipf("not enough cluster for the '%s'", testName)
	}
}
