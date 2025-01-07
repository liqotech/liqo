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

package connection

import (
	"time"

	"github.com/liqotech/liqo/pkg/gateway"
	"github.com/liqotech/liqo/pkg/gateway/connection/conncheck"
)

// Options contains the options for the wireguard interface.
type Options struct {
	// EnableConnectionController enables the connection controller.
	EnableConnectionController bool
	// GwOptions contains the options for the wireguard interface.
	GwOptions *gateway.Options
	// ConnCheckOptions contains the options for the connchecker.
	ConnCheckOptions *conncheck.Options
	// PingEnabled enables the ping check.
	PingEnabled bool
	// PingUpdateStatusInterval is the interval at which the status is updated.
	PingUpdateStatusInterval time.Duration
}

// NewOptions returns a new Options struct.
func NewOptions(gwOptions *gateway.Options,
	conncheckOptions *conncheck.Options) *Options {
	return &Options{
		GwOptions:        gwOptions,
		ConnCheckOptions: conncheckOptions,
	}
}
