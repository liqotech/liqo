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
	"fmt"
	"net"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
)

// ConnChecker is a struct that holds the receiver and senders.
type ConnChecker struct {
	receiver *Receiver
	// key is the target cluster ID.
	senders map[string]*Sender
	sm      sync.RWMutex
	conn    *net.UDPConn
}

// NewConnChecker creates a new ConnChecker.
func NewConnChecker() (*ConnChecker, error) {
	addr := &net.UDPAddr{
		Port: port,
		IP:   net.ParseIP("0.0.0.0"),
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on UDP socket %s : %w", addr, err)
	}
	klog.V(4).Infof("conncheck socket: listening on %s", addr)
	connChecker := ConnChecker{
		receiver: NewReceiver(conn),
		senders:  make(map[string]*Sender),
		conn:     conn,
	}
	return &connChecker, nil
}

// RunReceiver runs the receiver.
func (c *ConnChecker) RunReceiver() {
	c.receiver.Run()
}

// RunReceiverDisconnectObserver runs the receiver disconnect observer.
func (c *ConnChecker) RunReceiverDisconnectObserver() {
	c.receiver.RunDisconnectObserver()
}

// AddAndRunSender create a new sender and runs it.
func (c *ConnChecker) AddAndRunSender(clusterID, ip string, updateCallback UpdateFunc) {
	c.sm.Lock()
	if _, ok := c.senders[clusterID]; ok {
		c.sm.Unlock()
		klog.Infof("sender %s already exists", clusterID)
		return
	}

	ctxSender, cancelSender := context.WithCancel(context.Background())
	c.senders[clusterID] = NewSender(ctxSender, clusterID, cancelSender, c.conn, ip)

	err := c.receiver.InitPeer(clusterID, updateCallback)
	if err != nil {
		c.sm.Unlock()
		klog.Errorf("failed to add redirect chan: %w", err)
	}

	klog.Infof("conncheck sender %s starting", clusterID)
	pingCallback := func(ctx context.Context) (done bool, err error) {
		err = c.senders[clusterID].SendPing(ctx)
		if err != nil {
			klog.Warningf("failed to send ping: %s", err)
		}
		return false, nil
	}
	c.sm.Unlock()

	// Ignore errors because only caused by context cancellation.
	_ = wait.PollImmediateInfiniteWithContext(ctxSender, PingInterval, pingCallback)

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
