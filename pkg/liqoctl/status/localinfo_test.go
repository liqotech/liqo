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

package status

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gstruct"
	"github.com/pterm/pterm"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	"github.com/liqotech/liqo/pkg/liqoctl/factory"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
	"github.com/liqotech/liqo/pkg/utils/testutil"
)

var _ = Describe("LocalInfo", func() {
	const (
		clusterID   = "fake"
		clusterName = "fake"
		namespace   = "liqo"
	)

	var (
		clientBuilder  fake.ClientBuilder
		lic            *LocalInfoChecker
		ctx            context.Context
		options        Options
		errFmt, errCol error
		text           string
	)

	BeforeSuite(func() {
		ctx = context.Background()
		_ = netv1alpha1.AddToScheme(scheme.Scheme)
		pterm.DisableStyling()
	})

	BeforeEach(func() {
		clientBuilder = *fake.NewClientBuilder().WithScheme(scheme.Scheme)
		clientBuilder.WithObjects(
			testutil.FakeClusterIDConfigMap(namespace, clusterID, clusterName),
			testutil.FakeIPAM(namespace),
		)
		options = Options{Factory: factory.NewForLocal()}
		options.Printer = output.NewFakePrinter(GinkgoWriter)
		options.CRClient = clientBuilder.Build()
	})

	Context("Creating a new LocalInfoChecker", func() {
		JustBeforeEach(func() {
			lic = newLocalInfoChecker(&options)
		})
		It("should return a valid LocalInfoChecker", func() {
			Expect(lic.localInfoSection).To(Equal(output.NewRootSection()))
			Expect(lic.options).To(gstruct.PointTo(Equal(options)))
			Expect(lic.getReleaseValues).ToNot(BeNil())
		})
	})
	Context("Collecting and Formatting LocalInfoChecker", func() {
		BeforeEach(func() {
			lic = newLocalInfoCheckerTest(&options, testutil.FakeHelmValues())
			errCol = lic.Collect(ctx)
		})
		JustBeforeEach(func() {
			text, errFmt = lic.Format()
		})
		It("should not return errors", func() {
			Expect(errCol).ToNot(HaveOccurred())
			Expect(errFmt).ToNot(HaveOccurred())
		})
		It("should format a valid text", func() {
			Expect(text).To(ContainSubstring(
				pterm.Sprintf("Cluster ID: %s", clusterID),
			))
			Expect(text).To(ContainSubstring(
				pterm.Sprintf("Cluster Name: %s", clusterName),
			))
			for _, v := range testutil.ClusterLabels {
				if s, ok := v.(string); ok {
					Expect(text).To(ContainSubstring(s))
				}
			}
			Expect(text).To(ContainSubstring(
				pterm.Sprintf("Pod CIDR: %s", testutil.PodCIDR),
			))
			Expect(text).To(ContainSubstring(
				pterm.Sprintf("Service CIDR: %s", testutil.ServiceCIDR),
			))
			Expect(text).To(ContainSubstring(
				pterm.Sprintf("External CIDR: %s", testutil.ExternalCIDR),
			))
			Expect(text).To(ContainSubstring(
				pterm.Sprintf("Address: %s", testutil.APIAddress),
			))
			for _, v := range testutil.ReservedSubnets {
				Expect(text).To(ContainSubstring(v))
			}

		})
	})
})
