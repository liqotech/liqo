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

package installutils

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	"k8s.io/client-go/rest"
)

var _ = Describe("Test API Server Endpoint", func() {

	type checkEndpointTestcase struct {
		endpoint       string
		config         *rest.Config
		expectedOutput types.GomegaMatcher
	}

	DescribeTable("CheckEndpoint table",
		func(c checkEndpointTestcase) {
			res, err := CheckEndpoint(c.endpoint, c.config)
			Expect(err).ToNot(HaveOccurred())
			Expect(res).To(c.expectedOutput)
		},

		Entry("protocol and no port, equal", checkEndpointTestcase{
			endpoint: "https://example.com",
			config: &rest.Config{
				Host: "https://example.com",
			},
			expectedOutput: BeTrue(),
		}),

		Entry("protocol and no port, not equal", checkEndpointTestcase{
			endpoint: "https://example.com",
			config: &rest.Config{
				Host: "https://example2.com",
			},
			expectedOutput: BeFalse(),
		}),

		Entry("protocol and port, equal", checkEndpointTestcase{
			endpoint: "https://example.com:1234",
			config: &rest.Config{
				Host: "https://example.com:1234",
			},
			expectedOutput: BeTrue(),
		}),

		Entry("protocol and port, not equal", checkEndpointTestcase{
			endpoint: "https://example.com:1234",
			config: &rest.Config{
				Host: "https://example2.com:1234",
			},
			expectedOutput: BeFalse(),
		}),

		Entry("protocol and port, not equal (different port)", checkEndpointTestcase{
			endpoint: "https://example.com:1234",
			config: &rest.Config{
				Host: "https://example.com:123",
			},
			expectedOutput: BeFalse(),
		}),

		Entry("protocol and port, not equal (different port)", checkEndpointTestcase{
			endpoint: "https://example.com:1234",
			config: &rest.Config{
				Host: "https://example.com",
			},
			expectedOutput: BeFalse(),
		}),

		Entry("protocol and port, equal", checkEndpointTestcase{
			endpoint: "https://example.com:443",
			config: &rest.Config{
				Host: "https://example.com",
			},
			expectedOutput: BeTrue(),
		}),

		Entry("no protocol and port, equal", checkEndpointTestcase{
			endpoint: "example.com:443",
			config: &rest.Config{
				Host: "example.com",
			},
			expectedOutput: BeTrue(),
		}),

		Entry("no protocol and port, equal", checkEndpointTestcase{
			endpoint: "example.com:443",
			config: &rest.Config{
				Host: "https://example.com",
			},
			expectedOutput: BeTrue(),
		}),

		Entry("no protocol and port, equal", checkEndpointTestcase{
			endpoint: "example.com",
			config: &rest.Config{
				Host: "https://example.com:443",
			},
			expectedOutput: BeTrue(),
		}),
	)

})
