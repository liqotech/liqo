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

package main

import (
	"context"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	"github.com/liqotech/liqo/pkg/clusterid"
	"github.com/liqotech/liqo/pkg/utils"
)

// cluster-role
// +kubebuilder:rbac:groups=core,resources=nodes,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;create

func main() {
	signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	kubeconfigPath, ok := os.LookupEnv("KUBECONFIG")
	if !ok {
		kubeconfigPath = filepath.Join(os.Getenv("HOME"), ".kube", "config")
	}

	namespace, ok := os.LookupEnv("POD_NAMESPACE")
	if !ok {
		klog.Fatal("The POD_NAMESPACE environment variable is not set")
	}

	klog.Infof("Loading client: %s", kubeconfigPath)
	config, err := utils.GetRestConfig(kubeconfigPath)
	if err != nil {
		klog.Errorf("Unable to create client config: %s", err)
		os.Exit(1)
	}

	client := kubernetes.NewForConfigOrDie(config)
	klog.Infof("Loaded client: %s", kubeconfigPath)

	localClusterID, err := clusterid.NewClusterIDFromClient(client)
	if err != nil {
		klog.Error(err)
		os.Exit(1)
	}
	err = localClusterID.SetupClusterID(namespace)
	if err != nil {
		klog.Error(err)
		os.Exit(1)
	}
}
