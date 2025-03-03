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

package client

import (
	"context"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/liqotech/liqo/pkg/liqoctl/test/network/flags"
	"github.com/liqotech/liqo/pkg/utils"
)

// Client is a struct that contains all the k8s clients used in tests.
type Client struct {
	Consumer        client.Client
	ConsumerName    string
	ConsumerDynamic *dynamic.DynamicClient
	// Providers key is the name of the provider kubeconfig file.
	Providers map[string]client.Client
	// ProvidersDynamic key is the name of the provider kubeconfig file.
	ProvidersDynamic map[string]*dynamic.DynamicClient
}

// Configs is a map that contains the rest configuration for each client.
type Configs map[string]*rest.Config

// NewClient returns a new Client struct.
func NewClient(ctx context.Context, opts *flags.Options) (*Client, Configs, error) {
	var cl Client
	cfg := Configs{}

	if err := initConfigAndClient(ctx, "", &cl, cfg, opts); err != nil {
		return nil, nil, err
	}

	for _, kubeconfig := range opts.RemoteKubeconfigs {
		if err := initConfigAndClient(ctx, kubeconfig, &cl, cfg, opts); err != nil {
			return nil, nil, err
		}
	}

	return &cl, cfg, nil
}

func initConfigAndClient(ctx context.Context, kubeconfig string, cl *Client, cfg Configs, opts *flags.Options) error {
	var err error
	var cfgtmp *rest.Config
	var cltmp client.Client
	var cldyntmp *dynamic.DynamicClient
	if kubeconfig == "" {
		cfgtmp = opts.Topts.LocalFactory.RESTConfig
		cltmp = opts.Topts.LocalFactory.CRClient
		cldyntmp, err = dynamic.NewForConfig(cfgtmp)
		if err != nil {
			return err
		}
	} else {
		cfgtmp, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return err
		}
		cltmp, err = client.New(cfgtmp, client.Options{
			Scheme: opts.Topts.LocalFactory.CRClient.Scheme(),
		})
		if err != nil {
			return err
		}
		cldyntmp, err = dynamic.NewForConfig(cfgtmp)
		if err != nil {
			return err
		}
	}

	name, err := utils.GetClusterIDWithControllerClient(ctx, cltmp, opts.Topts.LocalFactory.LiqoNamespace)
	if err != nil {
		return err
	}

	sname := string(name)

	cfg[sname] = cfgtmp
	if cl.ConsumerName == "" {
		cl.ConsumerName = sname
		cl.Consumer = cltmp
		cl.ConsumerDynamic = cldyntmp
	} else {
		if cl.Providers == nil {
			cl.Providers = make(map[string]client.Client)
		}
		cl.Providers[sname] = cltmp

		if cl.ProvidersDynamic == nil {
			cl.ProvidersDynamic = make(map[string]*dynamic.DynamicClient)
		}
		cl.ProvidersDynamic[sname] = cldyntmp
	}

	return err
}
