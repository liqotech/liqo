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
	"k8s.io/utils/pointer"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
	argsutils "github.com/liqotech/liqo/pkg/utils/args"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/generic"
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
var DefaultReflectorsWorkers = map[generic.ResourceReflected]uint{
	generic.Pod:                   10,
	generic.Service:               3,
	generic.EndpointSlice:         10,
	generic.Ingress:               3,
	generic.ConfigMap:             3,
	generic.Secret:                3,
	generic.ServiceAccount:        3,
	generic.PersistentVolumeClaim: 3,
	generic.Event:                 3,
}

// DefaultReflectorsTypes contains the default type of reflection for each reflected resource.
var DefaultReflectorsTypes = map[generic.ResourceReflected]consts.ReflectionType{
	generic.Pod:                   consts.CustomLiqo,
	generic.Service:               consts.DenyList,
	generic.Ingress:               consts.DenyList,
	generic.ConfigMap:             consts.DenyList,
	generic.Secret:                consts.DenyList,
	generic.ServiceAccount:        consts.CustomLiqo,
	generic.PersistentVolumeClaim: consts.CustomLiqo,
	generic.Event:                 consts.DenyList,
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

	// Number of workers to use for each refleted resource
	ReflectorsWorkers map[string]*uint

	// Type of reflection to use for each reflected resource
	ReflectorsType map[string]*string

	LabelsNotReflected      argsutils.StringList
	AnnotationsNotReflected argsutils.StringList

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
		InformerResyncPeriod: DefaultInformerResyncPeriod,

		DisableIPReflection: false,

		CertificateType: argsutils.NewEnum([]string{CertificateTypeKubelet, CertificateTypeAWS, CertificateTypeSelfSigned}, CertificateTypeKubelet),
		ListenPort:      DefaultListenPort,
		EnableProfiling: false,

		ReflectorsWorkers: initReflectionWorkers(),
		ReflectorsType:    initReflectionType(),

		LabelsNotReflected:      argsutils.StringList{},
		AnnotationsNotReflected: argsutils.StringList{},

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
	reflectionWorkers := make(map[string]*uint, len(generic.Reflectors))
	for i := range generic.Reflectors {
		resource := &generic.Reflectors[i]
		reflectionWorkers[string(*resource)] = pointer.Uint(DefaultReflectorsWorkers[*resource])
	}
	return reflectionWorkers
}

func initReflectionType() map[string]*string {
	reflectionType := make(map[string]*string, len(generic.ReflectorsCustomizableType))
	for i := range generic.ReflectorsCustomizableType {
		resource := &generic.ReflectorsCustomizableType[i]
		reflectionType[string(*resource)] = pointer.String(string(DefaultReflectorsTypes[*resource]))
	}
	return reflectionType
}
