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

package gatewayclient

import (
	"github.com/liqotech/liqo/pkg/liqo-controller-manager/networking/forge"
	"github.com/liqotech/liqo/pkg/liqoctl/rest"
	"github.com/liqotech/liqo/pkg/utils/args"
)

// Options encapsulates the arguments of the gatewayclient command.
type Options struct {
	createOptions *rest.CreateOptions
	deleteOptions *rest.DeleteOptions

	RemoteClusterID   args.ClusterIDFlags
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
		EnableDelete: true,
	}
}

func (o *Options) getForgeOptions() *forge.GwClientOptions {
	if o.TemplateNamespace == "" {
		o.TemplateNamespace = o.createOptions.LiqoNamespace
	}

	return &forge.GwClientOptions{
		KubeClient:        o.createOptions.KubeClient,
		RemoteClusterID:   o.RemoteClusterID.GetClusterID(),
		GatewayType:       o.GatewayType,
		TemplateName:      o.TemplateName,
		TemplateNamespace: o.TemplateNamespace,
		MTU:               o.MTU,
		Addresses:         o.Addresses,
		Port:              o.Port,
		Protocol:          o.Protocol,
	}
}
