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

package gatewayserver

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"

	"github.com/liqotech/liqo/pkg/liqo-controller-manager/networking/forge"
	"github.com/liqotech/liqo/pkg/liqoctl/rest"
	argsutils "github.com/liqotech/liqo/pkg/utils/args"
)

// Options encapsulates the arguments of the gatewayserver command.
type Options struct {
	createOptions *rest.CreateOptions
	deleteOptions *rest.DeleteOptions

	RemoteClusterID   argsutils.ClusterIDFlags
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
			[]string{string(corev1.ServiceTypeLoadBalancer), string(corev1.ServiceTypeNodePort)}, string(forge.DefaultGwServerServiceType)),
	}
}

// APIOptions returns the APIOptions for the gatewayserver API.
func (o *Options) APIOptions() *rest.APIOptions {
	return &rest.APIOptions{
		EnableCreate: true,
		EnableDelete: true,
	}
}

func (o *Options) getForgeOptions() *forge.GwServerOptions {
	if o.TemplateNamespace == "" {
		o.TemplateNamespace = o.createOptions.LiqoNamespace
	}

	return &forge.GwServerOptions{
		KubeClient:        o.createOptions.KubeClient,
		RemoteClusterID:   o.RemoteClusterID.GetClusterID(),
		GatewayType:       o.GatewayType,
		TemplateName:      o.TemplateName,
		TemplateNamespace: o.TemplateNamespace,
		ServiceType:       corev1.ServiceType(o.ServiceType.Value),
		MTU:               o.MTU,
		Port:              o.Port,
		NodePort:          ptr.To(o.NodePort),
		LoadBalancerIP:    ptr.To(o.LoadBalancerIP),
	}
}
