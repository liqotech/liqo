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
	clusterID := "local"
	interfaceID := "test"
	cc, err := NewConnChecker(clusterID, 1, opts)
	require.NoError(t, err)
	defer cc.conn.Close()

	// When PingPort is 0 the kernel assigns a random port; make the sender use
	// the actual port the receiver is listening on.
	opts.PingPort = cc.conn.LocalAddr().(*net.UDPAddr).Port

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go cc.RunReceiver(ctx)
	go cc.RunReceiverDisconnectObserver(ctx)

	require.NoError(t, cc.AddSender(ctx, interfaceID, "127.0.0.1", observer))
	go cc.RunSender(interfaceID)

	require.Eventually(t, connected.Load, 5*time.Second, 50*time.Millisecond, "peer never became connected")
	assert.Greater(t, latency.Load(), int64(0), "latency should be positive")
}

// TestConnCheckerRunSenderMapRace exercises concurrent AddSender/RunSender and
// DelAndStopSender to ensure the senders map is never accessed without proper
// synchronization.
func TestConnCheckerRunSenderMapRace(t *testing.T) {
	clusterID := "race-cluster"
	cc, err := NewConnChecker(clusterID, 1, testOptions(0))
	require.NoError(t, err)
	defer cc.conn.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go cc.RunReceiver(ctx)
	go cc.RunReceiverDisconnectObserver(ctx)

	interfaceID := "test"
	var wg sync.WaitGroup
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := cc.AddSender(ctx, interfaceID, "127.0.0.1", nil); err == nil {
				go cc.RunSender(interfaceID)
			}
			time.Sleep(5 * time.Millisecond)
			cc.DelAndStopSender(interfaceID)
		}()
	}
	wg.Wait()
}

// TestReceiverDisconnectObserverRace exercises ReceivePong and
// RunDisconnectObserver concurrently to detect data races on Peer fields.
func TestReceiverDisconnectObserverRace(t *testing.T) {
	clusterID := "race-peer"
	cc, err := NewConnChecker(clusterID, 1, testOptions(0))
	require.NoError(t, err)
	defer cc.conn.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	interfaceID := "test"
	cc.receiver.InitPeer(interfaceID, nil)

	go cc.receiver.RunDisconnectObserver(ctx)

	var wg sync.WaitGroup
	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			msg := &Msg{InterfaceID: interfaceID, MsgType: PONG, TimeStamp: time.Now()}
			_ = cc.receiver.ReceivePong(msg, time.Now())
		}()
	}
	wg.Wait()
}

// TestConnCheckerEndToEndMultitunnel verifies that multiple senders pinging the local receiver are
// eventually reported as connected by the PeerMonitor with a non-zero latency.
func TestConnCheckerEndToEndMultitunnel(t *testing.T) {
	opts := testOptions(0)

	var connected atomic.Bool
	var latency atomic.Int64
	observer := func(c bool, l time.Duration) {
		connected.Store(c)
		latency.Store(int64(l))
	}
	clusterID := "local"
	baseInterfaceID := "test"
	numInterfaces := 10
	cc, err := NewConnChecker(clusterID, numInterfaces, opts)
	require.NoError(t, err)
	defer cc.conn.Close()

	// When PingPort is 0 the kernel assigns a random port; make the sender use
	// the actual port the receiver is listening on.
	opts.PingPort = cc.conn.LocalAddr().(*net.UDPAddr).Port

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go cc.RunReceiver(ctx)
	go cc.RunReceiverDisconnectObserver(ctx)
	go cc.RunPeerMonitor(ctx)
	cc.SetPeerMonitorObserver(observer)
	for i := range numInterfaces {
		interfaceID := fmt.Sprintf("%s-%d", baseInterfaceID, i)
		require.NoError(t, cc.AddSender(ctx, interfaceID, "127.0.0.1", nil))
		go cc.RunSender(interfaceID)
	}

	require.Eventually(t, connected.Load, 5*time.Second, 50*time.Millisecond, "peer never became connected")
	assert.Greater(t, latency.Load(), int64(0), "latency should be positive")
}

