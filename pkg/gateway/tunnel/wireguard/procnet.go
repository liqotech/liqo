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
	"fmt"
	"os"
	"strconv"
	"strings"
)

const (
	// procNetSnmpPath is the path of the file exposing per-protocol (namespaced) network statistics.
	procNetSnmpPath = "/proc/net/snmp"
	// procSysRmemMax is the path to the maximum UDP socket receive buffer size.
	procSysRmemMax = "/proc/sys/net/core/rmem_max"
	// procSysWmemMax is the path to the maximum UDP socket send buffer size.
	procSysWmemMax = "/proc/sys/net/core/wmem_max"

	// snmpUDPPrefix is the protocol prefix in /proc/net/snmp.
	snmpUDPPrefix = "Udp:"

	// Field names of interest in the Udp: section of /proc/net/snmp.
	udpFieldRcvbufErrors = "RcvbufErrors"
	udpFieldSndbufErrors = "SndbufErrors"
	udpFieldInErrors     = "InErrors"
	udpFieldNoPorts      = "NoPorts"
)

// udpSummary holds the Udp: counters parsed from /proc/net/snmp.
// These counters are scoped to the network namespace of the process that
// reads the file; in the gateway pod they describe the UDP stack that the
// WireGuard kernel socket uses.
type udpSummary struct {
	rcvbuf   uint64 // RcvbufErrors: datagrams dropped due to a full socket receive buffer.
	sndbuf   uint64 // SndbufErrors: datagrams dropped due to a full socket send buffer.
	inErrors uint64 // InErrors: datagrams dropped for other reasons (e.g. no memory).
	noPorts  uint64 // NoPorts: datagrams received for closed ports.
}

// parseNetSnmp parses /proc/net/snmp content and returns the Udp: counters.
// The file contains, for each protocol, a header line listing field names
// immediately followed by a values line with the counters, e.g.:
//
//	Udp: InDatagrams NoPorts InErrors OutDatagrams RcvbufErrors SndbufErrors ...
//	Udp: 1234 5 6 1234 0 0 ...
func parseNetSnmp(content string) (*udpSummary, error) {
	var headers []string
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, snmpUDPPrefix) {
			continue
		}
		fields := strings.Fields(strings.TrimPrefix(line, snmpUDPPrefix))
		if len(fields) == 0 {
			return nil, fmt.Errorf("malformed %q line: no fields", snmpUDPPrefix)
		}
		if headers == nil {
			// Header line.
			headers = fields
			continue
		}
		// Values line.
		if len(fields) != len(headers) {
			return nil, fmt.Errorf("malformed %q values line: got %d values, expected %d",
				snmpUDPPrefix, len(fields), len(headers))
		}
		values := make([]uint64, len(fields))
		for i, f := range fields {
			v, err := strconv.ParseUint(f, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("parsing %q values line: %w", snmpUDPPrefix, err)
			}
			values[i] = v
		}
		m := map[string]uint64{}
		for i := range headers {
			m[headers[i]] = values[i]
		}
		// Validate that all fields of interest are present.
		for _, name := range []string{udpFieldRcvbufErrors, udpFieldSndbufErrors, udpFieldInErrors, udpFieldNoPorts} {
			if _, ok := m[name]; !ok {
				return nil, fmt.Errorf("field %q not found in %q section", name, snmpUDPPrefix)
			}
		}
		return &udpSummary{
			rcvbuf:   m[udpFieldRcvbufErrors],
			sndbuf:   m[udpFieldSndbufErrors],
			inErrors: m[udpFieldInErrors],
			noPorts:  m[udpFieldNoPorts],
		}, nil
	}
	return nil, fmt.Errorf("%q section not found", snmpUDPPrefix)
}

// readUDPSummary reads and parses the Udp: section of /proc/net/snmp.
func readUDPSummary() (*udpSummary, error) {
	data, err := os.ReadFile(procNetSnmpPath)
	if err != nil {
		return nil, fmt.Errorf("reading %q: %w", procNetSnmpPath, err)
	}
	return parseNetSnmp(string(data))
}

// socketBufferLimits holds the kernel-configured maximum UDP socket buffer
// sizes visible to the current network namespace.
type socketBufferLimits struct {
	rmemMax uint64
	wmemMax uint64
}

// readSocketBufferLimits reads /proc/sys/net/core/{r,w}mem_max.
func readSocketBufferLimits() (*socketBufferLimits, error) {
	rmem, err := readUintFile(procSysRmemMax)
	if err != nil {
		return nil, fmt.Errorf("reading rmem_max: %w", err)
	}
	wmem, err := readUintFile(procSysWmemMax)
	if err != nil {
		return nil, fmt.Errorf("reading wmem_max: %w", err)
	}
	return &socketBufferLimits{rmemMax: rmem, wmemMax: wmem}, nil
}

// readUintFile reads a file containing a single unsigned integer.
func readUintFile(path string) (uint64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("reading %q: %w", path, err)
	}
	v, err := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parsing %q: %w", path, err)
	}
	return v, nil
}
