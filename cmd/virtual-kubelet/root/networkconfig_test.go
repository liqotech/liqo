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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/liqotech/liqo/pkg/virtualKubelet/networkconfig"
)

func TestBuildRemoteCIDR(t *testing.T) {
	t.Run("no flags returns nil", func(t *testing.T) {
		cfg, err := buildRemoteCIDR(&Opts{})
		require.NoError(t, err)
		assert.Nil(t, cfg)
	})

	t.Run("valid flags build a remote CIDR", func(t *testing.T) {
		o := &Opts{
			RemotePodCIDR:         []string{"10.0.0.0/16"},
			RemotePodCIDRRemap:    []string{"192.168.0.0/16"},
			RemoteExternalCIDR:    []string{"172.16.0.0/16"},
			RemoteExternalCIDRRemap: []string{"10.1.0.0/16"},
		}
		cfg, err := buildRemoteCIDR(o)
		require.NoError(t, err)
		require.NotNil(t, cfg)

		assert.Equal(t, &networkconfig.RemoteCIDR{
			Pod: networkconfig.CIDRPair{
				Original: []string{"10.0.0.0/16"},
				Remapped: []string{"192.168.0.0/16"},
			},
			External: networkconfig.CIDRPair{
				Original: []string{"172.16.0.0/16"},
				Remapped: []string{"10.1.0.0/16"},
			},
		}, cfg)
	})

	t.Run("mismatched pod lengths return an error", func(t *testing.T) {
		o := &Opts{
			RemotePodCIDR:         []string{"10.0.0.0/16"},
			RemotePodCIDRRemap:    []string{},
			RemoteExternalCIDR:    []string{"172.16.0.0/16"},
			RemoteExternalCIDRRemap: []string{"10.1.0.0/16"},
		}
		_, err := buildRemoteCIDR(o)
		require.Error(t, err)
	})

	t.Run("mismatched external lengths return an error", func(t *testing.T) {
		o := &Opts{
			RemotePodCIDR:         []string{"10.0.0.0/16"},
			RemotePodCIDRRemap:    []string{"192.168.0.0/16"},
			RemoteExternalCIDR:    []string{"172.16.0.0/16", "172.17.0.0/16"},
			RemoteExternalCIDRRemap: []string{"10.1.0.0/16"},
		}
		_, err := buildRemoteCIDR(o)
		require.Error(t, err)
	})
}
