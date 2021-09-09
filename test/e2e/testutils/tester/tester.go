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
	"os"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

	"github.com/liqotech/liqo/pkg/utils"
)

// Tester is used to encapsulate the context where the test is executed.
type Tester struct {
	Clusters  []ClusterContext
	Namespace string
}

// ClusterContext encapsulate all information and objects used to access a test cluster.
type ClusterContext struct {
	Config         *rest.Config
	Client         *kubernetes.Clientset
	ClusterID      string
	KubeconfigPath string
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
		klog.Error("KUBECONFIG_1 not set")
		os.Exit(1)
	}
	kubeconfig2 := os.Getenv("KUBECONFIG_2")
	if kubeconfig2 == "" {
		klog.Error("KUBECONFIG_2 not set")
		os.Exit(1)
	}
	namespace := os.Getenv("NAMESPACE")
	if namespace == "" {
		klog.Error("NAMESPACE not set")
		os.Exit(1)
	}

	config1, err := clientcmd.BuildConfigFromFlags("", kubeconfig1)
	if err != nil {
		klog.Error(err)
		os.Exit(1)
	}
	config2, err := clientcmd.BuildConfigFromFlags("", kubeconfig2)
	if err != nil {
		klog.Error(err)
		os.Exit(1)
	}
	clientset1, err := kubernetes.NewForConfig(config1)
	if err != nil {
		klog.Error(err)
		os.Exit(1)
	}
	clientset2, err := kubernetes.NewForConfig(config2)
	if err != nil {
		klog.Error(err)
		os.Exit(1)
	}
	clusterID1, err := utils.GetClusterIDWithNativeClient(ctx, clientset1, namespace)
	if err != nil {
		klog.Warningf("an error occurred while getting cluster-id configmap %s", err)
		clusterID1 = ""
	}
	clusterID2, err := utils.GetClusterIDWithNativeClient(ctx, clientset2, namespace)
	if err != nil {
		klog.Warningf("an error occurred while getting cluster-id configmap %s", err)
		clusterID2 = ""
	}
	return &Tester{
		Namespace: namespace,
		Clusters: []ClusterContext{
			{
				Config:         config1,
				KubeconfigPath: kubeconfig1,
				Client:         clientset1,
				ClusterID:      clusterID1,
			},
			{
				Config:         config2,
				KubeconfigPath: kubeconfig2,
				Client:         clientset2,
				ClusterID:      clusterID2,
			},
		},
	}
}
