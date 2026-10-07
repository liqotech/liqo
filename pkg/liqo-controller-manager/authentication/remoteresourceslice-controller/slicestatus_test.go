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

package remoteresourceslicecontroller

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	gwv1 "sigs.k8s.io/gateway-api/apis/v1"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
)

var _ = Describe("Gateway API offers", func() {
	var (
		ctx  context.Context
		opts *SliceStatusOptions
		cl   client.Client
	)

	gateway := func(namespace, name, label string) *gwv1.Gateway {
		gw := &gwv1.Gateway{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name}}
		if label != "" {
			gw.Labels = map[string]string{consts.SharedGatewayLabel: label}
		}
		return gw
	}

	BeforeEach(func() {
		ctx = context.Background()
		opts = &SliceStatusOptions{GatewayAPIEnabled: true}
		Expect(opts.GatewayClasses.Set("envoy;default,istio")).To(Succeed())

		scheme := runtime.NewScheme()
		Expect(gwv1.Install(scheme)).To(Succeed())
		cl = fake.NewClientBuilder().WithScheme(scheme).WithObjects(
			gateway("infra", "public", consts.SharedGatewayLabelDefaultValue),
			gateway("infra", "internal", consts.SharedGatewayLabelValue),
			gateway("infra", "invalid", "foo"),
			gateway("apps", "private", ""),
		).Build()
	})

	It("should return the offered GatewayClasses", func() {
		Expect(getGatewayClasses(opts)).To(Equal([]liqov1beta1.GatewayClassType{
			{GatewayClassName: "envoy", Default: true}, {GatewayClassName: "istio"},
		}))
	})

	It("should return the Gateways labeled as shared, sorted by namespaced name", func() {
		Expect(getSharedGateways(ctx, cl, opts)).To(Equal([]liqov1beta1.SharedGatewayType{
			{Namespace: "infra", Name: "internal"}, {Namespace: "infra", Name: "public", Default: true},
		}))
	})

	It("should return empty lists if no options are provided, or the Gateway API is not enabled", func() {
		Expect(getGatewayClasses(nil)).To(BeEmpty())
		Expect(getSharedGateways(ctx, cl, nil)).To(BeEmpty())
		Expect(getSharedGateways(ctx, cl, &SliceStatusOptions{})).To(BeEmpty())
	})
})
