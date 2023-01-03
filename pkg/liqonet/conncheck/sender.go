// Copyright 2019-2023 The Liqo Authors
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

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"time"

	"k8s.io/klog/v2"
)

// Sender is a sender for the conncheck server.
type Sender struct {
	clusterID string
	cancel    func()
	conn      *net.UDPConn
	raddr     net.UDPAddr
}

// NewSender creates a new conncheck sender.
func NewSender(ctx context.Context, clusterID string, cancel func(), conn *net.UDPConn, ip string) *Sender {
	return &Sender{
		clusterID: clusterID,
		cancel:    cancel,
		conn:      conn,
		raddr:     net.UDPAddr{IP: net.ParseIP(ip), Port: port},
	}
}

// SendPing sends a PING message to the given address.
func (s *Sender) SendPing(ctx context.Context) error {
	msgOut := Msg{ClusterID: s.clusterID, MsgType: PING, TimeStamp: time.Now()}
	b, err := json.Marshal(msgOut)
	if err != nil {
		return fmt.Errorf("conncheck sender: failed to marshal msg: %w", err)
	}
	_, err = s.conn.WriteToUDP(b, &s.raddr)
	if err != nil {
		return fmt.Errorf("conncheck sender: failed to write to %s: %w", s.raddr.String(), err)
	}
	klog.V(8).Infof("conncheck sender: sent a PING -> %s", msgOut)
	return nil
}
