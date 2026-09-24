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
	"fmt"
	"net"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
)

// ConnChecker is a struct that holds the receiver and senders.
type ConnChecker struct {
	targetClusterID string
	opts            *Options
	receiver        *Receiver
	// key is the interfaceID (e.g. "liqo-tunnel", "liqo-tunnel1").
	senders        map[string]*Sender
	runningSenders map[string]*Sender
	sm             sync.RWMutex
	conn           *net.UDPConn
}

// NewConnChecker creates a new ConnChecker.
func NewConnChecker(targetClusterID string, numInterfaces int, opts *Options) (*ConnChecker, error) {
	bindIP := opts.PingBindIP
	if bindIP == "" {
		bindIP = "0.0.0.0"
	}
	addr := &net.UDPAddr{
		Port: opts.PingPort,
		IP:   net.ParseIP(bindIP),
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on UDP socket %s : %w", addr, err)
	}
	klog.V(4).Infof("conncheck socket: listening on %s", addr)
	connChecker := ConnChecker{
		targetClusterID: targetClusterID,
		opts:            opts,
		receiver:        NewReceiver(conn, opts, numInterfaces),
		senders:         make(map[string]*Sender),
		runningSenders:  make(map[string]*Sender),
		conn:            conn,
	}
	return &connChecker, nil
}

// RunReceiver runs the receiver.
func (c *ConnChecker) RunReceiver(ctx context.Context) {
	c.receiver.Run(ctx)
}

// RunPeerMonitor runs the PeerMonitor.
func (c *ConnChecker) RunPeerMonitor(ctx context.Context) {
	c.receiver.RunPeerMonitor(ctx)
}

// RunReceiverDisconnectObserver runs the receiver disconnect observer.
func (c *ConnChecker) RunReceiverDisconnectObserver(ctx context.Context) {
	c.receiver.RunDisconnectObserver(ctx)
}

// PeerMonitorObserver returns the current observer set on the peer monitor.
func (c *ConnChecker) PeerMonitorObserver() PingObserver {
	return c.receiver.PeerMonitorObserver()
}

// SetPeerMonitorObserver sets the observer on the peer monitor.
func (c *ConnChecker) SetPeerMonitorObserver(observer PingObserver) {
	c.receiver.SetPeerMonitorObserver(observer)
}

// HasSender reports whether a sender has already been added for the given interfaceID.
func (c *ConnChecker) HasSender(interfaceID string) bool {
	c.sm.RLock()
	_, ok := c.senders[interfaceID]
	c.sm.RUnlock()
	return ok
}

// AddSender adds a sender.
func (c *ConnChecker) AddSender(ctx context.Context, interfaceID, ip string, observer PingObserver) error {
	var err error

	if interfaceID == "" {
		return fmt.Errorf("interfaceID cannot be empty")
	}

	c.sm.Lock()
	defer c.sm.Unlock()

	if _, ok := c.senders[interfaceID]; ok {
		return NewDuplicateError(c.peerName(interfaceID))
	}

	ctxSender, cancelSender := context.WithCancel(ctx)
	c.senders[interfaceID], err = NewSender(ctxSender, c.opts, interfaceID, cancelSender, c.conn, ip)
	if err != nil {
		return fmt.Errorf("failed to create sender: %w", err)
	}

	c.receiver.InitPeer(interfaceID, observer)

	klog.Infof("conncheck sender %q added", c.peerName(interfaceID))
	return nil
}

// RunSender runs a sender.
func (c *ConnChecker) RunSender(interfaceID string) {
	sender, err := c.setRunning(interfaceID)
	if err != nil {
		klog.Errorf("conncheck sender %s doesn't start for an error: %s", c.peerName(interfaceID), err)
		return
	}

	klog.Infof("conncheck sender %q starting against %q", c.peerName(interfaceID), sender.raddr.IP.String())

	if err := wait.PollUntilContextCancel(sender.Ctx, c.opts.PingInterval, false, func(_ context.Context) (done bool, err error) {
		if err = sender.SendPing(); err != nil {
			// A single UDP send failure is not fatal; keep polling so a transient
			// error does not permanently break the connection check.
			klog.Warningf("conncheck sender %s: failed to send ping: %s", c.peerName(interfaceID), err)
		}
		return false, nil
	}); err != nil && sender.Ctx.Err() == nil {
		klog.Errorf("conncheck sender %s stopped for an error: %s", c.peerName(interfaceID), err)
	}

	klog.Infof("conncheck sender %s stopped", c.peerName(interfaceID))
}

// DelAndStopSender stops and deletes a sender. If sender has been already stoped and deleted is a no-op function.
func (c *ConnChecker) DelAndStopSender(interfaceID string) {
	c.sm.Lock()
	defer c.sm.Unlock()

	c.receiver.m.Lock()
	defer c.receiver.m.Unlock()

	if _, ok := c.senders[interfaceID]; ok {
		c.senders[interfaceID].cancel()
		delete(c.senders, interfaceID)
	}

	delete(c.runningSenders, interfaceID)
	delete(c.receiver.peers, interfaceID)
}

// PeerStatus holds the current in-memory state of a peer.
type PeerStatus struct {
	Connected bool
	Latency   time.Duration
}

// GetStatus returns the connection status and latency for a specific peer.
// The key identifier (targetID) can be either an interfaceID or a ClusterID.
func (c *ConnChecker) GetStatus(targetID string) (PeerStatus, error) {
	c.receiver.m.RLock()
	defer c.receiver.m.RUnlock()

	peer, ok := c.receiver.peers[targetID]
	if !ok {
		return PeerStatus{}, fmt.Errorf("peer %s not found in receiver", c.targetClusterID)
	}

	return PeerStatus{
		Connected: peer.connected,
		Latency:   peer.latency,
	}, nil
}

// GetStatusMultitunnel returns the aggregated connection status and average latency across all interfaces,
// as calculated by the PeerMonitor.
func (c *ConnChecker) GetStatusMultitunnel() (PeerStatus, error) {
	connected, latency := c.receiver.GetPeerMonitorStatus()
	return PeerStatus{
		Connected: connected,
		Latency:   latency,
	}, nil
}

func (c *ConnChecker) setRunning(interfaceID string) (*Sender, error) {
	c.sm.Lock()
	defer c.sm.Unlock()
	sender, ok := c.senders[interfaceID]
	if !ok {
		return nil, fmt.Errorf("sender %s not found", c.peerName(interfaceID))
	}

	if _, ok := c.runningSenders[interfaceID]; ok {
		return nil, fmt.Errorf("sender %s already running", c.peerName(interfaceID))
	}
	c.runningSenders[interfaceID] = sender
	return sender, nil
}

func (c *ConnChecker) peerName(interfaceID string) string {
	if c.targetClusterID != "" {
		return fmt.Sprintf("%s:%s", c.targetClusterID, interfaceID)
	}
	return interfaceID
}
