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

package grpc

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
)

// WaitForConnectionReady tries to connect with a gRPC server until it is Ready.
// It stops waiting if the context expires or the connection is not Ready after the timeout.
func WaitForConnectionReady(ctx context.Context, conn *grpc.ClientConn, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	for {
		state := conn.GetState()
		switch state {
		case connectivity.Idle:
			conn.Connect()
		case connectivity.Ready:
			return nil
		default:
		}

		changed := conn.WaitForStateChange(ctx, state)
		if !changed {
			return fmt.Errorf("timeout connection to the gRPC server")
		}
	}
}
