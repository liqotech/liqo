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

package sourcedetector

import (
	"fmt"
	"net"
	"os"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

const (
	// PingTimeout is the default timeout used when checking IP reachability.
	PingTimeout = 5 * time.Second
	// PingPayload is the payload sent with every ICMP echo request.
	PingPayload = "liqo-source-detector"
)

// PingIP checks whether the provided IP is reachable by sending a single
// ICMP echo request and waiting for an echo reply.
// It works for both IPv4 and IPv6 destinations.
func PingIP(ip string, timeout time.Duration) error {
	dst := net.ParseIP(ip)
	if dst == nil {
		return fmt.Errorf("unable to parse IP %q", ip)
	}

	isV4 := dst.To4() != nil

	var network string
	var requestType, replyType icmp.Type
	var proto int

	if isV4 {
		network = "ip4:icmp"
		requestType = ipv4.ICMPTypeEcho
		replyType = ipv4.ICMPTypeEchoReply
		proto = ipv4.ICMPTypeEcho.Protocol()
	} else {
		network = "ip6:ipv6-icmp"
		requestType = ipv6.ICMPTypeEchoRequest
		replyType = ipv6.ICMPTypeEchoReply
		proto = ipv6.ICMPTypeEchoRequest.Protocol()
	}

	conn, err := icmp.ListenPacket(network, "")
	if err != nil {
		return fmt.Errorf("unable to open ICMP socket: %w", err)
	}
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		return fmt.Errorf("unable to set ICMP deadline: %w", err)
	}

	id := os.Getpid() & 0xffff
	seq := 1

	msg := &icmp.Message{
		Type: requestType,
		Code: 0,
		Body: &icmp.Echo{
			ID:   id,
			Seq:  seq,
			Data: []byte(PingPayload),
		},
	}

	data, err := msg.Marshal(nil)
	if err != nil {
		return fmt.Errorf("unable to marshal ICMP echo request: %w", err)
	}

	if _, err := conn.WriteTo(data, &net.IPAddr{IP: dst}); err != nil {
		return fmt.Errorf("unable to send ICMP echo request to %q: %w", ip, err)
	}

	reply := make([]byte, 1500)
	for {
		n, peer, err := conn.ReadFrom(reply)
		if err != nil {
			return fmt.Errorf("ICMP ping to %q timed out or failed: %w", ip, err)
		}

		rm, err := icmp.ParseMessage(proto, reply[:n])
		if err != nil {
			continue
		}

		if rm.Type != replyType {
			continue
		}

		echo, ok := rm.Body.(*icmp.Echo)
		if !ok {
			continue
		}

		if echo.ID != id || echo.Seq != seq {
			continue
		}

		if peerIP, ok := peer.(*net.IPAddr); !ok || !peerIP.IP.Equal(dst) {
			continue
		}

		return nil
	}
}
