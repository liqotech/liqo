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
	"fmt"
	"time"
)

// Msg represents a message sent between two nodes.
type Msg struct {
	ClusterID string    `json:"clusterID"`
	MsgType   MsgTypes  `json:"msgType"`
	TimeStamp time.Time `json:"timeStamp"`
}

func (msg Msg) String() string {
	return fmt.Sprintf("ClusterID: %s, MsgType: %s, Timestamp: %s",
		msg.ClusterID,
		msg.MsgType,
		msg.TimeStamp.Format("00:00:00.000000000"))
}

// MsgTypes represents the type of a message.
type MsgTypes string

const (
	// PING is the type of a ping message.
	PING MsgTypes = "PING"
	// PONG is the type of a pong message.
	PONG MsgTypes = "PONG"
)

// PingObserver reports the outcome of a single connectivity measurement for a peer, so a
// caller can react to it (e.g. record metrics, detect connected/disconnected transitions).
// It is invoked by the Receiver on every accepted PONG (connected=true, latency set) and
// whenever a peer is deemed unreachable due to exceeding the ping loss threshold
// (connected=false, latency zero). Implementations must be safe to call from multiple
// goroutines and should not block, since they run synchronously on the conncheck receive path.
type PingObserver func(connected bool, latency time.Duration)
