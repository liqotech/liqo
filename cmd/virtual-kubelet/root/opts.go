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
	"k8s.io/utils/ptr"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	argsutils "github.com/liqotech/liqo/pkg/utils/args"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/resources"
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
	DefaultNodePingTimeout      = 1 * time.Second
	DefaultNodeCheckNetwork     = true
)

// DefaultReflectorsWorkers contains the default number of workers for each reflected resource.
var DefaultReflectorsWorkers = map[resources.ResourceReflected]uint{
	resources.Pod:                   10,
	resources.Service:               3,
	resources.EndpointSlice:         10,
	resources.Ingress:               3,
	resources.ConfigMap:             3,
	resources.Secret:                3,
	resources.ServiceAccount:        3,
	resources.PersistentVolumeClaim: 3,
	resources.Event:                 3,
}

// DefaultReflectorsTypes contains the default type of reflection for each reflected resource.
var DefaultReflectorsTypes = map[resources.ResourceReflected]offloadingv1beta1.ReflectionType{
	resources.Pod:                   offloadingv1beta1.CustomLiqo,
	resources.Service:               offloadingv1beta1.DenyList,
	resources.Ingress:               offloadingv1beta1.DenyList,
	resources.ConfigMap:             offloadingv1beta1.DenyList,
	resources.Secret:                offloadingv1beta1.DenyList,
	resources.ServiceAccount:        offloadingv1beta1.CustomLiqo,
	resources.PersistentVolumeClaim: offloadingv1beta1.CustomLiqo,
	resources.Event:                 offloadingv1beta1.DenyList,
}

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
	LiqoNamespace        string
	InformerResyncPeriod time.Duration

	HomeCluster         argsutils.ClusterIDFlags
	ForeignCluster      argsutils.ClusterIDFlags
	DisableIPReflection bool
	LocalPodCIDR        string

	// Sets the addresses to listen for requests from the Kubernetes API server
	NodeIP          string
	ListenPort      uint16
	CertificateType *argsutils.StringEnum
	EnableProfiling bool

	// Number of workers to use for each refleted resource
	ReflectorsWorkers map[string]*uint

	// Type of reflection to use for each reflected resource
	ReflectorsType map[string]*string

	NodeLeaseDuration time.Duration
	NodePingInterval  time.Duration
	NodePingTimeout   time.Duration
	NodeCheckNetwork  bool

	NodeExtraAnnotations argsutils.StringMap
	NodeExtraLabels      argsutils.StringMap

	EnableAPIServerSupport          bool
	EnableStorage                   bool
	VirtualStorageClassName         string
	RemoteRealStorageClassName      string
	EnableIngress                   bool
	RemoteRealIngressClassName      string
	EnableLoadBalancer              bool
	RemoteRealLoadBalancerClassName string
	EnableMetrics                   bool
	MetricsAddress                  string

	HomeAPIServerHost string
	HomeAPIServerPort string

	CreateNode bool

	VirtualKubeletLeaseEnabled       bool
	VirtualKubeletLeaseLeaseDuration time.Duration
	VirtualKubeletLeaseRenewDeadline time.Duration
	VirtualKubeletLeaseRetryPeriod   time.Duration
}

// NewOpts returns an Opts struct with the default values set.
func NewOpts() *Opts {
	return &Opts{
		HomeKubeconfig:       os.Getenv("KUBECONFIG"),
		NodeName:             DefaultNodeName,
		PodName:              os.Getenv("POD_NAME"),
		TenantNamespace:      corev1.NamespaceDefault,
		LiqoNamespace:        consts.DefaultLiqoNamespace,
		InformerResyncPeriod: DefaultInformerResyncPeriod,

		DisableIPReflection: false,

		CertificateType: argsutils.NewEnum([]string{CertificateTypeKubelet, CertificateTypeAWS, CertificateTypeSelfSigned}, CertificateTypeKubelet),
		ListenPort:      DefaultListenPort,
		EnableProfiling: false,

		ReflectorsWorkers: initReflectionWorkers(),
		ReflectorsType:    initReflectionType(),

		NodeLeaseDuration: node.DefaultLeaseDuration * time.Second,
		NodePingInterval:  node.DefaultPingInterval,
		NodePingTimeout:   DefaultNodePingTimeout,
		NodeCheckNetwork:  DefaultNodeCheckNetwork,

		VirtualKubeletLeaseEnabled:       true,
		VirtualKubeletLeaseLeaseDuration: 15 * time.Second,
		VirtualKubeletLeaseRenewDeadline: 10 * time.Second,
		VirtualKubeletLeaseRetryPeriod:   5 * time.Second,
	}
}

func initReflectionWorkers() map[string]*uint {
	reflectionWorkers := make(map[string]*uint, len(resources.Reflectors))
	for i := range resources.Reflectors {
		resource := &resources.Reflectors[i]
		reflectionWorkers[string(*resource)] = ptr.To(DefaultReflectorsWorkers[*resource])
	}
	return reflectionWorkers
}

func initReflectionType() map[string]*string {
	reflectionType := make(map[string]*string, len(resources.ReflectorsCustomizableType))
	for i := range resources.ReflectorsCustomizableType {
		resource := &resources.ReflectorsCustomizableType[i]
		reflectionType[string(*resource)] = ptr.To(string(DefaultReflectorsTypes[*resource]))
	}
	return reflectionType
}
