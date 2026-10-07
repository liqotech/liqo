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

package forge

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
)

var _ = DescribeTable("the appendArgsGatewayAPI function",
	func(classes []liqov1beta1.GatewayClassType, gateways []liqov1beta1.SharedGatewayType, expected []string) {
		Expect(appendArgsGatewayAPI([]string{"--existing"}, classes, gateways)).To(Equal(append([]string{"--existing"}, expected...)))
	},
	Entry("nothing offered by the remote cluster", nil, nil, []string{}),
	Entry("GatewayClasses offered, without default",
		[]liqov1beta1.GatewayClassType{{GatewayClassName: "envoy"}, {GatewayClassName: "istio"}}, nil,
		[]string{"--enable-gateway-api", "--remote-real-gateway-class-name=envoy"}),
	Entry("GatewayClasses offered, with default",
		[]liqov1beta1.GatewayClassType{{GatewayClassName: "envoy"}, {GatewayClassName: "istio", Default: true}}, nil,
		[]string{"--enable-gateway-api", "--remote-real-gateway-class-name=istio"}),
	Entry("shared Gateways offered", nil,
		[]liqov1beta1.SharedGatewayType{{Namespace: "infra", Name: "internal"}, {Namespace: "infra", Name: "public", Default: true}},
		[]string{"--enable-gateway-api", "--enable-remote-shared-gateway"}),
	Entry("both GatewayClasses and shared Gateways offered",
		[]liqov1beta1.GatewayClassType{{GatewayClassName: "envoy"}},
		[]liqov1beta1.SharedGatewayType{{Namespace: "infra", Name: "public"}},
		[]string{"--enable-gateway-api", "--remote-real-gateway-class-name=envoy", "--enable-remote-shared-gateway"}),
)
