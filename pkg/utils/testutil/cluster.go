// Copyright 2019-2022 The Liqo Authors
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

package testutil

import (
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	sharingv1alpha1 "github.com/liqotech/liqo/apis/sharing/v1alpha1"
)

type Cluster struct {
	env    *envtest.Environment
	cfg    *rest.Config
	client kubernetes.Interface
}

// GetEnv returns the test environment.
func (c *Cluster) GetEnv() *envtest.Environment {
	return c.env
}

// GetClient returns the crd client.
func (c *Cluster) GetClient() kubernetes.Interface {
	return c.client
}

func (c *Cluster) GetCfg() *rest.Config {
	return c.cfg
}

func NewTestCluster(crdPath []string) (Cluster, manager.Manager, error) {
	cluster := Cluster{}

	cluster.env = &envtest.Environment{
		CRDDirectoryPaths: crdPath,
	}

	/*
		Then, we start the envtest cluster.
	*/
	var err error
	cluster.cfg, err = cluster.env.Start()
	if err != nil {
		klog.Error(err)
		return Cluster{}, nil, err
	}

	cluster.client = kubernetes.NewForConfigOrDie(cluster.cfg)

	utilruntime.Must(discoveryv1alpha1.AddToScheme(scheme.Scheme))
	utilruntime.Must(sharingv1alpha1.AddToScheme(scheme.Scheme))
	utilruntime.Must(netv1alpha1.AddToScheme(scheme.Scheme))

	mgr, err := ctrl.NewManager(cluster.cfg, ctrl.Options{
		Scheme:             scheme.Scheme,
		MetricsBindAddress: "0", // this avoids port binding collision
	})
	if err != nil {
		klog.Error(err)
		return Cluster{}, nil, err
	}

	return cluster, mgr, nil
}
