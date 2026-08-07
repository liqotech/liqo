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
	"testing"

	"github.com/vishvananda/netlink"
)

func TestFilterUDPQueueStatsByPort_Match(t *testing.T) {
	socks := []*netlink.Socket{
		{ID: netlink.SocketID{SourcePort: 1234}, WQueue: 10, RQueue: 20},
		{ID: netlink.SocketID{SourcePort: 8080}, WQueue: 100, RQueue: 200},
		{ID: netlink.SocketID{SourcePort: 8080}, WQueue: 1, RQueue: 2},
	}
	got, ok := filterUDPQueueStatsByPort(socks, 8080)
	if !ok {
		t.Fatal("expected match for port 8080")
	}
	// Multiple sockets on the same port must be summed.
	if got.txBytes != 101 || got.rxBytes != 202 {
		t.Fatalf("unexpected queue bytes: tx=%d rx=%d", got.txBytes, got.rxBytes)
	}
}

func TestFilterUDPQueueStatsByPort_NoMatch(t *testing.T) {
	socks := []*netlink.Socket{
		{ID: netlink.SocketID{SourcePort: 1234}, WQueue: 10, RQueue: 20},
	}
	if _, ok := filterUDPQueueStatsByPort(socks, 9999); ok {
		t.Fatal("expected no match for port 9999")
	}
}

func TestFilterUDPQueueStatsByPort_EmptyAndNil(t *testing.T) {
	if _, ok := filterUDPQueueStatsByPort(nil, 8080); ok {
		t.Fatal("expected no match for empty input")
	}
	socks := []*netlink.Socket{nil, {ID: netlink.SocketID{SourcePort: 1234}}}
	if _, ok := filterUDPQueueStatsByPort(socks, 1234); ok {
		t.Fatal("nil entries must not match")
	}
}

func TestReadUDPQueueStats_InvalidPort(t *testing.T) {
	if _, err := readUDPQueueStats(0); err == nil {
		t.Fatal("expected error for port 0")
	}
	if _, err := readUDPQueueStats(70000); err == nil {
		t.Fatal("expected error for port > 65535")
	}
	if _, err := readUDPQueueStats(-1); err == nil {
		t.Fatal("expected error for negative port")
	}
}
