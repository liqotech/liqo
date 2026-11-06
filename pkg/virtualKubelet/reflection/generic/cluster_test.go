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

package generic

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	reflectionfake "github.com/liqotech/liqo/pkg/virtualKubelet/reflection/generic/fake"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/manager"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/options"
)

var _ = Describe("Cluster Reflector tests", func() {
	const reflectorName = "cluster-reflector"

	var (
		rfl     manager.Reflector
		crfl    *reflectionfake.ClusterReflector
		workers uint
	)

	NewFakeClusterReflector := func(opts *options.ReflectorOpts) manager.NamespacedReflector {
		crfl = reflectionfake.NewClusterReflector(opts)
		return crfl
	}

	Context("a new cluster reflector is created", func() {
		JustBeforeEach(func() {
			rfl = NewClusterReflector(reflectorName, NewFakeClusterReflector,
				workers, offloadingv1beta1.CustomLiqo, ConcurrencyModeLeader)
		})

		When("no workers are specified", func() {
			BeforeEach(func() { workers = 0 })
			It("should return a dummy reflector", func() { Expect(rfl).To(PointTo(BeAssignableToTypeOf(dummyreflector{}))) })
		})

		When("at least one worker is specified", func() {
			BeforeEach(func() { workers = 1 })
			It("should return a real reflector", func() { Expect(rfl).To(PointTo(BeAssignableToTypeOf(reflector{}))) })
		})
	})

	Context("a new real cluster reflector is created", func() {
		var (
			opts   options.ReflectorOpts
			ctx    context.Context
			cancel context.CancelFunc
		)

		BeforeEach(func() {
			ctx, cancel = context.WithCancel(context.Background())
			workers = 0 /* do not start any child go routine */
			opts = options.ReflectorOpts{LocalClient: fake.NewClientset()}
		})
		JustBeforeEach(func() {
			rfl = newClusterReflector(reflectorName, NewFakeClusterReflector,
				workers, offloadingv1beta1.CustomLiqo, ConcurrencyModeLeader)
			rfl.Start(ctx, &opts)
		})
		AfterEach(func() { cancel() })

		It("should register the single cluster reflector instance", func() {
			Expect(rfl.(*reflector).reflectors).To(HaveKeyWithValue("", crfl))
		})
		It("should not configure any fallback reflector", func() {
			Expect(rfl.(*reflector).fallback).To(BeNil())
		})
		It("should correctly propagate the reflector options", func() {
			Expect(crfl.Opts.LocalClient).To(Equal(opts.LocalClient))
			Expect(crfl.Opts.HandlerFactory).ToNot(BeNil())
			Expect(crfl.Opts.ReflectionType).To(Equal(offloadingv1beta1.CustomLiqo))
		})

		Context("an item is handled", func() {
			var (
				key   types.NamespacedName
				ready bool
				err   error
			)

			BeforeEach(func() { ready = false })
			JustBeforeEach(func() {
				if ready {
					crfl.SetReady()
				}
				err = rfl.(*reflector).handle(ctx, key)
			})

			When("the item refers to a cluster-scoped object", func() {
				BeforeEach(func() { key = types.NamespacedName{Name: "foo"} })
				When("the cluster reflector is not ready", func() {
					It("should return an error", func() { Expect(err).To(HaveOccurred()) })
					It("should not execute the cluster reflector handler", func() { Expect(crfl.Handled).To(BeNumerically("==", 0)) })
				})
				When("the cluster reflector is ready", func() {
					BeforeEach(func() { ready = true })
					It("should not return an error", func() { Expect(err).ToNot(HaveOccurred()) })
					It("should execute the cluster reflector handler", func() { Expect(crfl.Handled).To(BeNumerically("==", 1)) })
				})
			})

			When("the item refers to a namespaced object", func() {
				BeforeEach(func() { key = types.NamespacedName{Namespace: "whatever", Name: "foo"} })
				It("should not return an error", func() { Expect(err).ToNot(HaveOccurred()) })
				It("should not execute the cluster reflector handler", func() { Expect(crfl.Handled).To(BeNumerically("==", 0)) })
			})
		})

		Context("the namespace lifecycle is triggered", func() {
			It("should not register any namespaced reflector", func() {
				rfl.StartNamespace(&options.NamespacedOpts{LocalNamespace: "local", RemoteNamespace: "remote"})
				Expect(rfl.(*reflector).reflectors).ToNot(HaveKey("local"))
				Expect(rfl.(*reflector).reflectors).To(HaveLen(1))
			})

			It("should not deregister the cluster reflector on namespace stop", func() {
				rfl.StopNamespace("local", "remote")
				Expect(rfl.(*reflector).reflectors).To(HaveKeyWithValue("", crfl))
			})
		})

		It("should resync the cluster reflector without errors", func() {
			Expect(rfl.Resync()).ToNot(HaveOccurred())
		})
	})
})
