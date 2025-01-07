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

package wireguard

import (
	"fmt"
	"net"

	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"k8s.io/klog/v2"

	"github.com/liqotech/liqo/pkg/gateway"
	"github.com/liqotech/liqo/pkg/gateway/tunnel"
)

func configureDevice(wgcl *wgctrl.Client, options *Options, peerPubKey wgtypes.Key) error {
	confdev := wgtypes.Config{
		PrivateKey: &options.PrivateKey,
		ListenPort: nil,
		Peers: []wgtypes.PeerConfig{
			{
				PublicKey:  peerPubKey,
				AllowedIPs: []net.IPNet{{IP: net.IP{0, 0, 0, 0}, Mask: net.CIDRMask(0, 32)}},
			},
		},
		ReplacePeers: true,
	}

	switch options.GwOptions.Mode {
	case gateway.ModeServer:
		confdev.ListenPort = &options.ListenPort
	case gateway.ModeClient:
		confdev.Peers[0].Endpoint = &net.UDPAddr{
			IP:   options.EndpointIP,
			Port: options.EndpointPort,
		}
	}

	klog.Infof("Configuring device %s", tunnel.TunnelInterfaceName)

	if err := wgcl.ConfigureDevice(tunnel.TunnelInterfaceName, confdev); err != nil {
		return fmt.Errorf("an error occurred while configuring the device: %w", err)
	}
	return nil
}
