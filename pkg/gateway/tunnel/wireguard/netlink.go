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

package wireguard

import (
	"github.com/vishvananda/netlink"
	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"k8s.io/klog/v2"

	"github.com/liqotech/liqo/pkg/gateway"
	"github.com/liqotech/liqo/pkg/gateway/tunnel"
	"github.com/liqotech/liqo/pkg/gateway/tunnel/common"
)

// InitWireguardLink inits the Wireguard interface.
func InitWireguardLink(options *Options) error {
	if err := createLink(options); err != nil {
		return err
	}

	link, err := common.GetLink(tunnel.TunnelInterfaceName)
	if err != nil {
		return err
	}

	klog.Infof("Setting up Wireguard interface %q with IP %q", tunnel.TunnelInterfaceName, common.GetInterfaceIP(options.GwOptions.Mode))
	if err := common.AddAddress(link, common.GetInterfaceIP(options.GwOptions.Mode)); err != nil {
		return err
	}

	return netlink.LinkSetUp(link)
}

// CreateLink creates a new Wireguard interface.
func createLink(options *Options) error {
	link := netlink.Wireguard{
		LinkAttrs: netlink.LinkAttrs{
			MTU:  options.MTU,
			Name: tunnel.TunnelInterfaceName,
		},
	}

	err := netlink.LinkAdd(&link)
	if err != nil {
		return err
	}

	if options.GwOptions.Mode == gateway.ModeServer {
		wgcl, err := wgctrl.New()
		if err != nil {
			return err
		}
		defer wgcl.Close()

		if err := wgcl.ConfigureDevice(tunnel.TunnelInterfaceName, wgtypes.Config{
			ListenPort: &options.ListenPort,
		}); err != nil {
			return err
		}
	}
	return nil
}
