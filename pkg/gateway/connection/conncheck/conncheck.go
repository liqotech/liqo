// Copyright 2019-2025 The Liqo Authors
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
	opts     *Options
	receiver *Receiver
	// key is the target cluster ID.
	senders        map[string]*Sender
	runningSenders map[string]*Sender
	sm             sync.RWMutex
	conn           *net.UDPConn
}

// NewConnChecker creates a new ConnChecker.
func NewConnChecker(opts *Options) (*ConnChecker, error) {
	addr := &net.UDPAddr{
		Port: opts.PingPort,
		IP:   net.ParseIP("0.0.0.0"),
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on UDP socket %s : %w", addr, err)
	}
	klog.V(4).Infof("conncheck socket: listening on %s", addr)
	connChecker := ConnChecker{
		opts:           opts,
		receiver:       NewReceiver(conn, opts),
		senders:        make(map[string]*Sender),
		runningSenders: make(map[string]*Sender),
		conn:           conn,
	}
	return &connChecker, nil
}

// RunReceiver runs the receiver.
func (c *ConnChecker) RunReceiver(ctx context.Context) {
	c.receiver.Run(ctx)
}

// RunReceiverDisconnectObserver runs the receiver disconnect observer.
func (c *ConnChecker) RunReceiverDisconnectObserver(ctx context.Context) {
	c.receiver.RunDisconnectObserver(ctx)
}

// AddSender adds a sender.
func (c *ConnChecker) AddSender(ctx context.Context, clusterID, ip string, updateCallback UpdateFunc) error {
	var err error

	if clusterID == "" {
		return fmt.Errorf("clusterID cannot be empty")
	}

	c.sm.Lock()
	defer c.sm.Unlock()

	if _, ok := c.senders[clusterID]; ok {
		return NewDuplicateError(clusterID)
	}

	ctxSender, cancelSender := context.WithCancel(ctx)
	c.senders[clusterID], err = NewSender(ctxSender, c.opts, clusterID, cancelSender, c.conn, ip)
	if err != nil {
		return fmt.Errorf("failed to create sender: %w", err)
	}

	err = c.receiver.InitPeer(clusterID, updateCallback)
	if err != nil {
		return fmt.Errorf("failed to init peer: %w", err)
	}

	klog.Infof("conncheck sender %q added", clusterID)
	return nil
}

// RunSender runs a sender.
func (c *ConnChecker) RunSender(clusterID string) {
	sender, err := c.setRunning(clusterID)
	if err != nil {
		klog.Errorf("conncheck sender %s doesn't start for an error: %s", clusterID, err)
		return
	}

	klog.Infof("conncheck sender %q starting against %q", clusterID, sender.raddr.IP.String())

	if err := wait.PollUntilContextCancel(sender.Ctx, c.opts.PingInterval, false, func(_ context.Context) (done bool, err error) {
		err = c.senders[clusterID].SendPing()
		if err != nil {
			klog.Warningf("failed to send ping: %s", err)
		}
		return false, nil
	}); err != nil {
		klog.Errorf("conncheck sender %s stopped for an error: %s", clusterID, err)
	}

	klog.Infof("conncheck sender %s stopped", clusterID)
}

// DelAndStopSender stops and deletes a sender. If sender has been already stoped and deleted is a no-op function.
func (c *ConnChecker) DelAndStopSender(clusterID string) {
	c.sm.Lock()
	defer c.sm.Unlock()

	c.receiver.m.Lock()
	defer c.receiver.m.Unlock()

	if _, ok := c.senders[clusterID]; ok {
		c.senders[clusterID].cancel()
		delete(c.senders, clusterID)
	}

	delete(c.runningSenders, clusterID)
	delete(c.receiver.peers, clusterID)
}

// GetLatency returns the latency with clusterID.
func (c *ConnChecker) GetLatency(clusterID string) (time.Duration, error) {
	c.receiver.m.RLock()
	defer c.receiver.m.RUnlock()
	if peer, ok := c.receiver.peers[clusterID]; ok {
		return peer.latency, nil
	}
	return 0, fmt.Errorf("sender %s not found", clusterID)
}

// GetConnected returns the connection status with clusterID.
func (c *ConnChecker) GetConnected(clusterID string) (bool, error) {
	c.receiver.m.RLock()
	defer c.receiver.m.RUnlock()
	if peer, ok := c.receiver.peers[clusterID]; ok {
		return peer.connected, nil
	}
	return false, fmt.Errorf("sender %s not found", clusterID)
}

func (c *ConnChecker) setRunning(clusterID string) (*Sender, error) {
	c.sm.Lock()
	defer c.sm.Unlock()
	sender, ok := c.senders[clusterID]
	if !ok {
		return nil, fmt.Errorf("sender %s not found", clusterID)
	}

	if _, ok := c.runningSenders[clusterID]; ok {
		return nil, fmt.Errorf("sender %s already running", clusterID)
	}
	c.runningSenders[clusterID] = sender
	return sender, nil
}
