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

package tester

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	capsulev1alpha1 "github.com/clastix/capsule/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
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
	// ClusterNumber represents the number of available clusters
	ClusterNumber int
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
	namespaceEnvVar      = "NAMESPACE"
	ClusterNumberVarKey  = "CLUSTER_NUMBER"
	kubeconfigBaseName   = "liqo_kubeconf_"
	KubeconfigDirVarName = "KUBECONFIGDIR"
)

var (
	tester *Tester
)

// GetTester returns a Tester instance.
func GetTester(ctx context.Context, controllerClientsPresence bool) *Tester {
	var err error
	if tester == nil {
		tester, err = createTester(ctx, controllerClientsPresence)
		if err != nil {
			klog.Fatal(err)
		}
	}
	return tester
}

func createTester(ctx context.Context, controllerClientsPresence bool) (*Tester, error) {
	namespace := testutils.GetEnvironmentVariable(namespaceEnvVar)
	TmpDir := testutils.GetEnvironmentVariable(KubeconfigDirVarName)

	// Here is necessary to add the controller runtime clients.
	scheme := getScheme()

	tester = &Tester{
		Namespace: namespace,
	}

	tester.ClustersClients = map[string]client.Client{}
	clusterNumber, err := getClusterNumberFromEnv()
	if err != nil {
		return nil, err
	}

	for i := 1; i <= clusterNumber; i++ {
		var kubeconfigName = strings.Join([]string{kubeconfigBaseName, fmt.Sprintf("%d",i)}, "")
		var kubeconfigPath = strings.Join([]string{TmpDir, kubeconfigName}, "")
		if _, err = os.Stat(kubeconfigPath); err != nil {
			return nil, err
		}
		var c = ClusterContext{
			Config:         testutils.GetRestConfig(kubeconfigPath),
			KubeconfigPath: kubeconfigPath,
		}
		c.NativeClient = testutils.GetNativeClient(c.Config)
		c.ClusterID = testutils.GetClusterID(ctx, c.NativeClient, namespace)

		if controllerClientsPresence {
			controllerClient := testutils.GetControllerClient(ctx, scheme, c.Config)
			c.ControllerClient = controllerClient
			tester.ClustersClients[c.ClusterID] = controllerClient
		}
		tester.Clusters = append(tester.Clusters, c)
	}

	return tester, nil
}

func getClusterNumberFromEnv() (int, error) {
	var clusterNumberString string
	var clusterNumber int
	var ok bool
	var err error
	if clusterNumberString, ok = os.LookupEnv(ClusterNumberVarKey); !ok {
		return 0, fmt.Errorf("%s Variable not found", ClusterNumberVarKey)
	}
	if clusterNumber, err = strconv.Atoi(clusterNumberString); err != nil || clusterNumber < 0 {
		return 0, err
	}
	return clusterNumber, nil
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
