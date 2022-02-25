// Copyright 2019-2022 The Liqo Authors
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

package common

import "fmt"

// Endpoint maps a service that has to be accessed by a remote cluster.
type Endpoint struct {
	ip         string
	port       string
	remappedIP string
}

// GetIP returns the ip address that has on the cluster where the endpoint lives.
func (ep *Endpoint) GetIP() string {
	return ep.ip
}

// SetRemappedIP sets the ip address as seen by the remote cluster.
func (ep *Endpoint) SetRemappedIP(ip string) {
	ep.remappedIP = ip
}

// GetHTTPURL returns the http url for the endpoint.
func (ep *Endpoint) GetHTTPURL() string {
	return fmt.Sprintf("http://%s:%s", ep.remappedIP, ep.port)
}

// GetHTTPSURL return the https url for the endpoint.
func (ep *Endpoint) GetHTTPSURL() string {
	return fmt.Sprintf("https://%s:%s", ep.remappedIP, ep.port)
}
