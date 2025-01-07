// Copyright 2019-2025 The Liqo Authors
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
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	ipamv1alpha1 "github.com/liqotech/liqo/apis/ipam/v1alpha1"
	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
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
		CRDDirectoryPaths:     crdPath,
		ErrorIfCRDPathMissing: true,
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

	utilruntime.Must(liqov1beta1.AddToScheme(scheme.Scheme))
	utilruntime.Must(ipamv1alpha1.AddToScheme(scheme.Scheme))
	utilruntime.Must(offloadingv1beta1.AddToScheme(scheme.Scheme))
	utilruntime.Must(networkingv1beta1.AddToScheme(scheme.Scheme))

	mgr, err := ctrl.NewManager(cluster.cfg, ctrl.Options{
		Scheme:  scheme.Scheme,
		Metrics: server.Options{BindAddress: "0"}, // this avoids port binding collision
	})
	if err != nil {
		klog.Error(err)
		return Cluster{}, nil, err
	}

	return cluster, mgr, nil
}
