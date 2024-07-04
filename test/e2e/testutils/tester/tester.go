// Copyright 2019-2024 The Liqo Authors
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

package tester

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	liqov1alpha1 "github.com/liqotech/liqo/apis/core/v1alpha1"
	ipamv1alpha1 "github.com/liqotech/liqo/apis/ipam/v1alpha1"
	offv1alpha1 "github.com/liqotech/liqo/apis/offloading/v1alpha1"
	virtualKubeletv1alpha1 "github.com/liqotech/liqo/apis/virtualkubelet/v1alpha1"
	"github.com/liqotech/liqo/pkg/utils"
	"github.com/liqotech/liqo/test/e2e/testconsts"
	testutils "github.com/liqotech/liqo/test/e2e/testutils/util"
)

// Tester is used to encapsulate the context where the test is executed.
type Tester struct {
	Clusters  []ClusterContext
	Namespace string
	// ClustersNumber represents the number of available clusters
	ClustersNumber   int
	OverlappingCIDRs bool
}

// ClusterContext encapsulate all information and objects used to access a test cluster.
type ClusterContext struct {
	Config             *rest.Config
	NativeClient       *kubernetes.Clientset
	ControllerClient   client.Client
	Cluster            liqov1alpha1.ClusterID
	KubeconfigPath     string
	HomeCluster        bool
	Role               liqov1alpha1.RoleType
	NumPeeredConsumers int
	NumPeeredProviders int
}

// Environment variable.
const (
	kubeconfigBaseName = "liqo_kubeconf"
)

var (
	tester *Tester
)

// GetTester returns a Tester instance.
func GetTester(ctx context.Context) *Tester {
	d, _ := os.Getwd()
	klog.Info(d)

	if tester == nil {
		var err error
		tester, err = createTester(ctx, false)
		if err != nil {
			klog.Fatalf("Failed to create e2e tester: %v", err)
		}
	}
	return tester
}

// GetTesterUninstall returns a Tester instance that do not interact with liqo resources.
func GetTesterUninstall(ctx context.Context) *Tester {
	d, _ := os.Getwd()
	klog.Info(d)

	if tester == nil {
		var err error
		tester, err = createTester(ctx, true)
		if err != nil {
			klog.Fatalf("Failed to create e2e tester: %v", err)
		}
	}
	return tester
}

func createTester(ctx context.Context, ignoreClusterIDError bool) (*Tester, error) {
	var err error
	namespace := testutils.GetEnvironmentVariableOrDie(testconsts.NamespaceEnvVar)
	TmpDir := testutils.GetEnvironmentVariableOrDie(testconsts.KubeconfigDirVarName)

	overlappingCIDRsString := testutils.GetEnvironmentVariableOrDie(testconsts.OverlappingCIDRsEnvVar)

	// Here is necessary to add the controller runtime clients.
	scheme := getScheme()

	tester = &Tester{
		Namespace:        namespace,
		OverlappingCIDRs: strings.EqualFold(overlappingCIDRsString, "true"),
	}

	tester.ClustersNumber, err = getClusterNumberFromEnv()
	if err != nil {
		return nil, err
	}

	for i := 1; i <= tester.ClustersNumber; i++ {
		var kubeconfigName = fmt.Sprintf("%s_%d", kubeconfigBaseName, i)
		var kubeconfigPath = filepath.Join(TmpDir, kubeconfigName)
		if _, err = os.Stat(kubeconfigPath); err != nil {
			return nil, err
		}
		var c = ClusterContext{
			Config:         testutils.GetRestConfigOrDie(kubeconfigPath),
			KubeconfigPath: kubeconfigPath,
			HomeCluster:    i == 1,
		}
		c.NativeClient = kubernetes.NewForConfigOrDie(c.Config)
		c.Cluster, err = utils.GetClusterIDWithNativeClient(ctx, c.NativeClient, namespace)
		if err != nil && !ignoreClusterIDError {
			return nil, err
		}

		controllerClient := testutils.GetControllerClient(scheme, c.Config)
		c.ControllerClient = controllerClient

		// The topology is one consumer (the first cluster) peered with multiple provider (the remaining clusters).
		// TODO: test also topology with multiple consumers peered with a single provider.
		if i == 1 {
			c.Role = liqov1alpha1.ConsumerRole
			c.NumPeeredConsumers = 0
			c.NumPeeredProviders = tester.ClustersNumber - 1
		} else {
			c.Role = liqov1alpha1.ProviderRole
			c.NumPeeredConsumers = 1
			c.NumPeeredProviders = 0
		}

		tester.Clusters = append(tester.Clusters, c)
	}

	return tester, nil
}

func getClusterNumberFromEnv() (int, error) {
	clusterNumberString := testutils.GetEnvironmentVariableOrDie(testconsts.ClusterNumberVarKey)

	clusterNumber, err := strconv.Atoi(clusterNumberString)
	if err != nil || clusterNumber < 0 {
		return 0, err
	}

	return clusterNumber, nil
}

func getScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = offv1alpha1.AddToScheme(scheme)
	_ = liqov1alpha1.AddToScheme(scheme)
	_ = ipamv1alpha1.AddToScheme(scheme)
	_ = virtualKubeletv1alpha1.AddToScheme(scheme)
	return scheme
}
