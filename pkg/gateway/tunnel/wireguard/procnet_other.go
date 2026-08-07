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

//go:build !linux

package wireguard

import "fmt"

// udpQueueStats is defined on Linux; this stub matches the type so the
// non-Linux build can keep the metrics package compiling.
type udpQueueStats struct {
	txBytes uint64
	rxBytes uint64
}

// readUDPQueueStats is unavailable on non-Linux platforms because the
// vishvananda/netlink INET_DIAG helpers used here are Linux-only. The
// metrics collector treats this as an unrecoverable error and falls back
// to emitting 0 with a logged warning, which is the correct behavior for
// a non-Linux host (the gateway runs on Linux in production).
func readUDPQueueStats(_ int) (*udpQueueStats, error) {
	return nil, fmt.Errorf("UDP queue statistics are only supported on Linux")
}
