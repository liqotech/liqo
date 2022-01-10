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

package searchdomainoperator

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	discovery "github.com/liqotech/liqo/pkg/discoverymanager"
	"github.com/liqotech/liqo/pkg/utils/testutil"
)

func TestWan(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Wan Suite")
}

var _ = Describe("Wan", func() {

	var (
		dnsServer testutil.DnsServer
	)

	BeforeSuite(func() {
		dnsServer = testutil.DnsServer{}
		dnsServer.Serve()
	})

	AfterSuite(func() {
		dnsServer.Shutdown()
	})

	Context("Wan", func() {

		It("resolve Wan", func() {
			data, err := loadAuthDataFromDNS(dnsServer.GetAddr(), dnsServer.GetName())
			Expect(err).To(BeNil())
			Expect(data).NotTo(BeNil())
			Expect(data).To(Equal([]*discovery.AuthData{
				discovery.NewAuthData("h1.test.liqo.io.", 1234, 60),
				discovery.NewAuthData("h2.test.liqo.io.", 4321, 60),
			}))
		})

	})

})
