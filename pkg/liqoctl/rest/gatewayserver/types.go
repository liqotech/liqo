// Copyright 2019-2023 The Liqo Authors
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

package gatewayserver

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/ptr"

	"github.com/liqotech/liqo/pkg/liqoctl/rest"
	argsutils "github.com/liqotech/liqo/pkg/utils/args"
)

// Default values for the gatewayserver command.
const (
	DefaultGatewayType       = "networking.liqo.io/v1alpha1/wggatewayservertemplates"
	DefaultTemplateName      = "wireguard-server"
	DefaultTemplateNamespace = "liqo"
	DefaultServiceType       = corev1.ServiceTypeLoadBalancer
	DefaultMTU               = 1340
	DefaultPort              = 51820
	DefaultProxy             = false
	DefaultWait              = false
)

// Options encapsulates the arguments of the gatewayserver command.
type Options struct {
	createOptions *rest.CreateOptions
	deleteOptions *rest.DeleteOptions

	RemoteClusterID   string
	GatewayType       string
	TemplateName      string
	TemplateNamespace string
	ServiceType       *argsutils.StringEnum
	MTU               int
	Port              int32
	NodePort          int32
	LoadBalancerIP    string
	Proxy             bool
	Wait              bool
}

var _ rest.API = &Options{}

// GatewayServer returns the rest API for the gatewayserver command.
func GatewayServer() rest.API {
	return &Options{
		ServiceType: argsutils.NewEnum(
			[]string{string(corev1.ServiceTypeLoadBalancer), string(corev1.ServiceTypeNodePort)}, string(DefaultServiceType)),
	}
}

// APIOptions returns the APIOptions for the gatewayserver API.
func (o *Options) APIOptions() *rest.APIOptions {
	return &rest.APIOptions{
		EnableCreate: true,
		EnableDelete: true,
	}
}

// ForgeOptions encapsulate the options to forge a gatewayserver.
type ForgeOptions struct {
	KubeClient        kubernetes.Interface
	RemoteClusterID   string
	GatewayType       string
	TemplateName      string
	TemplateNamespace string
	ServiceType       corev1.ServiceType
	MTU               int
	Port              int32
	NodePort          *int32
	LoadBalancerIP    *string
	Proxy             bool
}

func (o *Options) getForgeOptions() *ForgeOptions {
	return &ForgeOptions{
		KubeClient:        o.createOptions.KubeClient,
		RemoteClusterID:   o.RemoteClusterID,
		GatewayType:       o.GatewayType,
		TemplateName:      o.TemplateName,
		TemplateNamespace: o.TemplateNamespace,
		ServiceType:       corev1.ServiceType(o.ServiceType.Value),
		MTU:               o.MTU,
		Port:              o.Port,
		NodePort:          ptr.To(o.NodePort),
		LoadBalancerIP:    ptr.To(o.LoadBalancerIP),
		Proxy:             o.Proxy,
	}
}
