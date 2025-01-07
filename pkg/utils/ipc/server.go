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
	"net"
	"os"

	"k8s.io/klog/v2"
)

// GuestConnections is a map of guest IDs to connections.
type GuestConnections map[string]net.Conn

// NewGuestConnections creates a new GuestConnections map from string slice.
func NewGuestConnections(guests []string) GuestConnections {
	guestsConnections := make(GuestConnections)
	for _, guest := range guests {
		guestsConnections[guest] = nil
	}
	return guestsConnections
}

// checkAllGuestsConnected checks if all guests are connected.
func checkAllGuestsConnected(guestsConnections GuestConnections) bool {
	for _, connected := range guestsConnections {
		if connected == nil {
			return false
		}
	}
	return true
}

// addGuestConnection adds a new connection to the guestsConnections map.
func addGuestConnection(guestsConnections GuestConnections, guest string, connection net.Conn) {
	guestsConnections[guest] = connection
}

// CreateListenSocket creates a Unix domain socket and listens for incoming connections.
func CreateListenSocket(path string) (net.Listener, error) {
	if err := os.RemoveAll(path); err != nil {
		return nil, err
	}

	// Create a Unix domain socket and listen for incoming connections.
	klog.Infof("Listening on %s socket", path)
	socket, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	return socket, nil
}

// CloseListenSocket closes the listen socket.
func CloseListenSocket(socket net.Listener) {
	if err := socket.Close(); err != nil {
		klog.Errorf("Error closing socket: %v", err)
		os.Exit(1)
	}
}

// WaitAllGuestsConnections waits for all guests to connect to the socket.
func WaitAllGuestsConnections(guestsConnections GuestConnections, socket net.Listener) error {
	for !checkAllGuestsConnected(guestsConnections) {
		// Accept an incoming connection.
		conn, err := socket.Accept()
		if err != nil {
			return err
		}

		// Create a buffer for incoming data.
		buf := make([]byte, 4096)

		// Read data from the connection.
		n, err := conn.Read(buf)
		if err != nil {
			return err
		}
		id := string(buf[:n])

		addGuestConnection(guestsConnections, id, conn)

		klog.Infof("Guest %s connected\n", id)
	}
	return nil
}

// CloseAllGuestsConnections closes all connections in the guestsConnections map.
func CloseAllGuestsConnections(guestsConnections GuestConnections) {
	for _, conn := range guestsConnections {
		if conn != nil {
			if err := conn.Close(); err != nil {
				klog.Errorf("Error closing connection: %v", err)
				os.Exit(1)
			}
		}
	}
}

// StartAllGuestsConnections sends a start message to all connections in the guestsConnections map.
func StartAllGuestsConnections(guestsConnections GuestConnections) error {
	for guest, conn := range guestsConnections {
		if _, err := conn.Write([]byte(ForgeStartMessage(guest))); err != nil {
			return err
		}
	}
	klog.Infof("Sent start message to all guests")
	return nil
}
