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
	"fmt"
	"os"
	"strconv"
	"strings"
)

const (
	// procNetSoftnetPath is the path of the file exposing per-CPU softnet (NET_RX softirq) statistics.
	procNetSoftnetPath = "/proc/net/softnet_stat"
)

// softnetStat holds the counters parsed from a line of /proc/net/softnet_stat.
// Columns (hex, in order): processed, dropped, time_squeeze, 0, 0, 0, 0,
// cpu_collision, received_rps, flow_limit_count, backlog_len.
type softnetStat struct {
	processed   uint64 // packets processed by the NET_RX softirq on this CPU.
	dropped     uint64 // packets dropped because the per-CPU netdev backlog was full.
	timeSqueeze uint64 // times the softirq exhausted its budget/timeslice with work remaining.
	backlogLen  uint64 // current netdev backlog queue length (present only on recent kernels).
	hasBacklog  bool   // whether the kernel reports the backlog_len column.
}

// parseNetSoftnet parses /proc/net/softnet_stat content, returning one
// softnetStat entry per CPU (file line order corresponds to CPU index).
func parseNetSoftnet(content string) ([]softnetStat, error) {
	var stats []softnetStat
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			return nil, fmt.Errorf("malformed softnet_stat line %q: got %d fields, expected at least 3", line, len(fields))
		}
		values := make([]uint64, len(fields))
		for i, f := range fields {
			v, err := strconv.ParseUint(f, 16, 64)
			if err != nil {
				return nil, fmt.Errorf("parsing softnet_stat line %q: %w", line, err)
			}
			values[i] = v
		}
		stat := softnetStat{
			processed:   values[0],
			dropped:     values[1],
			timeSqueeze: values[2],
		}
		// The backlog_len column (11th, index 10) was added in kernel 2.6.41;
		// tolerate older kernels reporting fewer columns.
		if len(values) > 10 {
			stat.backlogLen = values[10]
			stat.hasBacklog = true
		}
		stats = append(stats, stat)
	}
	if len(stats) == 0 {
		return nil, fmt.Errorf("no softnet_stat entries found")
	}
	return stats, nil
}

// readSoftnetStats reads and parses /proc/net/softnet_stat.
func readSoftnetStats() ([]softnetStat, error) {
	data, err := os.ReadFile(procNetSoftnetPath)
	if err != nil {
		return nil, fmt.Errorf("reading %q: %w", procNetSoftnetPath, err)
	}
	return parseNetSoftnet(string(data))
}
