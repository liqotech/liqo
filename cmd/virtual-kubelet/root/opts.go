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
	"os"
	"time"

	"github.com/virtual-kubelet/virtual-kubelet/node"
	corev1 "k8s.io/api/core/v1"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	argsutils "github.com/liqotech/liqo/pkg/utils/args"
)

const (
	// CertificateTypeKubelet -> the kubelet certificate is requested to be signed by kubernetes.io/kubelet-serving.
	CertificateTypeKubelet = "kubelet"
	// CertificateTypeAWS -> the kubelet certificate is requested to be signed by beta.eks.amazonaws.com/app-serving.
	CertificateTypeAWS = "aws"
	// CertificateTypeSelfSigned -> the kubelet certificate is self signed.
	CertificateTypeSelfSigned = "self-signed"
)

// Defaults for root command options.
const (
	DefaultNodeName             = "virtual-kubelet"
	DefaultInformerResyncPeriod = 10 * time.Hour
	DefaultListenPort           = 10250

	DefaultPodWorkers                  = 10
	DefaultServiceWorkers              = 3
	DefaultEndpointSliceWorkers        = 10
	DefaultIngressWorkers              = 3
	DefaultConfigMapWorkers            = 3
	DefaultSecretWorkers               = 3
	DefaultServiceAccountWorkers       = 3
	DefaultPersistenVolumeClaimWorkers = 3
	DefaultEventWorkers                = 3

	DefaultNodePingTimeout  = 1 * time.Second
	DefaultNodeCheckNetwork = true
)

// Opts stores all the options for configuring the root virtual-kubelet command.
// It is used for setting flag values.
type Opts struct {
	HomeKubeconfig             string
	RemoteKubeconfigSecretName string

	// Node name to use when creating a node in Kubernetes
	NodeName string
	// PodName to use when holding the virtual-kubelet lease
	PodName              string
	TenantNamespace      string
	InformerResyncPeriod time.Duration

	HomeCluster         discoveryv1alpha1.ClusterIdentity
	ForeignCluster      discoveryv1alpha1.ClusterIdentity
	LiqoIpamServer      string
	DisableIPReflection bool

	// Sets the addresses to listen for requests from the Kubernetes API server
	NodeIP          string
	ListenPort      uint16
	CertificateType *argsutils.StringEnum
	EnableProfiling bool

	// Number of workers to use to handle pod notifications and resource reflection
	PodWorkers                   uint
	ServiceWorkers               uint
	EndpointSliceWorkers         uint
	IngressWorkers               uint
	ConfigMapWorkers             uint
	SecretWorkers                uint
	ServiceAccountWorkers        uint
	PersistentVolumeClaimWorkers uint
	EventWorkers                 uint

	NodeLeaseDuration time.Duration
	NodePingInterval  time.Duration
	NodePingTimeout   time.Duration
	NodeCheckNetwork  bool

	NodeExtraAnnotations argsutils.StringMap
	NodeExtraLabels      argsutils.StringMap

	EnableAPIServerSupport     bool
	EnableStorage              bool
	VirtualStorageClassName    string
	RemoteRealStorageClassName string
	EnableMetrics              bool
	MetricsAddress             string

	HomeAPIServerHost string
	HomeAPIServerPort string

	CreateNode bool
}

// NewOpts returns an Opts struct with the default values set.
func NewOpts() *Opts {
	return &Opts{
		HomeKubeconfig:       os.Getenv("KUBECONFIG"),
		NodeName:             DefaultNodeName,
		PodName:              os.Getenv("POD_NAME"),
		TenantNamespace:      corev1.NamespaceDefault,
		InformerResyncPeriod: DefaultInformerResyncPeriod,

		DisableIPReflection: false,

		CertificateType: argsutils.NewEnum([]string{CertificateTypeKubelet, CertificateTypeAWS, CertificateTypeSelfSigned}, CertificateTypeKubelet),
		ListenPort:      DefaultListenPort,
		EnableProfiling: false,

		PodWorkers:                   DefaultPodWorkers,
		ServiceWorkers:               DefaultServiceWorkers,
		EndpointSliceWorkers:         DefaultEndpointSliceWorkers,
		IngressWorkers:               DefaultIngressWorkers,
		ConfigMapWorkers:             DefaultConfigMapWorkers,
		SecretWorkers:                DefaultSecretWorkers,
		ServiceAccountWorkers:        DefaultServiceAccountWorkers,
		PersistentVolumeClaimWorkers: DefaultPersistenVolumeClaimWorkers,
		EventWorkers:                 DefaultEventWorkers,

		NodeLeaseDuration: node.DefaultLeaseDuration * time.Second,
		NodePingInterval:  node.DefaultPingInterval,
		NodePingTimeout:   DefaultNodePingTimeout,
		NodeCheckNetwork:  DefaultNodeCheckNetwork,
	}
}
