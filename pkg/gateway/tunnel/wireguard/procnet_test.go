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
	"testing"
)

const validSnmp = `Ip: Forwarding DefaultTTL InReceives InHdrErrors InAddrErrors ForwDatagrams InUnknownProtos InDiscards InDelivers OutRequests OutDiscards OutNoRoutes ReasmTimeout ReasmReqds ReasmOKs ReasmFails FragOKs FragFails FragCreates
Ip: 1 64 100000 0 0 0 0 0 90000 95000 0 0 0 0 0 0 0 0 0
Tcp: RtoAlgorithm RtoMin RtoMax MaxConn ActiveOpens PassiveOpens AttemptFails EstabResets CurrEstab InSegs OutSegs RetransSegs InErrs OutRsts InCsumErrors
Tcp: 1 200 120000 -1 100 50 2 1 5 50000 48000 10 0 3 0
Udp: InDatagrams NoPorts InErrors OutDatagrams RcvbufErrors SndbufErrors InCsumErrors IgnoredMulti MemErrors
Udp: 2000 4 7 2100 42 5 1 0 0
UdpLite: InDatagrams NoPorts InErrors OutDatagrams RcvbufErrors SndbufErrors InCsumErrors IgnoredMulti MemErrors
UdpLite: 0 0 0 0 0 0 0 0 0
`

func TestParseNetSnmp(t *testing.T) {
	s, err := parseNetSnmp(validSnmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.rcvbuf != 42 || s.sndbuf != 5 || s.inErrors != 7 || s.noPorts != 4 {
		t.Fatalf("unexpected values: %+v", s)
	}
}

func TestParseNetSnmpMissingSection(t *testing.T) {
	if _, err := parseNetSnmp("Ip: A\nIp: 1\n"); err == nil {
		t.Fatal("expected error for missing Udp: section")
	}
}

func TestParseNetSnmpMissingField(t *testing.T) {
	content := "Udp: InDatagrams OutDatagrams\nUdp: 1 2\n"
	if _, err := parseNetSnmp(content); err == nil {
		t.Fatal("expected error for missing fields of interest")
	}
}

func TestParseNetSnmpMismatch(t *testing.T) {
	content := "Udp: InDatagrams NoPorts\nUdp: 1 2 3\n"
	if _, err := parseNetSnmp(content); err == nil {
		t.Fatal("expected error for mismatched header/values lengths")
	}
}
