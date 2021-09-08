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

package crdReplicator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	crdreplicator "github.com/liqotech/liqo/internal/crdReplicator"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/utils/restcfg"
)

var (
	numberPeeringClusters = 1

	peeringIDTemplate                = "peering-cluster-"
	localClusterID                   = "localClusterID"
	peeringClustersTestEnvs          = map[string]*envtest.Environment{}
	peeringClustersManagers          = map[string]ctrl.Manager{}
	peeringClustersDynClients        = map[string]dynamic.Interface{}
	k8sManagerLocal                  ctrl.Manager
	testEnvLocal                     *envtest.Environment
	dOperator                        *crdreplicator.Controller
	clusterIDToRemoteNamespaceMapper = map[string]string{}
)

func TestMain(m *testing.M) {
	setupEnv()
	defer tearDown()
	startDispatcherOperator()
	time.Sleep(10 * time.Second)
	klog.Info("main set up")
	os.Exit(m.Run())
}

func startDispatcherOperator() {
	err := setupDispatcherOperator()
	if err != nil {
		klog.Error(err)
		os.Exit(-1)
	}
	ctx := context.TODO()
	go func() {
		if err = k8sManagerLocal.Start(ctrl.SetupSignalHandler()); err != nil {
			klog.Error(err)
			panic(err)
		}
	}()
	started := k8sManagerLocal.GetCache().WaitForCacheSync(ctx)
	if !started {
		klog.Errorf("an error occurred while waiting for the chache to start")
		os.Exit(-1)
	}
	configLocal := k8sManagerLocal.GetConfig()
	// gotta go fast during tests -- we don't really care about overwhelming our test API server
	restcfg.SetRateLimiterWithCustomParamenters(configLocal, 1000, 2000)
	fc := getForeignClusterResource()
	_, err = dOperator.LocalDynClient.Resource(fcGVR).Create(context.TODO(), fc, metav1.CreateOptions{})
	if err != nil {
		klog.Error(err, err.Error())
		os.Exit(-1)
	}

	if err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		tmp, err := dOperator.LocalDynClient.Resource(fcGVR).Get(context.TODO(), fc.GetName(), metav1.GetOptions{})
		if err != nil {
			klog.Error(err, err.Error())
			return err
		}

		fc.SetResourceVersion(tmp.GetResourceVersion())
		_, err = dOperator.LocalDynClient.Resource(fcGVR).UpdateStatus(context.TODO(), fc, metav1.UpdateOptions{})
		if err != nil {
			klog.Error(err, err.Error())
			return err
		}

		return nil
	}); err != nil {
		klog.Error(err, err.Error())
		os.Exit(-1)
	}
}

func setupEnv() {
	err := discoveryv1alpha1.AddToScheme(scheme.Scheme)
	if err != nil {
		klog.Error(err)
	}
	//save the environment variables in the map
	for i := 1; i <= numberPeeringClusters; i++ {
		peeringClusterID := peeringIDTemplate + fmt.Sprintf("%d", i)
		peeringClustersTestEnvs[peeringClusterID] = &envtest.Environment{
			CRDDirectoryPaths: []string{filepath.Join("..", "..", "..", "deployments", "liqo", "crds")},
		}
	}
	//start the peering environments, save the managers, create dynamic clients
	for peeringClusterID, testEnv := range peeringClustersTestEnvs {
		config, err := testEnv.Start()
		if err != nil {
			klog.Errorf("%s -> an error occurred while setting test environment: %s", peeringClusterID, err)
			os.Exit(-1)
		} else {
			klog.Infof("%s -> created test environment", peeringClusterID)
		}
		manager, err := ctrl.NewManager(config, ctrl.Options{
			Scheme:             scheme.Scheme,
			MetricsBindAddress: "0",
		})
		if err != nil {
			klog.Errorf("%s -> an error occurred while creating the manager %s", peeringClusterID, err)
			os.Exit(-1)
		}
		peeringClustersManagers[peeringClusterID] = manager
		dynClient := dynamic.NewForConfigOrDie(manager.GetConfig())
		peeringClustersDynClients[peeringClusterID] = dynClient
		clusterIDToRemoteNamespaceMapper[peeringClusterID] = testNamespace
	}
	//setup the local testing environment
	testEnvLocal = &envtest.Environment{
		CRDDirectoryPaths: []string{filepath.Join("..", "..", "..", "deployments", "liqo", "crds")},
	}
	configLocal, err := testEnvLocal.Start()
	if err != nil {
		klog.Error(err, "an error occurred while setting up the local testing environment")
	}
	klog.Infof("%s -> created test environmen", localClusterID)
	restcfg.SetRateLimiterWithCustomParamenters(configLocal, 1000, 2000)
	k8sManagerLocal, err = ctrl.NewManager(configLocal, ctrl.Options{
		Scheme:             scheme.Scheme,
		MetricsBindAddress: "0",
	})
	if err != nil {
		klog.Errorf("%s -> an error occurred while creating the manager %s", localClusterID, err)
		os.Exit(-1)
	}
	klog.Info("setup of testing environments finished")
}

func tearDown() {
	//stop the peering testing environments
	for id, env := range peeringClustersTestEnvs {
		err := env.Stop()
		if err != nil {
			klog.Errorf("%s -> an error occurred while stopping peering environment test: %s", id, err)
		}
	}
	err := testEnvLocal.Stop()
	if err != nil {
		klog.Errorf("%s -> an error occurred while stopping local environment test: %s", localClusterID, err)
	}
}

func updateOwnership(ownership consts.OwnershipType) {
	for i := range dOperator.RegisteredResources {
		dOperator.RegisteredResources[i].Ownership = ownership
	}
}
