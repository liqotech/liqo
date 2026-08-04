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
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testOptions(port int) *Options {
	return &Options{
		PingEnabled:              true,
		PingPort:                 port,
		PingBufferSize:           1024,
		PingLossThreshold:        5,
		PingInterval:             50 * time.Millisecond,
		PingUpdateStatusInterval: 10 * time.Second,
		PingBindIP:               "127.0.0.1",
	}
}

// TestConnCheckerEndToEnd verifies that a sender pinging the local receiver is
// eventually reported as connected with a non-zero latency.
func TestConnCheckerEndToEnd(t *testing.T) {
	opts := testOptions(0)

	var connected atomic.Bool
	var latency atomic.Int64
	observer := func(c bool, l time.Duration) {
		connected.Store(c)
		latency.Store(int64(l))
	}

	cc, err := NewConnChecker(opts)
	require.NoError(t, err)
	defer cc.conn.Close()

	// When PingPort is 0 the kernel assigns a random port; make the sender use
	// the actual port the receiver is listening on.
	opts.PingPort = cc.conn.LocalAddr().(*net.UDPAddr).Port

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go cc.RunReceiver(ctx)
	go cc.RunReceiverDisconnectObserver(ctx)

	clusterID := "local"

	require.NoError(t, cc.AddSender(ctx, clusterID, "127.0.0.1", observer))
	go cc.RunSender(clusterID)

	require.Eventually(t, connected.Load, 5*time.Second, 50*time.Millisecond, "peer never became connected")
	assert.Greater(t, latency.Load(), int64(0), "latency should be positive")
}

// TestConnCheckerRunSenderMapRace exercises concurrent AddSender/RunSender and
// DelAndStopSender to ensure the senders map is never accessed without proper
// synchronization.
func TestConnCheckerRunSenderMapRace(t *testing.T) {
	cc, err := NewConnChecker(testOptions(0))
	require.NoError(t, err)
	defer cc.conn.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go cc.RunReceiver(ctx)
	go cc.RunReceiverDisconnectObserver(ctx)

	clusterID := "race-cluster"

	var wg sync.WaitGroup
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := cc.AddSender(ctx, clusterID, "127.0.0.1", nil); err == nil {
				go cc.RunSender(clusterID)
			}
			time.Sleep(5 * time.Millisecond)
			cc.DelAndStopSender(clusterID)
		}()
	}
	wg.Wait()
}

// TestReceiverDisconnectObserverRace exercises ReceivePong and
// RunDisconnectObserver concurrently to detect data races on Peer fields.
func TestReceiverDisconnectObserverRace(t *testing.T) {
	cc, err := NewConnChecker(testOptions(0))
	require.NoError(t, err)
	defer cc.conn.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	clusterID := "race-peer"
	cc.receiver.InitPeer(clusterID, nil)

	go cc.receiver.RunDisconnectObserver(ctx)

	var wg sync.WaitGroup
	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			msg := &Msg{ClusterID: clusterID, MsgType: PONG, TimeStamp: time.Now()}
			_ = cc.receiver.ReceivePong(msg, time.Now())
		}()
	}
	wg.Wait()
}
