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

package gatewayapi_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	gwutils "github.com/liqotech/liqo/pkg/utils/gatewayapi"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/gatewayapi"
)

var _ = DescribeTable("Gateway API reflection support",
	func(local, remote bool, expected gatewayapi.Support) {
		gvr := gwutils.HTTPRoutesGVR
		Expect(gatewayapi.SupportFor(gvr, gwutils.Availability{gvr: local}, gwutils.Availability{gvr: remote})).To(Equal(expected))
	},
	Entry("missing in both clusters", false, false, gatewayapi.SupportNone),
	Entry("missing in the local cluster only", false, true, gatewayapi.SupportNone),
	Entry("missing in the remote cluster only", true, false, gatewayapi.SupportDegraded),
	Entry("available in both clusters", true, true, gatewayapi.SupportFull),
)
