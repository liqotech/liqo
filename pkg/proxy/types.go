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

package proxy

import (
	"context"
	"fmt"
	"net"
	"strings"

	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var _ manager.Runnable = &Proxy{}

// Proxy is a simple HTTP Connect proxy.
type Proxy struct {
	AllowedHosts []string
	Port         int
	ForceHost    string
}

// New creates a new Proxy.
func New(allowedHosts string, port int, forceHost string) *Proxy {
	ah := strings.Split(allowedHosts, ",")
	// remove empty strings
	for i := 0; i < len(ah); i++ {
		if ah[i] == "" {
			ah = append(ah[:i], ah[i+1:]...)
			i--
		}
	}

	return &Proxy{
		AllowedHosts: ah,
		Port:         port,
		ForceHost:    forceHost,
	}
}

// Start starts the proxy.
func (p *Proxy) Start(ctx context.Context) error {
	klog.Infof("proxy listening on port %d", p.Port)
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", p.Port))
	if err != nil {
		return err
	}
	defer listener.Close()

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		conn, err := listener.Accept()
		if err != nil {
			klog.Errorf("error accepting connection: %v", err)
			continue
		}

		go p.handleConnect(conn)
	}
}

func (p *Proxy) isAllowed(host string) bool {
	if len(p.AllowedHosts) == 0 {
		return true
	}

	for _, allowedHost := range p.AllowedHosts {
		if host == allowedHost {
			return true
		}
	}
	return false
}
