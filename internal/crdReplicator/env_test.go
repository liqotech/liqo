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

package crdreplicator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
)

var (
	k8sManagerLocal ctrl.Manager
	testEnvLocal    *envtest.Environment
	k8sclient       kubernetes.Interface
	dynClient       dynamic.Interface
	dynFac          dynamicinformer.DynamicSharedInformerFactory
	localDynFac     dynamicinformer.DynamicSharedInformerFactory
	gvr             = schema.GroupVersionResource{
		Group:    netv1alpha1.GroupVersion.Group,
		Version:  netv1alpha1.GroupVersion.Version,
		Resource: "networkconfigs",
	}
	clusterID = "ClusterID-test"
)

func TestMain(m *testing.M) {
	setupEnv()
	defer tearDown()
	os.Exit(m.Run())
}

func setupEnv() {
	testEnvLocal = &envtest.Environment{
		CRDDirectoryPaths: []string{filepath.Join("..", "..", "deployments", "liqo", "crds")},
	}

	configLocal, err := testEnvLocal.Start()
	if err != nil {
		klog.Error(err, "an error occurred while setting up the local testing environment")
		os.Exit(-1)
	}

	k8sclient = kubernetes.NewForConfigOrDie(configLocal)

	k8sManagerLocal, err = ctrl.NewManager(configLocal, ctrl.Options{
		Scheme:             scheme.Scheme,
		MetricsBindAddress: "0",
	})
	if err != nil {
		klog.Error(err)
		panic(err)
	}
	dynClient = dynamic.NewForConfigOrDie(configLocal)
	dynFac = dynamicinformer.NewFilteredDynamicSharedInformerFactory(dynClient, ResyncPeriod, metav1.NamespaceAll, func(options *metav1.ListOptions) {
		//we want to watch only the resources that have been created by us on the remote cluster
		if options.LabelSelector == "" {
			newLabelSelector := []string{RemoteLabelSelector, "=", localClusterID}
			options.LabelSelector = strings.Join(newLabelSelector, "")
		} else {
			newLabelSelector := []string{options.LabelSelector, RemoteLabelSelector, "=", localClusterID}
			options.LabelSelector = strings.Join(newLabelSelector, "")
		}
	})

	localDynFac = dynamicinformer.NewFilteredDynamicSharedInformerFactory(dynClient, ResyncPeriod, metav1.NamespaceAll, nil)
	time.Sleep(1 * time.Second)
}

func tearDown() {
	err := testEnvLocal.Stop()
	if err != nil {
		klog.Error(err, err.Error())
		os.Exit(1)
	}
}
