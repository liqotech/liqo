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

package testutil

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/liqotech/liqo/apis/discovery/v1alpha1"
	nettypes "github.com/liqotech/liqo/apis/net/v1alpha1"
	advtypes "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	crdclient "github.com/liqotech/liqo/pkg/crdClient"
)

type Cluster struct {
	env       *envtest.Environment
	cfg       *rest.Config
	client    *crdclient.CRDClient
	netClient *crdclient.CRDClient
}

// GetEnv returns the test environment.
func (c *Cluster) GetEnv() *envtest.Environment {
	return c.env
}

// GetClient returns the crd client.
func (c *Cluster) GetClient() *crdclient.CRDClient {
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

	cluster.cfg.ContentConfig.GroupVersion = &v1alpha1.GroupVersion
	cluster.cfg.APIPath = "/apis"
	cluster.cfg.NegotiatedSerializer = scheme.Codecs.WithoutConversion()
	cluster.cfg.UserAgent = rest.DefaultKubernetesUserAgent()

	netCfg := *cluster.cfg
	netCfg.ContentConfig.GroupVersion = &nettypes.GroupVersion
	crdclient.AddToRegistry("networkconfigs", &nettypes.NetworkConfig{}, &nettypes.NetworkConfigList{}, nil, nettypes.TunnelEndpointGroupResource)
	crdclient.AddToRegistry("tunnelendpoints", &nettypes.TunnelEndpoint{}, &nettypes.TunnelEndpointList{}, nil, nettypes.TunnelEndpointGroupResource)

	err = v1alpha1.AddToScheme(scheme.Scheme)
	if err != nil {
		klog.Error(err)
		return Cluster{}, nil, err
	}
	err = advtypes.AddToScheme(scheme.Scheme)
	if err != nil {
		klog.Error(err)
		return Cluster{}, nil, err
	}
	err = nettypes.AddToScheme(scheme.Scheme)
	if err != nil {
		klog.Error(err)
		return Cluster{}, nil, err
	}

	cluster.client, err = crdclient.NewFromConfig(cluster.cfg)
	if err != nil {
		klog.Error(err)
		return Cluster{}, nil, err
	}
	cluster.netClient, err = crdclient.NewFromConfig(&netCfg)
	if err != nil {
		klog.Error(err)
		return Cluster{}, nil, err
	}
	k8sManager, err := ctrl.NewManager(cluster.cfg, ctrl.Options{
		Scheme:             scheme.Scheme,
		MetricsBindAddress: "0", // this avoids port binding collision
	})
	if err != nil {
		klog.Error(err)
		return Cluster{}, nil, err
	}

	// creates empty CaData secret
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "ca-data",
		},
		Data: map[string][]byte{
			"ca.crt": []byte(""),
		},
	}
	_, err = cluster.client.Client().CoreV1().Secrets("default").Create(context.TODO(), secret, metav1.CreateOptions{})
	if err != nil {
		klog.Error(err)
		return Cluster{}, nil, err
	}

	return cluster, k8sManager, nil
}
