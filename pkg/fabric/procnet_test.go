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

package fabric

import (
	"testing"
)

// 3 CPUs; first has 11 columns (with backlog_len), second full, third minimal (8 columns).
const validSoftnet = `00007b1f 00000000 00000002 00000000 00000000 00000000 00000000 00000000 00000000 00000000 00000006
00001234 00000005 00000000 00000000 00000000 00000000 00000000 00000001 00000000 00000000 00000000
00000042 00000000 00000001 00000000 00000000 00000000 00000000 00000000
`

func TestParseNetSoftnet(t *testing.T) {
	stats, err := parseNetSoftnet(validSoftnet)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(stats) != 3 {
		t.Fatalf("expected 3 CPUs, got %d", len(stats))
	}
	if stats[0].processed != 0x7b1f || stats[0].dropped != 0 || stats[0].timeSqueeze != 2 {
		t.Fatalf("unexpected cpu0 values: %+v", stats[0])
	}
	if !stats[0].hasBacklog || stats[0].backlogLen != 6 {
		t.Fatalf("unexpected cpu0 backlog: %+v", stats[0])
	}
	if stats[1].dropped != 5 {
		t.Fatalf("unexpected cpu1 dropped: %+v", stats[1])
	}
	if stats[2].hasBacklog {
		t.Fatalf("cpu2 should not report backlog length: %+v", stats[2])
	}
}

func TestParseNetSoftnetMalformed(t *testing.T) {
	if _, err := parseNetSoftnet("00000001 00000000\n"); err == nil {
		t.Fatal("expected error for short line")
	}
	if _, err := parseNetSoftnet("zz 0 0\n"); err == nil {
		t.Fatal("expected error for non-hex content")
	}
	if _, err := parseNetSoftnet("\n\n"); err == nil {
		t.Fatal("expected error for empty content")
	}
}
