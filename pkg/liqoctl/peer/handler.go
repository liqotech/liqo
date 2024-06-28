// Copyright 2019-2024 The Liqo Authors
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

package peer

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"

	nwforge "github.com/liqotech/liqo/pkg/liqo-controller-manager/networking/forge"
	"github.com/liqotech/liqo/pkg/liqoctl/authenticate"
	"github.com/liqotech/liqo/pkg/liqoctl/factory"
	"github.com/liqotech/liqo/pkg/liqoctl/network"
	"github.com/liqotech/liqo/pkg/liqoctl/rest"
	"github.com/liqotech/liqo/pkg/liqoctl/rest/resourceslice"
	tenantnamespace "github.com/liqotech/liqo/pkg/tenantNamespace"
	liqoutils "github.com/liqotech/liqo/pkg/utils"
	argsutils "github.com/liqotech/liqo/pkg/utils/args"
)

// Options encapsulates the arguments of the peer command.
type Options struct {
	LocalFactory  *factory.Factory
	RemoteFactory *factory.Factory
	Timeout       time.Duration

	// Networking options
	NetworkingDisabled bool
	ServerServiceType  *argsutils.StringEnum
	ServerPort         int32
	MTU                int

	// Authentication options
	CreateResourceSlice bool
	ResourceSliceClass  string
	ProxyURL            string

	// Offloading options
	CreateVirtualNode bool
	CPU               string
	Memory            string
	Pods              string
}

// NewOptions returns a new Options struct.
func NewOptions(localFactory *factory.Factory) *Options {
	return &Options{
		LocalFactory: localFactory,
		ServerServiceType: argsutils.NewEnum(
			[]string{string(corev1.ServiceTypeLoadBalancer), string(corev1.ServiceTypeNodePort)}, string(nwforge.DefaultGwServerServiceType)),
	}
}

// RunPeer implements the peer command.
func (o *Options) RunPeer(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, o.Timeout)
	defer cancel()

	// Ensure networking
	if !o.NetworkingDisabled {
		if err := ensureNetworking(ctx, o); err != nil {
			o.LocalFactory.Printer.CheckErr(fmt.Errorf("unable to ensure networking: %w", err))
			return err
		}
	}

	// Ensure authentication
	if err := ensureAuthentication(ctx, o); err != nil {
		o.LocalFactory.Printer.CheckErr(fmt.Errorf("unable to ensure authentication: %w", err))
		return err
	}

	// Ensure offloading
	if o.CreateResourceSlice {
		if err := ensureOffloading(ctx, o); err != nil {
			o.LocalFactory.Printer.CheckErr(fmt.Errorf("unable to ensure offloading: %w", err))
			return err
		}
	}

	return nil
}

func ensureNetworking(ctx context.Context, o *Options) error {
	networkOptions := network.Options{
		LocalFactory:  o.LocalFactory,
		RemoteFactory: o.RemoteFactory,

		Timeout: o.Timeout,
		Wait:    true,

		ServerGatewayType:       nwforge.DefaultGwServerType,
		ServerTemplateName:      nwforge.DefaultGwServerTemplateName,
		ServerTemplateNamespace: o.RemoteFactory.LiqoNamespace,
		ServerServiceType:       o.ServerServiceType,
		ServerPort:              o.ServerPort,
		ServerNodePort:          0,
		ServerLoadBalancerIP:    "",

		ClientGatewayType:       nwforge.DefaultGwClientType,
		ClientTemplateName:      nwforge.DefaultGwClientTemplateName,
		ClientTemplateNamespace: o.LocalFactory.LiqoNamespace,

		MTU:                o.MTU,
		DisableSharingKeys: false,
	}

	if err := networkOptions.RunInit(ctx); err != nil {
		return err
	}

	if err := networkOptions.RunConnect(ctx); err != nil {
		return err
	}

	return nil
}

func ensureAuthentication(ctx context.Context, o *Options) error {
	authOptions := authenticate.Options{
		LocalFactory:  o.LocalFactory,
		RemoteFactory: o.RemoteFactory,
		Timeout:       o.Timeout,

		ProxyURL: o.ProxyURL,
	}

	if err := authOptions.RunAuthenticate(ctx); err != nil {
		return err
	}

	return nil
}

func ensureOffloading(ctx context.Context, o *Options) error {
	providerClusterID, err := liqoutils.GetClusterIDWithControllerClient(ctx, o.RemoteFactory.CRClient, o.RemoteFactory.LiqoNamespace)
	if err != nil {
		return err
	}

	providerClusterIDFlag := argsutils.ClusterIDFlags{}
	if err := providerClusterIDFlag.Set(string(providerClusterID)); err != nil {
		return err
	}

	rsOptions := resourceslice.Options{
		CreateOptions: &rest.CreateOptions{
			Factory: o.LocalFactory,
			Name:    string(providerClusterID),
		},

		NamespaceManager:           tenantnamespace.NewManager(o.LocalFactory.KubeClient),
		RemoteClusterID:            providerClusterIDFlag,
		Class:                      o.ResourceSliceClass,
		DisableVirtualNodeCreation: !o.CreateVirtualNode,

		CPU:    o.CPU,
		Memory: o.Memory,
		Pods:   o.Pods,
	}

	if err := rsOptions.HandleCreate(ctx); err != nil {
		return err
	}

	return nil
}
