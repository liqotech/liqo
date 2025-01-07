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

package portforwarder

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/httpstream"
	"k8s.io/apimachinery/pkg/util/runtime"
	netutils "k8s.io/utils/net"
)

// PortForwardProtocolV1Name is the subprotocol used for port forwarding.
const PortForwardProtocolV1Name = "portforward.k8s.io"

// ErrLostConnectionToPod is the variable used for error handling.
var ErrLostConnectionToPod = errors.New("lost connection to pod")

// PortForwarder gets an input stream and forward it to
// a remote pod via an upgraded HTTP request.
type PortForwarder struct {
	addresses []listenAddress
	ports     []ForwardedPort
	stopChan  <-chan struct{}

	dialer      httpstream.Dialer
	streamConn  httpstream.Connection
	inputStream io.ReadWriteCloser
	Ready       chan struct{}
	out         io.Writer
	errOut      io.Writer
}

// ForwardedPort contains a Local:Remote port pairing.
type ForwardedPort struct {
	Local  uint16
	Remote uint16
}

/*
valid port specifications:

5000
- forwards from localhost:5000 to pod:5000

8888:5000
- forwards from localhost:8888 to pod:5000

0:5000
:5000
  - selects a random available local port,
    forwards from localhost:<random port> to pod:5000
*/
func parsePorts(ports []string) ([]ForwardedPort, error) {
	var forwards []ForwardedPort
	for _, portString := range ports {
		parts := strings.Split(portString, ":")
		var localString, remoteString string
		switch len(parts) {
		case 1:
			localString = parts[0]
			remoteString = parts[0]
		case 2:
			localString = parts[0]
			if localString == "" {
				// support :5000
				localString = "0"
			}
			remoteString = parts[1]
		default:
			return nil, fmt.Errorf("invalid port format '%s'", portString)
		}

		localPort, err := strconv.ParseUint(localString, 10, 16)
		if err != nil {
			return nil, fmt.Errorf("error parsing local port '%s': %w", localString, err)
		}

		remotePort, err := strconv.ParseUint(remoteString, 10, 16)
		if err != nil {
			return nil, fmt.Errorf("error parsing remote port '%s': %w", remoteString, err)
		}
		if remotePort == 0 {
			return nil, fmt.Errorf("remote port must be > 0")
		}

		forwards = append(forwards, ForwardedPort{uint16(localPort), uint16(remotePort)})
	}

	return forwards, nil
}

type listenAddress struct {
	address     string
	protocol    string
	failureMode string
}

func parseAddresses(addressesToParse []string) ([]listenAddress, error) {
	var addresses []listenAddress
	parsed := make(map[string]listenAddress)
	for _, address := range addressesToParse {
		switch {
		case address == "localhost":
			if _, exists := parsed["127.0.0.1"]; !exists {
				ip := listenAddress{address: "127.0.0.1", protocol: "tcp4", failureMode: "all"}
				parsed[ip.address] = ip
			}
			if _, exists := parsed["::1"]; !exists {
				ip := listenAddress{address: "::1", protocol: "tcp6", failureMode: "all"}
				parsed[ip.address] = ip
			}
		case netutils.ParseIPSloppy(address).To4() != nil:
			parsed[address] = listenAddress{address: address, protocol: "tcp4", failureMode: "any"}
		case netutils.ParseIPSloppy(address) != nil:
			parsed[address] = listenAddress{address: address, protocol: "tcp6", failureMode: "any"}
		default:
			return nil, fmt.Errorf("%s is not a valid IP", address)
		}
	}
	addresses = make([]listenAddress, len(parsed))
	id := 0
	for _, v := range parsed {
		addresses[id] = v
		id++
	}
	// Sort addresses before returning to get a stable order
	sort.Slice(addresses, func(i, j int) bool { return addresses[i].address < addresses[j].address })

	return addresses, nil
}

// New creates a new PortForwarder with localhost listen addresses.
func New(dialer httpstream.Dialer, ports []string, stopChan <-chan struct{}, readyChan chan struct{},
	out, errOut io.Writer, stream io.ReadWriteCloser) (*PortForwarder, error) {
	return NewOnAddresses(dialer, []string{"localhost"}, ports, stopChan, readyChan, out, errOut, stream)
}

