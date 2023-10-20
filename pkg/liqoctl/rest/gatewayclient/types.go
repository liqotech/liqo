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

package gatewayclient

import (
	"k8s.io/client-go/kubernetes"

	rest "github.com/liqotech/liqo/pkg/liqoctl/rest"
)

// Default values for the gatewayclient command.
const (
	DefaultGatewayType       = "networking.liqo.io/v1alpha1/wggatewayclienttemplates"
	DefaultTemplateName      = "wireguard-client"
	DefaultTemplateNamespace = "liqo"
	DefaultMTU               = 1340
	DefaultProtocol          = "UDP"
	DefaultWait              = false
)

// Options encapsulates the arguments of the gatewayclient command.
type Options struct {
	createOptions *rest.CreateOptions

	ClusterID         string
	GatewayType       string
	TemplateName      string
	TemplateNamespace string
	MTU               int
	Addresses         []string
	Port              int32
	Protocol          string
	Wait              bool
}

var _ rest.API = &Options{}

// GatewayClient returns the rest API for the gatewayclient command.
func GatewayClient() rest.API {
	return &Options{}
}

// APIOptions returns the APIOptions for the gatewayclient API.
func (o *Options) APIOptions() *rest.APIOptions {
	return &rest.APIOptions{
		EnableCreate: true,
	}
}

// ForgeOptions encapsulate the options to forge a gatewayclient.
type ForgeOptions struct {
	KubeClient        kubernetes.Interface
	RemoteClusterID   string
	GatewayType       string
	TemplateName      string
	TemplateNamespace string
	MTU               int
	Addresses         []string
	Port              int32
	Protocol          string
}

func (o *Options) getForgeOptions() *ForgeOptions {
	return &ForgeOptions{
		KubeClient:        o.createOptions.KubeClient,
		RemoteClusterID:   o.ClusterID,
		GatewayType:       o.GatewayType,
		TemplateName:      o.TemplateName,
		TemplateNamespace: o.TemplateNamespace,
		MTU:               o.MTU,
		Addresses:         o.Addresses,
		Port:              o.Port,
		Protocol:          o.Protocol,
	}
}
