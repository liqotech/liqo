// Copyright 2019-2021 The Liqo Authors
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

package apiserver_test

import (
	"flag"
	"strconv"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"github.com/liqotech/liqo/pkg/utils/apiserver"
)

var _ = Describe("The API server configuration", func() {

	var cfg apiserver.Config

	Describe("the GetConfig function", func() {
		Context("configuring the API server parameters", func() {
			var fs flag.FlagSet

			BeforeEach(func() {
				fs = *flag.NewFlagSet("test-flags", flag.PanicOnError)
				apiserver.InitFlags(&fs)
			})
			JustBeforeEach(func() { cfg = apiserver.GetConfig() })

			When("using the default configuration", func() {
				It("should set an empty address", func() { Expect(cfg.Address).To(Equal("")) })
				It("should set an un-trusted CA", func() { Expect(cfg.TrustedCA).To(BeFalse()) })
			})

			When("specifying a custom configuration", func() {
				BeforeEach(func() {
					utilruntime.Must(fs.Set("advertise-api-server-address", "https://foo.bar:8080"))
					utilruntime.Must(fs.Set("advertise-api-server-trusted-ca", strconv.FormatBool(true)))
				})

				It("should set the desired address", func() { Expect(cfg.Address).To(Equal("https://foo.bar:8080")) })
				It("should set the desired trusted CA value", func() { Expect(cfg.TrustedCA).To(BeTrue()) })
			})
		})
	})
})