// NewOnAddresses creates a new PortForwarder with custom listen addresses.
func NewOnAddresses(dialer httpstream.Dialer, addresses, ports []string, stopChan <-chan struct{}, readyChan chan struct{},
	out, errOut io.Writer, stream io.ReadWriteCloser) (*PortForwarder, error) {
	if len(addresses) == 0 {
		return nil, errors.New("you must specify at least 1 address")
	}
	parsedAddresses, err := parseAddresses(addresses)
	if err != nil {
		return nil, err
	}
	if len(ports) == 0 {
		return nil, errors.New("you must specify at least 1 port")
	}
	parsedPorts, err := parsePorts(ports)
	if err != nil {
		return nil, err
	}
	return &PortForwarder{
		dialer:      dialer,
		addresses:   parsedAddresses,
		ports:       parsedPorts,
		stopChan:    stopChan,
		Ready:       readyChan,
		out:         out,
		errOut:      errOut,
		inputStream: stream,
	}, nil
}

// ForwardPorts formats and executes a port forwarding request. The connection will remain
// open until stopChan is closed.
func (pf *PortForwarder) ForwardPorts() error {
	var err error
	pf.streamConn, _, err = pf.dialer.Dial(PortForwardProtocolV1Name)
	if err != nil {
		return fmt.Errorf("error upgrading connection: %w", err)
	}
	defer pf.streamConn.Close()
	defer pf.inputStream.Close()

	return pf.forward()
}

// forward dials the remote host specific in req, upgrades the request
// and forwards connections to the remote host via streams.
func (pf *PortForwarder) forward() error {
	for i := range pf.ports {
		port := pf.ports[i]
		pf.handleConnection(port)
	}

	if pf.Ready != nil {
		close(pf.Ready)
	}

	// wait for interrupt or conn closure
	select {
	case <-pf.stopChan:
	case <-pf.streamConn.CloseChan():
		return ErrLostConnectionToPod
	}

	return nil
}

// handleConnection copies data between the local connection and the stream to
// the remote server.
func (pf *PortForwarder) handleConnection(port ForwardedPort) {
	if pf.out != nil {
		fmt.Fprintf(pf.out, "Handling connection for %d\n", port.Local)
	}

	// create error stream
	headers := http.Header{}
	headers.Set(v1.StreamType, v1.StreamTypeError)
	headers.Set(v1.PortHeader, fmt.Sprintf("%d", port.Remote))
	errorStream, err := pf.streamConn.CreateStream(headers)
	if err != nil {
		runtime.HandleError(fmt.Errorf("error creating error stream for port %d -> %d: %w", port.Local, port.Remote, err))
		return
	}
	// we're not writing to this stream
	runtime.Must(errorStream.Close())

	defer pf.streamConn.RemoveStreams(errorStream)

	errorChan := make(chan error)
	go func() {
		message, err := io.ReadAll(errorStream)
		switch {
		case err != nil:
			errorChan <- fmt.Errorf("error reading from error stream for port %d -> %d: %w", port.Local, port.Remote, err)
		case len(message) > 0:
			errorChan <- fmt.Errorf("an error occurred forwarding %d -> %d: %v", port.Local, port.Remote, string(message))
		}
		close(errorChan)
	}()

	// create data stream
	headers.Set(v1.StreamType, v1.StreamTypeData)
	dataStream, err := pf.streamConn.CreateStream(headers)
	if err != nil {
		runtime.HandleError(fmt.Errorf("error creating forwarding stream for port %d -> %d: %w", port.Local, port.Remote, err))
		return
	}
	defer pf.streamConn.RemoveStreams(dataStream)

	localError := make(chan struct{})
	remoteDone := make(chan struct{})

	// Go routine to Copy data: remote side ---> input stream.
	go func() {
		if _, err := io.Copy(pf.inputStream, dataStream); err != nil && !strings.Contains(err.Error(), "use of closed network connection") {
			runtime.HandleError(fmt.Errorf("error copying from remote stream to local connection: %w", err))
		}

		// inform the select below that the remote copy is done
		close(remoteDone)
	}()

	// Go Routine to Copy data: input stream ---> remote side.
	go func() {
		// inform server we're not sending any more data after copy unblocks
		defer dataStream.Close()

		if _, err := io.Copy(dataStream, pf.inputStream); err != nil && !strings.Contains(err.Error(), "use of closed network connection") {
			runtime.HandleError(fmt.Errorf("error copying from local connection to remote stream: %w", err))
			// break out of the select below without waiting for the other copy to finish
			close(localError)
		}
	}()

	// wait for either a local->remote error or for copying from remote->local to finish
	select {
	case <-remoteDone:
	case <-localError:
	}

	// always expect something on errorChan (it may be nil)
	err = <-errorChan
	if err != nil {
		runtime.HandleError(err)
		runtime.Must(pf.streamConn.Close())
		runtime.Must(pf.inputStream.Close())
	}
}
