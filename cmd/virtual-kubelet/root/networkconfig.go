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

package root

import (
	"fmt"

	"github.com/liqotech/liqo/pkg/virtualKubelet/networkconfig"
)

// CIDRPair holds a set of CIDRs and their local remappings.
type CIDRPair struct {
	Original []string
	Remapped []string
}

// RemoteCIDR holds the remote pod and external CIDRs, with their local remappings.
type RemoteCIDR struct {
	Pod      CIDRPair
	External CIDRPair
}

// buildRemoteCIDR builds a RemoteCIDR from the CLI flags.
// It returns nil when no network CIDRs are provided (networking module disabled).
func buildRemoteCIDR(c *Opts) (*networkconfig.RemoteCIDR, error) {
	if len(c.RemotePodCIDR) == 0 && len(c.RemotePodCIDRRemap) == 0 &&
		len(c.RemoteExternalCIDR) == 0 && len(c.RemoteExternalCIDRRemap) == 0 {
		return nil, nil
	}

	if len(c.RemotePodCIDR) != len(c.RemotePodCIDRRemap) {
		return nil, fmt.Errorf("remote pod CIDR/remap lengths do not match: %d vs %d",
			len(c.RemotePodCIDR), len(c.RemotePodCIDRRemap))
	}
	if len(c.RemoteExternalCIDR) != len(c.RemoteExternalCIDRRemap) {
		return nil, fmt.Errorf("remote external CIDR/remap lengths do not match: %d vs %d",
			len(c.RemoteExternalCIDR), len(c.RemoteExternalCIDRRemap))
	}

	return &networkconfig.RemoteCIDR{
		Pod: networkconfig.CIDRPair{
			Original: c.RemotePodCIDR,
			Remapped: c.RemotePodCIDRRemap,
		},
		External: networkconfig.CIDRPair{
			Original: c.RemoteExternalCIDR,
			Remapped: c.RemoteExternalCIDRRemap,
		},
	}, nil
}
