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

package concurrent

import (
	"context"
	"fmt"
	"net"
	"os"

	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/liqotech/liqo/pkg/utils/ipc"
)

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

var _ manager.Runnable = &RunnableGuest{}

// RunnableGuest is a RunnableGuest that manages concurrency.
type RunnableGuest struct {
	GuestID    string
	Connection net.Conn
}

// NewRunnableGuest creates a new Runnable.
func NewRunnableGuest(guestID string) (*RunnableGuest, error) {
	conn, err := ipc.Connect(unixSocketPath, guestID)
	if err != nil {
		return nil, err
	}

	return &RunnableGuest{
		GuestID:    guestID,
		Connection: conn,
	}, nil
}

// Start starts the ConcurrentRunnable.
func (rg *RunnableGuest) Start(_ context.Context) error {
	if err := ipc.WaitForStart(rg.GuestID, rg.Connection); err != nil {
		return err
	}
	return nil
}

// Close closes the Runnable.
func (rg *RunnableGuest) Close() {
	if err := rg.Connection.Close(); err != nil {
		fmt.Printf("Error closing connection: %v\n", err)
		os.Exit(1)
	}
}
