// Copyright 2019-2026 The Liqo Authors
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
	"net"
	"os"

	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"k8s.io/klog/v2"

	"github.com/liqotech/liqo/pkg/gateway"
	"github.com/liqotech/liqo/pkg/gateway/tunnel"
)

var errWgEndpointPeerNotFound = errors.New("wg endpoint peer not found")

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
		if options.PreserveClientEndpoint {
			endpoint, err := getExistingPeerEndpoint(wgcl, peerPubKey)
			switch {
			case err == nil:
				klog.Infof("Found existing endpoint %s for current peer. Re-using it.", endpoint.String())
				confdev.Peers[0].Endpoint = endpoint
			case errors.Is(err, errWgEndpointPeerNotFound):
				klog.Infof("Skipping peer endpoint preservation: %v", err)
			default:
				return fmt.Errorf("getting existing peer endpoint: %w", err)
			}
		}
	case gateway.ModeClient:
		confdev.Peers[0].Endpoint = &net.UDPAddr{
			IP:   options.EndpointIP,
			Port: options.EndpointPort,
		}
	}

	if err := wgcl.ConfigureDevice(tunnel.TunnelInterfaceName, confdev); err != nil {
		return fmt.Errorf("an error occurred while configuring the device: %w", err)
	}
	klog.Infof("Device %s configured", tunnel.TunnelInterfaceName)

	return nil
}

func getExistingPeerEndpoint(wgcl *wgctrl.Client, peerPubKey wgtypes.Key) (*net.UDPAddr, error) {
	peer, err := getExistingPeer(wgcl, peerPubKey)
	if err != nil {
		return nil, fmt.Errorf("getting existing peer: %w", err)
	}
	if peer.Endpoint == nil {
		return nil, fmt.Errorf("peer has no endpoint: %w", errWgEndpointPeerNotFound)
	}
	return peer.Endpoint, nil
}

func getExistingPeer(wgcl *wgctrl.Client, peerPubKey wgtypes.Key) (*wgtypes.Peer, error) {
	dev, err := getExistingDevice(wgcl)
	if err != nil {
		return nil, fmt.Errorf("getting existing device: %w", err)
	}
	for i := range dev.Peers {
		if dev.Peers[i].PublicKey == peerPubKey {
			return &dev.Peers[i], nil
		}
	}
	return nil, fmt.Errorf("no matching peer: %w", errWgEndpointPeerNotFound)
}

func getExistingDevice(wgcl *wgctrl.Client) (*wgtypes.Device, error) {
	dev, err := wgcl.Device(tunnel.TunnelInterfaceName)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("device %s not found: %w", tunnel.TunnelInterfaceName, errWgEndpointPeerNotFound)
		}
		return nil, fmt.Errorf("fetching device: %w", err)
	}
	return dev, nil
}
