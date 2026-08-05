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

package conncheck

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
)

// Peer represents a peer.
type Peer struct {
	connected bool
	latency   time.Duration
	// lastPingTimestamp is the timestamp of the last received PING (used for out-of-order detection).
	lastPingTimestamp time.Time
	// lastPongTimestamp is the time when the last PONG was received.
	lastPongTimestamp time.Time
	// observer is called on PONG and disconnect for this peer.
	observer PingObserver
}

// Receiver is a receiver for conncheck messages.
type Receiver struct {
	peers map[string]*Peer
	m     sync.RWMutex
	buff  []byte
	conn  *net.UDPConn
	opts  *Options
}

// NewReceiver creates a new conncheck receiver.
func NewReceiver(conn *net.UDPConn, opts *Options) *Receiver {
	return &Receiver{
		peers: make(map[string]*Peer),
		buff:  make([]byte, opts.PingBufferSize),
		conn:  conn,
		opts:  opts,
	}
}

// SendPong sends a PONG message to the given address.
func (r *Receiver) SendPong(raddr *net.UDPAddr, msg *Msg) error {
	msg.MsgType = PONG
	b, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal msg: %w", err)
	}
	_, err = r.conn.WriteToUDP(b, raddr)
	if err != nil {
		return fmt.Errorf("failed to write to %s: %w", raddr.String(), err)
	}
	klog.V(8).Infof("conncheck receiver: sent a PONG -> %s", msg)
	return nil
}

// ReceivePong receives a PONG message.
func (r *Receiver) ReceivePong(msg *Msg, receivedAt time.Time) error {
	r.m.Lock()

	peer, ok := r.peers[msg.ClusterID]
	if !ok {
		r.m.Unlock()
		return fmt.Errorf("%s sender has not been initialized", msg.ClusterID)
	}

	if msg.TimeStamp.Before(peer.lastPingTimestamp) {
		klog.V(8).Infof("dropped a PONG message from %s because out-of-order", msg.ClusterID)
		r.m.Unlock()
		return nil
	}
	peer.lastPingTimestamp = msg.TimeStamp
	peer.lastPongTimestamp = receivedAt
	peer.latency = receivedAt.Sub(msg.TimeStamp)
	peer.connected = true
	latency := peer.latency
	r.m.Unlock()

	if peer.observer != nil {
		peer.observer(true, latency)
	}
	return nil
}

// InitPeer initializes a peer.
func (r *Receiver) InitPeer(clusterID string, observer PingObserver) {
	r.m.Lock()
	defer r.m.Unlock()
	r.peers[clusterID] = &Peer{
		connected:         false,
		latency:           0,
		lastPingTimestamp: time.Time{},
		lastPongTimestamp: time.Now(),
		observer:          observer,
	}
}

// Run starts the receiver.
func (r *Receiver) Run(ctx context.Context) {
	klog.Infof("conncheck receiver: started on %s:%d", r.opts.PingBindIP, r.opts.PingPort)
	err := wait.PollUntilContextCancel(ctx, time.Duration(0), false, func(_ context.Context) (done bool, err error) {
		n, raddr, err := r.conn.ReadFromUDP(r.buff)
		if err != nil {
			if raddr != nil {
				klog.Errorf("conncheck receiver: failed to read from %s: %v", raddr.String(), err)
			} else {
				klog.Errorf("conncheck receiver: failed to read: %v", err)
			}
			return false, nil
		}
		receivedAt := time.Now()
		msgr := &Msg{}
		err = json.Unmarshal(r.buff[:n], msgr)
		if err != nil {
			klog.Errorf("conncheck receiver: failed to unmarshal msg: %v", err)
			return false, nil
		}
		klog.V(9).Infof("conncheck receiver: received a msg -> %s", msgr)
		switch msgr.MsgType {
		case PING:
			klog.V(8).Infof("conncheck receiver: received a PING %s -> %s", raddr, msgr)
			if err := r.SendPong(raddr, msgr); err != nil {
				klog.Errorf("conncheck receiver: sendPong error: %v", err)
			}
		case PONG:
			klog.V(8).Infof("conncheck receiver: received a PONG from %s  -> %s", raddr, msgr)
			if err := r.ReceivePong(msgr, receivedAt); err != nil {
				klog.Errorf("conncheck receiver: receivePong error: %v", err)
			}
		}
		return false, nil
	})
	if err != nil {
		klog.Errorf("conncheck receiver: %v", err)
	}
}

// RunDisconnectObserver starts the disconnect observer.
func (r *Receiver) RunDisconnectObserver(ctx context.Context) {
	klog.Infof("conncheck receiver disconnect checker: started")
	// Ignore errors because only caused by context cancellation.
	threshold := r.opts.PingLossThreshold
	if threshold > uint(math.MaxInt64) {
		threshold = uint(math.MaxInt64)
	}
	thresholdDuration := time.Duration(threshold)
	err := wait.PollUntilContextCancel(ctx, thresholdDuration*r.opts.PingInterval/10, true,
		func(_ context.Context) (done bool, err error) {
			// Snapshot the peer IDs, then check each peer under the write lock so that
			// all reads/writes of Peer fields stay synchronized with ReceivePong.
			r.m.RLock()
			peerIDs := make([]string, 0, len(r.peers))
			for id := range r.peers {
				peerIDs = append(peerIDs, id)
			}
			r.m.RUnlock()

			for _, id := range peerIDs {
				r.m.Lock()
				peer, ok := r.peers[id]
				if !ok {
					r.m.Unlock()
					continue
				}
				if time.Since(peer.lastPongTimestamp) <= r.opts.PingInterval*thresholdDuration {
					r.m.Unlock()
					continue
				}
				peer.connected = false
				peer.latency = 0
				r.m.Unlock()

				klog.V(8).Infof("conncheck receiver: %s unreachable", id)
				if peer.observer != nil {
					peer.observer(false, 0)
				}
			}
			return false, nil
		})
	if err != nil {
		klog.Errorf("conncheck disconnect observer: %v", err)
	}
}
