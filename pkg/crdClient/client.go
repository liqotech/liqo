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

package crdclient

import (
	"os"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	clientsetFake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	restFake "k8s.io/client-go/rest/fake"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/liqotech/liqo/pkg/consts"
)

var Fake bool

type NamespacedCRDClientInterface interface {
	Resource(resource string) CrdClientInterface
}

type CRDClient struct {
	crdClient rest.Interface
	client    kubernetes.Interface

	config *rest.Config
	Store  cache.Store
	Stop   chan struct{}
}

func NewKubeconfig(configPath string, gv *schema.GroupVersion, configOptions func(config *rest.Config)) (*rest.Config, error) {
	config := &rest.Config{}

	if !Fake {
		// Check if the kubeConfig file exists.
		if _, err := os.Stat(configPath); !os.IsNotExist(err) {
			// Get the kubeconfig from the filepath.
			config, err = clientcmd.BuildConfigFromFlags("", configPath)
			if err != nil {
				return nil, errors.Wrap(err, "error building Client config")
			}
		} else {
			// Set to in-cluster config.
			config, err = rest.InClusterConfig()
			if err != nil {
				return nil, errors.Wrap(err, "error building in cluster config")
			}
		}
	} else {
		config.ContentConfig = rest.ContentConfig{ContentType: "application/json"}
	}

	config.ContentConfig.GroupVersion = gv
	config.APIPath = consts.ApisPath
	config.NegotiatedSerializer = scheme.Codecs.WithoutConversion()
	config.UserAgent = rest.DefaultKubernetesUserAgent()

	if configOptions != nil {
		configOptions(config)
	}

	return config, nil
}

func NewFromConfig(config *rest.Config) (*CRDClient, error) {
	if Fake {
		return newFakeFromConfig(config)
	}

	return newRealFromconfig(config)
}

func newRealFromconfig(config *rest.Config) (*CRDClient, error) {
	crdClient, err := rest.RESTClientFor(config)
	if err != nil {
		return nil, err
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &CRDClient{crdClient: crdClient,
		client: client,
		config: config}, nil
}

func newFakeFromConfig(config *rest.Config) (*CRDClient, error) {
	var gv schema.GroupVersion
	if config == nil {
		gv = schema.GroupVersion{}
	} else {
		gv = *config.GroupVersion
	}

	c := &CRDClient{
		crdClient: &restFake.RESTClient{NegotiatedSerializer: scheme.Codecs.WithoutConversion(),
			GroupVersion: gv},
		client: clientsetFake.NewSimpleClientset(),
		config: config}

	return c, nil
}

func (c *CRDClient) Resource(api string) CrdClientInterface {
	if Fake {
		return &FakeClient{
			Client:   c.crdClient,
			api:      api,
			resource: Registry[api],
			storage:  c.Store.(*fakeInformer),
		}
	} else {
		return &Client{
			Client:   c.crdClient,
			api:      api,
			resource: Registry[api],
			storage:  c.Store,
		}
	}
}

func (c *CRDClient) Client() kubernetes.Interface {
	if Fake {
		return c.client.(*clientsetFake.Clientset)
	}

	return c.client.(*kubernetes.Clientset)
}

func (c *CRDClient) Config() *rest.Config {
	return c.config
}