// TestConnCheckerPeerMonitorWaitsForAllInterfaces verifies that RunPeerMonitor never reports
// connected=true when fewer senders than numInterfaces are registered.
func TestConnCheckerPeerMonitorWaitsForAllInterfaces(t *testing.T) {
	opts := testOptions(0)

	var connected atomic.Bool
	var latency atomic.Int64
	observer := func(c bool, l time.Duration) {
		connected.Store(c)
		latency.Store(int64(l))
	}
	clusterID := "local"
	baseInterfaceID := "test"
	const (
		numInterfaces = 5
		numSenders    = 3 // intentionally fewer than numInterfaces
	)
	cc, err := NewConnChecker(clusterID, numInterfaces, opts)
	require.NoError(t, err)
	defer cc.conn.Close()

	// When PingPort is 0 the kernel assigns a random port; make the sender use
	// the actual port the receiver is listening on.
	opts.PingPort = cc.conn.LocalAddr().(*net.UDPAddr).Port

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go cc.RunReceiver(ctx)
	go cc.RunReceiverDisconnectObserver(ctx)
	go cc.RunPeerMonitor(ctx)
	cc.SetPeerMonitorObserver(observer)
	for i := range numSenders {
		interfaceID := fmt.Sprintf("%s-%d", baseInterfaceID, i)
		require.NoError(t, cc.AddSender(ctx, interfaceID, "127.0.0.1", nil))
		go cc.RunSender(interfaceID)
	}
	// Give the registered senders plenty of time to become individually
	// connected (several multiples of PingInterval), then assert the
	// aggregated status never flips to true because numSenders < numInterfaces.
	assert.Never(t, connected.Load, 1*time.Second, 50*time.Millisecond,
		"PeerMonitor reported connected=true despite fewer senders than numInterfaces")
	assert.Zero(t, latency.Load(), "latency should stay zero while the monitor never aggregates")
	// Sanity check: the registered peers are actually connected, so the Never above
	// is due to the numInterfaces guard and not to broken senders
	for i := range numSenders {
		interfaceID := fmt.Sprintf("%s-%d", baseInterfaceID, i)
		status, err := cc.GetStatus(interfaceID)
		require.NoError(t, err)
		assert.True(t, status.Connected, "peer %s should be individually connected", interfaceID)
	}
}

