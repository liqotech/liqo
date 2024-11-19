// Copyright 2019-2024 The Liqo Authors
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
	"bufio"
	"io"
	"net"
	"net/http"
	"time"

	"k8s.io/klog/v2"
)

func (p *Proxy) handleConnect(c net.Conn) {
	br := bufio.NewReader(c)
	req, err := http.ReadRequest(br)
	if err != nil {
		klog.Errorf("error reading request: %v", err)
		return
	}

	if req.Method != http.MethodConnect {
		response := &http.Response{
			StatusCode: http.StatusMethodNotAllowed,
			ProtoMajor: 1,
			ProtoMinor: 1,
		}
		response.Write(c)
		c.Close()
		return
	}

	if !p.isAllowed(req.URL.Host) {
		klog.Infof("host %s is not allowed", req.URL.Host)

		response := &http.Response{
			StatusCode: http.StatusForbidden,
			ProtoMajor: 1,
			ProtoMinor: 1,
		}
		response.Write(c)
		return
	}

	klog.Infof("handling CONNECT to %s", req.URL.Host)

	response := &http.Response{
		StatusCode: 200,
		ProtoMajor: 1,
		ProtoMinor: 1,
	}
	response.Write(c)

	destConn, err := net.DialTimeout("tcp", req.URL.Host, 30*time.Second)
	if err != nil {
		response := &http.Response{
			StatusCode: http.StatusRequestTimeout,
			ProtoMajor: 1,
			ProtoMinor: 1,
		}
		response.Write(c)
		return
	}

	go transfer(destConn, c)
	go transfer(c, destConn)
}

func transfer(destination io.WriteCloser, source io.ReadCloser) {
	defer destination.Close()
	defer source.Close()
	io.Copy(destination, source)
}
