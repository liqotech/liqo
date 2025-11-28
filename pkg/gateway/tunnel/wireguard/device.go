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

		endpoint := getExistingEndpoint(wgcl, peerPubKey)
		if endpoint != nil {
			confdev.Peers[0].Endpoint = endpoint
		}
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

func getExistingEndpoint(wgcl *wgctrl.Client, peerPubKey wgtypes.Key) *net.UDPAddr {
	peer := getExistingPeer(wgcl, peerPubKey)

	if peer == nil {
		return nil
	}

	if peer.Endpoint != nil {
		klog.Infof("Discovered endpoint %s for peer %s", peer.Endpoint, peerPubKey.String())
		return peer.Endpoint
	}

	return nil
}

func getExistingPeer(wgcl *wgctrl.Client, peerPubKey wgtypes.Key) *wgtypes.Peer {
	dev := getExistingDevice(wgcl)

	if dev == nil {
		return nil
	}

	for i := range dev.Peers {
		if dev.Peers[i].PublicKey == peerPubKey {
			klog.Infof("Found existing peer for key %s", peerPubKey.String())
			return &dev.Peers[i]
		}
	}

	klog.Infof("No existing peer %s found", peerPubKey.String())
	return nil
}

func getExistingDevice(wgcl *wgctrl.Client) *wgtypes.Device {
	dev, err := wgcl.Device(tunnel.TunnelInterfaceName)

	if err == nil {
		klog.Infof("Found existing device %s", tunnel.TunnelInterfaceName)
		return dev
	}

	klog.Infof("No existing device %s found", tunnel.TunnelInterfaceName)
	return nil
}
