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

package wireguard

import (
	"github.com/vishvananda/netlink"
	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"k8s.io/klog/v2"

	"github.com/liqotech/liqo/pkg/gateway/tunnel/common"
)

const (
	// ServerInterfaceIP is the IP address of the Wireguard interface in server mode.
	ServerInterfaceIP = "169.254.0.1/30"
	// ClientInterfaceIP is the IP address of the Wireguard interface in client mode.
	ClientInterfaceIP = "169.254.0.2/30"
)

// InitWireguardLink inits the Wireguard interface.
func InitWireguardLink(options *Options) error {
	if err := createLink(options); err != nil {
		return err
	}

	link, err := common.GetLink(options.InterfaceName)
	if err != nil {
		return err
	}

	klog.Infof("Setting up Wireguard interface %q with IP %q", options.InterfaceName, GetInterfaceIP(options.Mode))
	if err := common.AddAddress(link, GetInterfaceIP(options.Mode)); err != nil {
		return err
	}

	return netlink.LinkSetUp(link)
}

// GetInterfaceIP returns the IP address of the Wireguard interface.
func GetInterfaceIP(mode common.Mode) string {
	switch mode {
	case common.ModeServer:
		return ServerInterfaceIP
	case common.ModeClient:
		return ClientInterfaceIP
	}
	return ""
}

// CreateLink creates a new Wireguard interface.
func createLink(options *Options) error {
	link := netlink.Wireguard{
		LinkAttrs: netlink.LinkAttrs{
			MTU:  options.MTU,
			Name: options.InterfaceName,
		},
	}

	err := netlink.LinkAdd(&link)
	if err != nil {
		return err
	}

	if options.Mode == common.ModeServer {
		wgcl, err := wgctrl.New()
		if err != nil {
			return err
		}
		defer wgcl.Close()

		if err := wgcl.ConfigureDevice(options.InterfaceName, wgtypes.Config{
			ListenPort: &options.ListenPort,
		}); err != nil {
			return err
		}
	}
	return nil
}
