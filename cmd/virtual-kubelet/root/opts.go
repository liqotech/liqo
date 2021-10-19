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
	"strconv"
	"time"

	"github.com/pkg/errors"

	"github.com/liqotech/liqo/pkg/consts"
	argsutils "github.com/liqotech/liqo/pkg/utils/args"
)

// Defaults for root command options.
const (
	DefaultNodeName                               = "virtual-kubelet"
	DefaultInformerResyncPeriod                   = 1 * time.Minute
	DefaultLiqoInformerResyncPeriod time.Duration = 0
	DefaultMetricsAddr                            = ":10255"
	DefaultListenPort                             = 10250

	DefaultPodWorkers                  = 10
	DefaultServiceWorkers              = 3
	DefaultEndpointSliceWorkers        = 10
	DefaultConfigMapWorkers            = 3
	DefaultSecretWorkers               = 3
	DefaultPersistenVolumeClaimWorkers = 3

	DefaultKubeletNamespace = "default"
	DefaultLiqoIpamServer   = consts.NetworkManagerServiceName
)

// Opts stores all the options for configuring the root virtual-kubelet command.
// It is used for setting flag values.
//
// You can set the default options by creating a new `Opts` struct and passing
// it into `SetDefaultOpts`.
type Opts struct {
	// Sets the port to listen for requests from the Kubernetes API server
	ListenPort int32

	// Node name to use when creating a node in Kubernetes
	NodeName string

	HomeKubeconfig string

	MetricsAddr string

	// Number of workers to use to handle pod notifications and resource reflection
	PodWorkers                  uint
	ServiceWorkers              uint
	EndpointSliceWorkers        uint
	ConfigMapWorkers            uint
	SecretWorkers               uint
	PersistenVolumeClaimWorkers uint

	InformerResyncPeriod     time.Duration
	LiqoInformerResyncPeriod time.Duration

	// Startup Timeout is how long to wait for the kubelet to start
	StartupTimeout time.Duration

	ForeignClusterID string
	HomeClusterID    string
	KubeletNamespace string

	LiqoIpamServer string

	Profiling bool

	NodeExtraAnnotations argsutils.StringMap
	NodeExtraLabels      argsutils.StringMap

	EnableStorage              bool
	VirtualStorageClassName    string
	RemoteRealStorageClassName string
}

// SetDefaultOpts sets default options for unset values on the passed in option struct.
// Fields tht are already set will not be modified.
func SetDefaultOpts(c *Opts) error {
	if c.InformerResyncPeriod == 0 {
		c.InformerResyncPeriod = DefaultInformerResyncPeriod
	}

	if c.LiqoInformerResyncPeriod == 0 {
		c.InformerResyncPeriod = DefaultLiqoInformerResyncPeriod
	}

	if c.MetricsAddr == "" {
		c.MetricsAddr = DefaultMetricsAddr
	}

	if c.PodWorkers == 0 {
		c.PodWorkers = DefaultPodWorkers
	}

	if c.ServiceWorkers == 0 {
		c.ServiceWorkers = DefaultServiceWorkers
	}

	if c.EndpointSliceWorkers == 0 {
		c.EndpointSliceWorkers = DefaultEndpointSliceWorkers
	}

	if c.ConfigMapWorkers == 0 {
		c.ConfigMapWorkers = DefaultConfigMapWorkers
	}

	if c.SecretWorkers == 0 {
		c.SecretWorkers = DefaultSecretWorkers
	}

	if c.PersistenVolumeClaimWorkers == 0 {
		c.PersistenVolumeClaimWorkers = DefaultPersistenVolumeClaimWorkers
	}

	if c.ListenPort == 0 {
		if kp := os.Getenv("KUBELET_PORT"); kp != "" {
			p, err := strconv.ParseInt(kp, 10, 32)
			if err != nil {
				return errors.Wrap(err, "error parsing KUBELET_PORT environment variable")
			}
			c.ListenPort = int32(p)
		} else {
			c.ListenPort = DefaultListenPort
		}
	}

	if c.KubeletNamespace == "" {
		c.KubeletNamespace = DefaultKubeletNamespace
	}
	if c.HomeKubeconfig == "" {
		c.HomeKubeconfig = os.Getenv("KUBECONFIG")
	}
	if c.LiqoIpamServer == "" {
		c.LiqoIpamServer = fmt.Sprintf("%v:%v", consts.NetworkManagerServiceName, consts.NetworkManagerIpamPort)
	}

	return nil
}
