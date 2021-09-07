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

package discovery

import (
	"context"
	"sync"
	"time"

	"github.com/grandcat/zeroconf"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/liqotech/liqo/pkg/clusterid"
)

// MDNSConfig defines the configuration parameters for the mDNS service.
type MDNSConfig struct {
	EnableAdvertisement bool
	EnableDiscovery     bool

	Service string
	Domain  string
	TTL     time.Duration

	ResolveRefreshTime time.Duration
}

// Controller is the controller for the discovery functionalities.
type Controller struct {
	client.Client
	namespacedClient client.Client
	namespace        string

	LocalClusterID clusterid.ClusterID

	serverMux      sync.Mutex
	dialTCPTimeout time.Duration

	mdnsServerAuth *zeroconf.Server
	mdnsConfig     MDNSConfig
}

// NewDiscoveryCtrl returns a new discovery controller.
func NewDiscoveryCtrl(cl, namespacedClient client.Client, namespace string,
	localClusterID clusterid.ClusterID, config MDNSConfig, dialTCPTimeout time.Duration) *Controller {
	return &Controller{
		Client:           cl,
		namespacedClient: namespacedClient,
		namespace:        namespace,

		LocalClusterID: localClusterID,

		mdnsConfig:     config,
		dialTCPTimeout: dialTCPTimeout,
	}
}

// Start starts the discovery logic.
func (discovery *Controller) Start(ctx context.Context) error {
	if discovery.mdnsConfig.EnableAdvertisement {
		go discovery.register(ctx)
		go discovery.startGratuitousAnswers(ctx)
	}

	if discovery.mdnsConfig.EnableDiscovery {
		go discovery.startResolver(ctx)
	}

	go discovery.startGarbageCollector(ctx)

	<-ctx.Done()
	return nil
}
