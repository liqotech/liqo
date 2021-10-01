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

package forge_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
	"github.com/liqotech/liqo/pkg/virtualKubelet/options/types"
)

var _ = Describe("Meta forging", func() {
	const (
		localClusterID  = "local"
		remoteClusterID = "remote"
	)

	Describe("Reflection labels", func() {
		BeforeEach(func() {
			localClusterIDOpts := types.NewNetworkingOption(types.LocalClusterID, localClusterID)
			remoteClusterIDOpts := types.NewNetworkingOption(types.RemoteClusterID, remoteClusterID)
			forge.InitForger(nil, localClusterIDOpts, remoteClusterIDOpts)
		})

		Describe("the ReflectionLabels function", func() {
			It("should set exactly two labels", func() { Expect(forge.ReflectionLabels()).To(HaveLen(2)) })
			It("should set the origin cluster label", func() {
				Expect(forge.ReflectionLabels()).To(HaveKeyWithValue(forge.LiqoOriginClusterIDKey, localClusterID))
			})
			It("should set the destination cluster label", func() {
				Expect(forge.ReflectionLabels()).To(HaveKeyWithValue(forge.LiqoDestinationClusterIDKey, remoteClusterID))
			})
		})

		DescribeTableFactory := func(checker func(labels.Set) bool) func() {
			return func() {
				DescribeTable("checking whether there is a match",
					func(labels map[string]string, matches bool) {
						Expect(checker(labels)).To(BeIdenticalTo(matches))
					},
					Entry("when no label is specified", nil, false),
					Entry("when different labels are specified", map[string]string{"foo": "bar"}, false),
					Entry("when only one label is specified", map[string]string{forge.LiqoOriginClusterIDKey: localClusterID}, false),
					Entry("when only the other label is specified", map[string]string{forge.LiqoDestinationClusterIDKey: remoteClusterID}, false),
					Entry("when both labels are specified, with incorrect values", map[string]string{
						forge.LiqoOriginClusterIDKey:      "foo",
						forge.LiqoDestinationClusterIDKey: "bar",
					}, false),
					Entry("when both labels are specified, with the correct values", map[string]string{
						forge.LiqoOriginClusterIDKey:      localClusterID,
						forge.LiqoDestinationClusterIDKey: remoteClusterID,
					}, true),
				)
			}
		}

		Describe("the ReflectedLabelSelector function", DescribeTableFactory(func(labels labels.Set) bool {
			return forge.ReflectedLabelSelector().Matches(labels)
		}))
		Describe("the IsReflected function", DescribeTableFactory(func(labels labels.Set) bool {
			return forge.IsReflected(&corev1.Service{ObjectMeta: metav1.ObjectMeta{Labels: labels}})
		}))
	})
})
