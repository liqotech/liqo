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

package discovery

import (
	"context"
	"crypto/tls"
	"net/http"
	"sync"
	"time"

	"github.com/grandcat/zeroconf"
	"sigs.k8s.io/controller-runtime/pkg/client"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
)

// clusterRole
// +kubebuilder:rbac:groups=discovery.liqo.io,resources=foreignclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=discovery.liqo.io,resources=foreignclusters/status,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=nodes,verbs=get;list;watch
// role
// +kubebuilder:rbac:groups=core,namespace="liqo",resources=services,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,namespace="liqo",resources=configmaps,verbs=get;list;watch;create;update;delete

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

	LocalCluster discoveryv1alpha1.ClusterIdentity

	serverMux      sync.Mutex
	dialTCPTimeout time.Duration

	mdnsServerAuth *zeroconf.Server
	mdnsConfig     MDNSConfig

	insecureTransport *http.Transport
}

// NewDiscoveryCtrl returns a new discovery controller.
func NewDiscoveryCtrl(cl, namespacedClient client.Client, namespace string,
	localCluster discoveryv1alpha1.ClusterIdentity, config MDNSConfig, dialTCPTimeout time.Duration) *Controller {
	return &Controller{
		Client:           cl,
		namespacedClient: namespacedClient,
		namespace:        namespace,

		LocalCluster: localCluster,

		mdnsConfig:     config,
		dialTCPTimeout: dialTCPTimeout,

		insecureTransport: &http.Transport{IdleConnTimeout: 10 * time.Minute, TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
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
