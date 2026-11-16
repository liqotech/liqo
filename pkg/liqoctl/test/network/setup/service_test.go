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

package setup

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	testclient "github.com/liqotech/liqo/pkg/liqoctl/test/network/client"
	"github.com/liqotech/liqo/pkg/liqoctl/test/network/flags"
)

var _ = Describe("CreateService", func() {
	var (
		ctx context.Context
		cl  client.Client
	)

	BeforeEach(func() {
		ctx = context.Background()
		scheme := runtime.NewScheme()
		utilruntime.Must(corev1.AddToScheme(scheme))
		cl = fake.NewClientBuilder().WithScheme(scheme).Build()
	})

	DescribeTable("should force IPv4 single-stack on all the services",
		func(loadBalancer bool, expected []string) {
			Expect(CreateService(ctx, &testclient.Client{Consumer: cl}, &flags.Options{LoadBalancer: loadBalancer})).To(Succeed())

			var svcs corev1.ServiceList
			Expect(cl.List(ctx, &svcs, client.InNamespace(NamespaceName))).To(Succeed())
			names := make([]string, 0, len(svcs.Items))
			for i := range svcs.Items {
				names = append(names, svcs.Items[i].Name)
				Expect(svcs.Items[i].Spec.IPFamilyPolicy).ToNot(BeNil(), "service %s", svcs.Items[i].Name)
				Expect(*svcs.Items[i].Spec.IPFamilyPolicy).To(Equal(corev1.IPFamilyPolicySingleStack), "service %s", svcs.Items[i].Name)
				Expect(svcs.Items[i].Spec.IPFamilies).To(Equal([]corev1.IPFamily{corev1.IPv4Protocol}), "service %s", svcs.Items[i].Name)
			}
			Expect(names).To(ConsistOf(expected))
		},
		Entry("ClusterIP and NodePort", false, []string{DeploymentName, DeploymentName + "np"}),
		Entry("ClusterIP, NodePort and LoadBalancer", true, []string{DeploymentName, DeploymentName + "np", DeploymentName + "lb"}),
	)
})
