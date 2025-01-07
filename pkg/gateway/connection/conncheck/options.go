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

package conncheck

import "time"

// Options contains the options for the wireguard interface.
type Options struct {
	// PingPort is the port used for the ping check.
	PingPort int
	// PingBufferSize is the size of the buffer used for the ping check.
	PingBufferSize uint
	// PingLossThreshold is the number of lost packets after which the connection check is considered as failed.
	PingLossThreshold uint
	// PingInterval is the interval at which the ping is sent.
	PingInterval time.Duration
}

// NewOptions returns a new Options struct.
func NewOptions() *Options {
	return &Options{}
}
