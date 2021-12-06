// Copyright © 2017 The virtual-kubelet authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package root

import (
	"fmt"
	"os"
	"time"

	"github.com/virtual-kubelet/virtual-kubelet/node"
	corev1 "k8s.io/api/core/v1"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
	argsutils "github.com/liqotech/liqo/pkg/utils/args"
)

// Defaults for root command options.
const (
	DefaultNodeName             = "virtual-kubelet"
	DefaultInformerResyncPeriod = 10 * time.Hour
	DefaultListenPort           = 10250
	DefaultMetricsAddress       = ":10255"

	DefaultPodWorkers                  = 10
	DefaultServiceWorkers              = 3
	DefaultEndpointSliceWorkers        = 10
	DefaultConfigMapWorkers            = 3
	DefaultSecretWorkers               = 3
	DefaultPersistenVolumeClaimWorkers = 3

	DefaultNodePingTimeout = 1 * time.Second
)

// Opts stores all the options for configuring the root virtual-kubelet command.
// It is used for setting flag values.
type Opts struct {
	HomeKubeconfig string

	// Node name to use when creating a node in Kubernetes
	NodeName             string
	TenantNamespace      string
	InformerResyncPeriod time.Duration

	HomeCluster    discoveryv1alpha1.ClusterIdentity
	ForeignCluster discoveryv1alpha1.ClusterIdentity
	LiqoIpamServer string

	// Sets the port to listen for requests from the Kubernetes API server
	ListenPort      uint16
	MetricsAddress  string
	EnableProfiling bool

	// Number of workers to use to handle pod notifications and resource reflection
	PodWorkers                  uint
	ServiceWorkers              uint
	EndpointSliceWorkers        uint
	ConfigMapWorkers            uint
	SecretWorkers               uint
	PersistenVolumeClaimWorkers uint

	NodeLeaseDuration time.Duration
	NodePingInterval  time.Duration
	NodePingTimeout   time.Duration

	NodeExtraAnnotations argsutils.StringMap
	NodeExtraLabels      argsutils.StringMap

	EnableStorage              bool
	VirtualStorageClassName    string
	RemoteRealStorageClassName string
}

// NewOpts returns an Opts struct with the default values set.
func NewOpts() *Opts {
	return &Opts{
		HomeKubeconfig:       os.Getenv("KUBECONFIG"),
		NodeName:             DefaultNodeName,
		TenantNamespace:      corev1.NamespaceDefault,
		InformerResyncPeriod: DefaultInformerResyncPeriod,

		LiqoIpamServer: fmt.Sprintf("%v:%v", consts.NetworkManagerServiceName, consts.NetworkManagerIpamPort),

		ListenPort:      DefaultListenPort,
		MetricsAddress:  DefaultMetricsAddress,
		EnableProfiling: false,

		PodWorkers:                  DefaultPodWorkers,
		ServiceWorkers:              DefaultServiceWorkers,
		EndpointSliceWorkers:        DefaultEndpointSliceWorkers,
		ConfigMapWorkers:            DefaultConfigMapWorkers,
		SecretWorkers:               DefaultSecretWorkers,
		PersistenVolumeClaimWorkers: DefaultPersistenVolumeClaimWorkers,

		NodeLeaseDuration: node.DefaultLeaseDuration * time.Second,
		NodePingInterval:  node.DefaultPingInterval,
		NodePingTimeout:   DefaultNodePingTimeout,
	}
}
