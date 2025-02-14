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

package consts

import "time"

const (
	// DefaultGenevePort is the default port used for the geneve tunnel.
	DefaultGenevePort uint16 = 6091
	// DefaultGeneveCleanupInterval is the default interval used to cleanup the geneve tunnels.
	DefaultGeneveCleanupInterval = time.Minute * 30
	// DefaultRouteTable is the name of the default table used for routes.
	DefaultRouteTable = "liqo"
	// InternalFabricName is the label used to identify the internal fabric name.
	InternalFabricName = "networking.liqo.io/internal-fabric-name"
	// InternalNodeName is the label used to identify the internal node name.
	InternalNodeName = "networking.liqo.io/internal-node-name"
	// InternalFabricGeneveTunnelFinalizer is the finalizer used to ensure that the geneve tunnel is deleted and the
	// id is freed.
	InternalFabricGeneveTunnelFinalizer = "networking.liqo.io/internal-fabric-geneve-tunnel-finalizer"
)