// TestConnCheckerPeerMonitorRecoversAfterSenderRestart verifies that killing a single sender's
// underlying context (without touching receiver.peers, so numInterfaces stays intact)
// makes that peer and the aggregated PeerMonitor status go from connected to disconnected,
// and that re-adding the sender for the same interfaceID brings both back to connected.
func TestConnCheckerPeerMonitorRecoversAfterSenderRestart(t *testing.T) {
	opts := testOptions(0)

	var connected atomic.Bool
	var latency atomic.Int64
	observer := func(c bool, l time.Duration) {
		connected.Store(c)
		latency.Store(int64(l))
	}
	clusterID := "local"
	baseInterfaceID := "test"
	numInterfaces := 10
	cc, err := NewConnChecker(clusterID, numInterfaces, opts)
	require.NoError(t, err)
	defer cc.conn.Close()

	// When PingPort is 0 the kernel assigns a random port; make the sender use
	// the actual port the receiver is listening on.
	opts.PingPort = cc.conn.LocalAddr().(*net.UDPAddr).Port

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go cc.RunReceiver(ctx)
	go cc.RunReceiverDisconnectObserver(ctx)
	go cc.RunPeerMonitor(ctx)
	cc.SetPeerMonitorObserver(observer)
	for i := range numInterfaces {
		interfaceID := fmt.Sprintf("%s-%d", baseInterfaceID, i)
		require.NoError(t, cc.AddSender(ctx, interfaceID, "127.0.0.1", nil))
		go cc.RunSender(interfaceID)
	}

	require.Eventually(t, connected.Load, 5*time.Second, 50*time.Millisecond, "peer never became connected")
	assert.Greater(t, latency.Load(), int64(0), "latency should be positive")

	// Kill one sender (0) keeping receiver.peers intact
	victimID := fmt.Sprintf("%s-%d", baseInterfaceID, 0)

	cc.sm.Lock()
	victim, ok := cc.senders[victimID]
	require.True(t, ok, "victim sender should exist before killing it")
	victim.cancel()
	delete(cc.senders, victimID)
	delete(cc.runningSenders, victimID)
	cc.sm.Unlock()

	// The individual peer must flip to disconnected once it exceeds PingLossThreshold*PingInterval,
	// and the aggregated PeerMonitor must follow because allConnected requires every peer connected.
	require.Eventually(t, func() bool {
		status, err := cc.GetStatus(victimID)
		return err == nil && !status.Connected && status.Latency == 0
	}, 2*time.Second, 50*time.Millisecond, "victim peer never became disconnected")

	require.Eventually(t, func() bool {
		return !connected.Load()
	}, 2*time.Second, 50*time.Millisecond, "PeerMonitor never reflected the victim's disconnection")
	assert.Zero(t, latency.Load(), "aggregated latency should reset to zero once not all peers are connected")

	// The other peers must be unaffected by the victim's death.
	for i := 1; i < numInterfaces; i++ {
		interfaceID := fmt.Sprintf("%s-%d", baseInterfaceID, i)
		status, err := cc.GetStatus(interfaceID)
		require.NoError(t, err)
		assert.True(t, status.Connected, "peer %s should remain connected while only %s is down", interfaceID, victimID)
	}

	// Re-add the sender for the same interfaceID and expect full recovery
	require.NoError(t, cc.AddSender(ctx, victimID, "127.0.0.1", nil))
	go cc.RunSender(victimID)

	require.Eventually(t, func() bool {
		status, err := cc.GetStatus(victimID)
		return err == nil && status.Connected
	}, 2*time.Second, 50*time.Millisecond, "victim peer never reconnected")

	require.Eventually(t, connected.Load, 2*time.Second, 50*time.Millisecond,
		"PeerMonitor never recovered to connected after sender restart")
	assert.Greater(t, latency.Load(), int64(0), "latency should be positive again after recovery")
}

