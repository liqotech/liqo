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
	"errors"
	"fmt"

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
	exists, err := existsLink()
	if err != nil {
		return fmt.Errorf("cannot check if Wireguard interface exists: %w", err)
	}
	if exists {
		klog.Infof("Wireguard interface %q already exists", tunnel.TunnelInterfaceName)
		return nil
	}

	if err := createLink(options); err != nil {
		return fmt.Errorf("cannot create Wireguard interface: %w", err)
	}

	link, err := common.GetLink(tunnel.TunnelInterfaceName)
	if err != nil {
		return fmt.Errorf("cannot get Wireguard interface: %w", err)
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
		return fmt.Errorf("cannot add Wireguard interface: %w", err)
	}

	if options.GwOptions.Mode == gateway.ModeServer {
		wgcl, err := wgctrl.New()
		if err != nil {
			return fmt.Errorf("cannot create Wireguard client: %w", err)
		}
		defer wgcl.Close()

		if err := wgcl.ConfigureDevice(tunnel.TunnelInterfaceName, wgtypes.Config{
			ListenPort: &options.ListenPort,
		}); err != nil {
			return fmt.Errorf("cannot configure Wireguard interface: %w", err)
		}
	}
	return nil
}

func existsLink() (bool, error) {
	_, err := common.GetLink(tunnel.TunnelInterfaceName)
	if err != nil {
		if errors.As(err, &netlink.LinkNotFoundError{}) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
