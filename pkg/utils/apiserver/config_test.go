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

package apiserver_test

import (
	"encoding/base64"
	"flag"
	"strconv"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"

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

	Describe("the Complete function", func() {
		const caData = "certification-authority"

		var (
			config  apiserver.Config
			restcfg rest.Config
			client  kubernetes.Interface

			err error
		)

		BeforeEach(func() {
			// Testing only with an already specified address, as the retrieval logic
			// is already validated with an appropriate test.
			config = apiserver.Config{Address: "foo.bar:8080"}

			restcfg.CAData = []byte(caData)
			client = fake.NewSimpleClientset()
		})

		JustBeforeEach(func() { err = config.Complete(&restcfg, client) })

		When("the TrustedCA is not set", func() {
			BeforeEach(func() { config.TrustedCA = false })
			It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
			It("should set the correct address", func() { Expect(config.Address).To(Equal("https://foo.bar:8080")) })
			It("should set the correct CA", func() { Expect(config.CA).To(Equal(base64.StdEncoding.EncodeToString([]byte(caData)))) })
		})

		When("the TrustedCA is set", func() {
			BeforeEach(func() { config.TrustedCA = true })
			It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
			It("should set the correct address", func() { Expect(config.Address).To(Equal("https://foo.bar:8080")) })
			It("should leave the CA empty", func() { Expect(config.CA).To(Equal("")) })
		})
	})
})
