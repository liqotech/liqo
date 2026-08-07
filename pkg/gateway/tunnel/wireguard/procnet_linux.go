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

//go:build linux

package wireguard

import (
	"fmt"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

// udpQueueStats holds the current send/receive queue lengths (in bytes) for
// the WireGuard UDP socket(s), expressed as the sum across all matching sockets.
type udpQueueStats struct {
	txBytes uint64
	rxBytes uint64
}

// readUDPQueueStats returns the current tx/rx queue byte counters of the
// WireGuard UDP socket(s) bound to localPort, using the INET_DIAG netlink
// interface (kernel ≥ 2.6.32). It works for both server (where the listen
// port is set explicitly) and client (where the kernel auto-binds an
// ephemeral port and reflects it in the device's netlink attributes) modes.
//
// When localPort <= 0 — for example with userspace WireGuard, which does not
// bind a kernel UDP socket — this function returns an error. The metrics
// collector handles that by reporting invalid metrics, so the failure is
// visible in Prometheus rather than being masked as zero queue bytes.
func readUDPQueueStats(localPort int) (*udpQueueStats, error) {
	if localPort <= 0 || localPort > 65535 {
		return nil, fmt.Errorf("invalid local port %d", localPort)
	}

	// WireGuard tunnels in Liqo use IPv4 endpoints. Enumerate UDP sockets for
	// AF_INET only; if v6 support is ever added to the wireguard tunnels here,
	// a parallel call for unix.AF_INET6 and a sum would be straightforward.
	socks, err := netlink.SocketDiagUDP(unix.AF_INET)
	if err != nil {
		return nil, fmt.Errorf("listing UDP sockets via INET_DIAG: %w", err)
	}

	stats, found := filterUDPQueueStatsByPort(socks, localPort)
	if !found {
		return nil, fmt.Errorf("no UDP socket found for port %d", localPort)
	}
	return stats, nil
}

// filterUDPQueueStatsByPort sums the tx/rx queue bytes of all UDP sockets
// matching the given local source port. Multiple rows can match the same
// port when several endpoints share it (e.g. IPv4-mapped or duplicate binds);
// in that case we sum, matching the previous /proc/net/udp semantics.
//
// Exposed at package scope so it can be unit-tested with synthetic socket
// rows without needing CAP_NET_ADMIN.
func filterUDPQueueStatsByPort(socks []*netlink.Socket, localPort int) (*udpQueueStats, bool) {
	var stats udpQueueStats
	var found bool
	for _, s := range socks {
		if s == nil {
			continue
		}
		if int(s.ID.SourcePort) != localPort {
			continue
		}
		stats.txBytes += uint64(s.WQueue)
		stats.rxBytes += uint64(s.RQueue)
		found = true
	}
	if !found {
		return nil, false
	}
	return &stats, true
}
