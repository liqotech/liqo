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

package ipc

import (
	"context"
	"fmt"
	"net"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
)

// Connect connects to the Unix domain socket at the given path and sends the given ID.
func Connect(path, id string) (net.Conn, error) {
	var c net.Conn
	var err error
	klog.Infof("Connecting to %s socket", path)
	if err := wait.PollUntilContextTimeout(context.Background(), time.Millisecond*100, time.Second*30, false, func(context.Context) (bool, error) {
		c, err = net.Dial("unix", path)
		if err != nil {
			return false, nil
		}
		return true, nil
	}); err != nil {
		return nil, err
	}

	klog.Infof("Sending message to %s", path)
	if _, err := c.Write([]byte(id)); err != nil {
		return nil, err
	}
	return c, nil
}

// WaitForStart waits for the start signal from the given ID.
func WaitForStart(id string, conn net.Conn) error {
	klog.Infof("Waiting for start message")
	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		return err
	}
	msg := string(buf[:n])

	if msg != ForgeStartMessage(id) {
		return fmt.Errorf("unexpected message: %s", msg)
	}
	klog.Infof("Received start message")
	return nil
}
