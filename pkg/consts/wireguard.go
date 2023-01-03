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

package consts

const (
	// PublicKey is the key of publicKey entry in back-end map and also for the secret containing the wireguard keys.
	PublicKey = "publicKey"
	// ListeningPort is the key of the listeningPort entry in the back-end map.
	ListeningPort = "port"
	// DeviceName name of wireguard tunnel created on the custom network namespace.
	DeviceName = "liqo.tunnel"
	// DriverName  name of the driver which is also used as the type of the backend in tunnelendpoint CRD.
	DriverName = "wireguard"
	// KeysLabel label for the secret that contains the public key.
	KeysLabel = "net.liqo.io/key"
	// WgTunnelIP is the IP address of the wireguard tunnel interface.
	WgTunnelIP = "169.254.0.1"
	// WgEndpointIP is the key of the endpointIP entry in back-end map of wireguard interface.
	WgEndpointIP = "endpointIP"
	// WgPrivateKey is the key of the private key entry for the secret containing the wireguard keys.
	WgPrivateKey = "privateKey"
	// WgAllowedIPs is the key of the allowedIPs entry in the back-end map of wireguard interface.
	WgAllowedIPs = "allowedIPs"
	// WgKeysName is the name of the secret that contains the public key used by wireguard.
	WgKeysName = "wireguard-pubkey"
)
