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

package network

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/liqotech/liqo/pkg/liqo-controller-manager/networking/forge"
)

var _ = Describe("Validation", func() {
	type portsTestcase struct {
		serverServicePort     int32
		serverServiceNodePort int32
		clientConnectPort     int32
		// expectedError is the flag name expected in the error message; empty means no error is expected.
		expectedError string
	}

	DescribeTable("gateway ports validation table",
		func(c portsTestcase) {
			options := &Options{
				ServerServicePort:     c.serverServicePort,
				ServerServiceNodePort: c.serverServiceNodePort,
				ClientConnectPort:     c.clientConnectPort,
			}
			err := options.Validate()
			if c.expectedError == "" {
				Expect(err).ToNot(HaveOccurred())
			} else {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(c.expectedError))
			}
		},
		Entry("default values are accepted",
			portsTestcase{serverServicePort: forge.DefaultGwServerPort, serverServiceNodePort: 0, clientConnectPort: 0}),
		Entry("minimum server service port is accepted",
			portsTestcase{serverServicePort: 1, serverServiceNodePort: 0, clientConnectPort: 0}),
		Entry("maximum server service port is accepted",
			portsTestcase{serverServicePort: 65535, serverServiceNodePort: 0, clientConnectPort: 0}),
		Entry("zero server service port is rejected",
			portsTestcase{serverServicePort: 0, serverServiceNodePort: 0, clientConnectPort: 0,
				expectedError: "--gw-server-service-port"}),
		Entry("negative server service port is rejected",
			portsTestcase{serverServicePort: -1, serverServiceNodePort: 0, clientConnectPort: 0,
				expectedError: "--gw-server-service-port"}),
		Entry("server service port above the maximum is rejected",
			portsTestcase{serverServicePort: 65536, serverServiceNodePort: 0, clientConnectPort: 0,
				expectedError: "--gw-server-service-port"}),
		Entry("server service port 70000 is rejected",
			portsTestcase{serverServicePort: 70000, serverServiceNodePort: 0, clientConnectPort: 0,
				expectedError: "--gw-server-service-port"}),
		Entry("minimum NodePort is accepted",
			portsTestcase{serverServicePort: forge.DefaultGwServerPort, serverServiceNodePort: 30000, clientConnectPort: 0}),
		Entry("maximum NodePort is accepted",
			portsTestcase{serverServicePort: forge.DefaultGwServerPort, serverServiceNodePort: 32767, clientConnectPort: 0}),
		Entry("NodePort below the NodePort range is rejected",
			portsTestcase{serverServicePort: forge.DefaultGwServerPort, serverServiceNodePort: 29999, clientConnectPort: 0,
				expectedError: "--gw-server-service-nodeport"}),
		Entry("NodePort above the NodePort range is rejected",
			portsTestcase{serverServicePort: forge.DefaultGwServerPort, serverServiceNodePort: 32768, clientConnectPort: 0,
				expectedError: "--gw-server-service-nodeport"}),
		Entry("NodePort 70000 is rejected",
			portsTestcase{serverServicePort: forge.DefaultGwServerPort, serverServiceNodePort: 70000, clientConnectPort: 0,
				expectedError: "--gw-server-service-nodeport"}),
		Entry("negative NodePort is rejected",
			portsTestcase{serverServicePort: forge.DefaultGwServerPort, serverServiceNodePort: -1, clientConnectPort: 0,
				expectedError: "--gw-server-service-nodeport"}),
		Entry("minimum client connect port is accepted",
			portsTestcase{serverServicePort: forge.DefaultGwServerPort, serverServiceNodePort: 0, clientConnectPort: 1}),
		Entry("maximum client connect port is accepted",
			portsTestcase{serverServicePort: forge.DefaultGwServerPort, serverServiceNodePort: 0, clientConnectPort: 65535}),
		Entry("client connect port above the maximum is rejected",
			portsTestcase{serverServicePort: forge.DefaultGwServerPort, serverServiceNodePort: 0, clientConnectPort: 65536,
				expectedError: "--gw-client-port"}),
		Entry("client connect port 70000 is rejected",
			portsTestcase{serverServicePort: forge.DefaultGwServerPort, serverServiceNodePort: 0, clientConnectPort: 70000,
				expectedError: "--gw-client-port"}),
		Entry("negative client connect port is rejected",
			portsTestcase{serverServicePort: forge.DefaultGwServerPort, serverServiceNodePort: 0, clientConnectPort: -1,
				expectedError: "--gw-client-port"}),
	)
})