// TestConnCheckerPeerMonitorAveragesHeterogeneousLatencies verifies that RunPeerMonitor computes
// the aggregated latency as the arithmetic mean across all peers, using injected per-peer
// latencies (no real sender/receiver traffic) so the exact expected average is known up front.
func TestConnCheckerPeerMonitorAveragesHeterogeneousLatencies(t *testing.T) {
	opts := testOptions(0)

	var connected atomic.Bool
	var latency atomic.Int64
	observer := func(c bool, l time.Duration) {
		connected.Store(c)
		latency.Store(int64(l))
	}
	clusterID := "local"
	baseInterfaceID := "test"
	numInterfaces := 5
	cc, err := NewConnChecker(clusterID, numInterfaces, opts)
	require.NoError(t, err)
	defer cc.conn.Close()

	// When PingPort is 0 the kernel assigns a random port; make the sender use
	// the actual port the receiver is listening on.
	opts.PingPort = cc.conn.LocalAddr().(*net.UDPAddr).Port

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// No RunReceiver and no RunReceiverDisconnectObserver: this test injects peer state
	// directly, without any real UDP traffic. Running the disconnect observer here would
	// otherwise flip everything back to disconnected once lastPongTimestamp goes stale.
	go cc.RunPeerMonitor(ctx)
	cc.SetPeerMonitorObserver(observer)
	var wantTotal time.Duration
	for i := range numInterfaces {
		interfaceID := fmt.Sprintf("%s-%d", baseInterfaceID, i)
		cc.receiver.InitPeer(interfaceID, nil)

		peerLatency := time.Duration(i+1) * time.Millisecond // 1ms, 2ms, 3ms, 4ms, 5ms
		wantTotal += peerLatency

		cc.receiver.m.Lock()
		peer := cc.receiver.peers[interfaceID]
		peer.connected = true
		peer.latency = peerLatency
		peer.lastPongTimestamp = time.Now()
		cc.receiver.m.Unlock()
	}
	wantAvg := wantTotal / time.Duration(numInterfaces)

	require.Eventually(t, connected.Load, 2*time.Second, 50*time.Millisecond, "PeerMonitor never reported connected=true")
	assert.Equal(t, wantAvg, time.Duration(latency.Load()), "aggregated latency should be the arithmetic mean of the injected per-peer latencies")

	// Now flip one peer to disconnected by hand (no disconnect observer running here)
	// and verify both the individual peer and the aggregated PeerMonitor reflect it.
	victimID := fmt.Sprintf("%s-%d", baseInterfaceID, 2)
	cc.receiver.m.Lock()
	victim := cc.receiver.peers[victimID]
	victim.connected = false
	victim.latency = 0
	cc.receiver.m.Unlock()

	require.Eventually(t, func() bool {
		status, err := cc.GetStatus(victimID)
		return err == nil && !status.Connected && status.Latency == 0
	}, 2*time.Second, 50*time.Millisecond, "victim peer never reflected the manual disconnection")

	require.Eventually(t, func() bool {
		return !connected.Load()
	}, 2*time.Second, 50*time.Millisecond, "PeerMonitor never reflected the victim's disconnection")
	assert.Zero(t, latency.Load(), "aggregated latency should reset to zero once not all peers are connected")

	// The other peers must be unaffected by the victim's disconnection.
	for i := range numInterfaces {
		if i == 2 {
			continue
		}
		interfaceID := fmt.Sprintf("%s-%d", baseInterfaceID, i)
		status, err := cc.GetStatus(interfaceID)
		require.NoError(t, err)
		assert.True(t, status.Connected, "peer %s should remain connected while only %s is down", interfaceID, victimID)
	}
}

// TestConnCheckerMultiInterfaceRunSenderMapRace exercises concurrent AddSender/RunSender/
// DelAndStopSender across many distinct interfaceIDs (unlike TestConnCheckerRunSenderMapRace,
// which stresses only a single shared key), while RunPeerMonitor concurrently iterates
// receiver.peers in the background. Ensures senders/runningSenders/receiver.peers are never
// accessed without proper synchronization across a realistic N-tunnel key space.
func TestConnCheckerMultiInterfaceRunSenderMapRace(t *testing.T) {
	clusterID := "race-cluster"
	const (
		numInterfaces = 50
		numIterations = 50
	)
	cc, err := NewConnChecker(clusterID, numInterfaces, testOptions(0))
	require.NoError(t, err)
	defer cc.conn.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go cc.RunReceiver(ctx)
	go cc.RunReceiverDisconnectObserver(ctx)
	go cc.RunPeerMonitor(ctx)
	cc.SetPeerMonitorObserver(func(bool, time.Duration) {})

	interfaceBaseID := "test"

	var wg sync.WaitGroup
	for range numIterations {
		for j := range numInterfaces {
			interfaceName := fmt.Sprintf("%s-%d", interfaceBaseID, j)
			wg.Add(1)
			go func() {
				defer wg.Done()
				if err := cc.AddSender(ctx, interfaceName, "127.0.0.1", nil); err == nil {
					go cc.RunSender(interfaceName)
				}
				time.Sleep(5 * time.Millisecond)
				cc.DelAndStopSender(interfaceName)
			}()
		}
	}
	wg.Wait()

	// Once every goroutine has completed its own Add/Delete lifecycle, no entries
	// should remain: this catches logical leaks, not just data races.
	cc.sm.Lock()
	assert.Empty(t, cc.senders, "senders map should be empty once all goroutines finished")
	assert.Empty(t, cc.runningSenders, "runningSenders map should be empty once all goroutines finished")
	cc.sm.Unlock()

	cc.receiver.m.Lock()
	assert.Empty(t, cc.receiver.peers, "receiver.peers map should be empty once all goroutines finished")
	cc.receiver.m.Unlock()
}
